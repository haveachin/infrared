package infrared

import (
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

func RateLimit(requestLimit int, windowLength time.Duration, options ...RateLimiterOption) func(Handler) Handler {
	return newRateLimiter(requestLimit, windowLength, options...).Handler
}

func RateLimitByIP(requestLimit int, windowLength time.Duration) func(Handler) Handler {
	return RateLimit(requestLimit, windowLength, WithKeyFuncs(KeyByIP))
}

func KeyByIP(c Conn) string {
	ip, _, err := net.SplitHostPort(c.RemoteAddr().String())
	if err != nil {
		ip = c.RemoteAddr().String()
	}
	return canonicalizeIP(ip)
}

func KeyByGatewayID(c Conn) string {
	return c.GatewayID()
}

func WithKeyFuncs(keyFuncs ...RateLimiterKeyFunc) RateLimiterOption {
	return func(rl *rateLimiter) {
		if len(keyFuncs) > 0 {
			rl.keyFn = composedKeyFunc(keyFuncs...)
		}
	}
}

func WithKeyByIP() RateLimiterOption {
	return WithKeyFuncs(KeyByIP)
}

func composedKeyFunc(keyFuncs ...RateLimiterKeyFunc) RateLimiterKeyFunc {
	return func(c Conn) string {
		var key strings.Builder
		for i := 0; i < len(keyFuncs); i++ {
			k := keyFuncs[i](c)
			key.WriteString(k)
		}
		return key.String()
	}
}

type RateLimiterKeyFunc func(c Conn) string
type RateLimiterOption func(rl *rateLimiter)

// canonicalizeIP returns a form of ip suitable for comparison to other IPs.
// For IPv4 addresses, this is simply the whole string.
// For IPv6 addresses, this is the /64 prefix.
func canonicalizeIP(ip string) string {
	isIPv6 := false
	// This is how net.ParseIP decides if an address is IPv6
	for i := 0; !isIPv6 && i < len(ip); i++ {
		switch ip[i] {
		case '.':
			// IPv4
			return ip
		case ':':
			// IPv6
			isIPv6 = true
		}
	}

	if !isIPv6 {
		// Not an IP address at all
		return ip
	}

	ipv6 := net.ParseIP(ip)
	if ipv6 == nil {
		return ip
	}

	return ipv6.Mask(net.CIDRMask(64, 128)).String()
}

func newRateLimiter(requestLimit int, windowLength time.Duration, options ...RateLimiterOption) *rateLimiter {
	rl := &rateLimiter{
		requestLimit: requestLimit,
		windowLength: windowLength,
		limitCounter: localCounter{
			counters:     make(map[uint64]*count),
			windowLength: windowLength,
		},
	}

	for _, opt := range options {
		opt(rl)
	}

	if rl.keyFn == nil {
		rl.keyFn = func(c Conn) string {
			return "*"
		}
	}

	if rl.onRequestLimit == nil {
		rl.onRequestLimit = func(c net.Conn) {
			c.Close()
		}
	}

	return rl
}

type rateLimiter struct {
	requestLimit   int
	windowLength   time.Duration
	keyFn          RateLimiterKeyFunc
	limitCounter   localCounter
	onRequestLimit func(c net.Conn)
}

func (r *rateLimiter) Status(key string) (bool, float64) {
	t := time.Now().UTC()
	currentWindow := t.Truncate(r.windowLength)
	previousWindow := currentWindow.Add(-r.windowLength)

	currCount, prevCount := r.limitCounter.Get(key, currentWindow, previousWindow)

	diff := t.Sub(currentWindow)
	rate := float64(prevCount)*(float64(r.windowLength)-float64(diff))/float64(r.windowLength) + float64(currCount)
	return rate > float64(r.requestLimit), rate
}

func (r *rateLimiter) Handler(next Handler) Handler {
	return HandlerFunc(func(c Conn) {
		key := r.keyFn(c)
		currentWindow := time.Now().UTC().Truncate(r.windowLength)

		_, rate := r.Status(key)
		nrate := int(math.Round(rate))

		if nrate >= r.requestLimit {
			r.onRequestLimit(c)
			return
		}

		r.limitCounter.Inc(key, currentWindow)
		next.ServeProtocol(c)
	})
}

type localCounter struct {
	counters     map[uint64]*count
	windowLength time.Duration
	lastEvict    time.Time
	mu           sync.Mutex
}

type count struct {
	value     int
	updatedAt time.Time
}

func (c *localCounter) Inc(key string, currentWindow time.Time) {
	c.evict()

	c.mu.Lock()
	defer c.mu.Unlock()

	hkey := limitCounterKey(key, currentWindow)

	v, ok := c.counters[hkey]
	if !ok {
		v = &count{}
		c.counters[hkey] = v
	}
	v.value += 1
	v.updatedAt = time.Now()
}

func (c *localCounter) Get(key string, currentWindow, previousWindow time.Time) (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	curr, ok := c.counters[limitCounterKey(key, currentWindow)]
	if !ok {
		curr = &count{value: 0, updatedAt: time.Now()}
	}
	prev, ok := c.counters[limitCounterKey(key, previousWindow)]
	if !ok {
		prev = &count{value: 0, updatedAt: time.Now()}
	}

	return curr.value, prev.value
}

func (c *localCounter) evict() {
	c.mu.Lock()
	defer c.mu.Unlock()

	d := c.windowLength * 3

	if time.Since(c.lastEvict) < d {
		return
	}
	c.lastEvict = time.Now()

	for k, v := range c.counters {
		if time.Since(v.updatedAt) >= d {
			delete(c.counters, k)
		}
	}
}

func limitCounterKey(key string, window time.Time) uint64 {
	h := xxhash.New()
	h.WriteString(key)
	h.WriteString(fmt.Sprintf("%d", window.Unix()))
	return h.Sum64()
}
