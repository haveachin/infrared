package infrared

import (
	"errors"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

type RateLimiterConfig struct {
	RequestLimit int           `yaml:"requestLimit"`
	WindowLength time.Duration `yaml:"windowLength"`
}

func RateLimit(requestLimit int, windowLength time.Duration, options ...RateLimiterOption) Filterer {
	return newRateLimiter(requestLimit, windowLength, options...).Filterer()
}

func RateLimitByIP(requestLimit int, windowLength time.Duration) Filterer {
	return RateLimit(requestLimit, windowLength, WithKeyByIP())
}

func KeyByIP(c net.Conn) string {
	rAddr := c.RemoteAddr().String()
	ip, _, err := net.SplitHostPort(rAddr)
	if err != nil {
		ip = rAddr
	}
	return canonicalizeIP(ip)
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
	return func(c net.Conn) string {
		var key strings.Builder
		for i := 0; i < len(keyFuncs); i++ {
			k := keyFuncs[i](c)
			key.WriteString(k)
		}
		return key.String()
	}
}

type RateLimiterKeyFunc func(c net.Conn) string
type RateLimiterOption func(rl *rateLimiter)

// canonicalizeIP returns a form of ip suitable for comparison to other IPs.
// For IPv4 addresses, this is simply the whole string.
// For IPv6 addresses, this is the /64 prefix.
// https://github.com/didip/tollbooth/blob/v6.1.2/libstring/libstring.go#L57-L102
func canonicalizeIP(ip string) string {
	isIPv6 := false
	// This is how net.ParseIP decides if an address is IPv6
	// https://cs.opensource.google/go/go/+/refs/tags/go1.17.7:src/net/ip.go;l=704
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

	// By default, the string representation of a net.IPNet (masked IP address) is just
	// "full_address/mask_bits". But using that will result in different addresses with
	// the same /64 prefix comparing differently. So we need to zero out the last 64 bits
	// so that all IPs in the same prefix will be the same.
	//
	// Note: When 1.18 is the minimum Go version, this can be written more cleanly like:
	// netip.PrefixFrom(netip.MustParseAddr(ipv6), 64).Masked().Addr().String()
	// (With appropriate error checking.)

	ipv6 := net.ParseIP(ip)
	if ipv6 == nil {
		return ip
	}

	const bytesToZero = (128 - 64) / 8
	for i := len(ipv6) - bytesToZero; i < len(ipv6); i++ {
		ipv6[i] = 0
	}

	// Note that this doesn't have the "/64" suffix customary with a CIDR representation,
	// but those three bytes add nothing for us.
	return ipv6.String()
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
		rl.keyFn = func(c net.Conn) string {
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

var ErrRateLimitReached = errors.New("rate limit reached")

func (r *rateLimiter) Filterer() Filterer {
	return FilterFunc(func(c net.Conn) error {
		key := r.keyFn(c)
		currentWindow := time.Now().UTC().Truncate(r.windowLength)

		_, rate := r.Status(key)
		nrate := int(math.Round(rate))

		if nrate >= r.requestLimit {
			r.onRequestLimit(c)
			return ErrRateLimitReached
		}

		r.limitCounter.Inc(key, currentWindow)
		return nil
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
	v.value++
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

	if time.Since(c.lastEvict) < c.windowLength {
		return
	}
	c.lastEvict = time.Now()

	for k, v := range c.counters {
		if time.Since(v.updatedAt) >= c.windowLength {
			delete(c.counters, k)
		}
	}
}

func limitCounterKey(key string, window time.Time) uint64 {
	h := xxhash.New()
	_, _ = h.WriteString(key)
	_, _ = h.WriteString(strconv.FormatInt(window.Unix(), 10))
	return h.Sum64()
}
