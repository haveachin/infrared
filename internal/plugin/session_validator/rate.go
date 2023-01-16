package session_validator

import (
	"math"
	"sync"
	"time"
)

type cpsCounter struct {
	lastEvict time.Time
	counters  map[time.Time]uint32
	mu        sync.Mutex
}

func (c *cpsCounter) CPS() uint32 {
	t := time.Now()
	currWindow := t.Truncate(time.Second)
	currCount, prevCount := c.get()
	diff := t.Sub(currWindow)
	rate := float64(prevCount)*(float64(time.Second)-float64(diff))/float64(time.Second) + float64(currCount)
	return uint32(math.Round(rate))
}

func (c *cpsCounter) Inc() {
	key := time.Now().UTC().Truncate(time.Second)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.evict()
	if _, ok := c.counters[key]; !ok {
		c.counters[key] = 1
		return
	}
	c.counters[key]++
}

func (c *cpsCounter) get() (uint32, uint32) {
	currkey := time.Now().UTC().Truncate(time.Second)
	prevKey := currkey.Add(-time.Second)

	c.mu.Lock()
	defer c.mu.Unlock()

	currCount, ok := c.counters[currkey]
	if !ok {
		currCount = 0
	}

	prevCount, ok := c.counters[prevKey]
	if !ok {
		prevCount = 0
	}

	return currCount, prevCount
}

func (c *cpsCounter) evict() {
	if time.Since(c.lastEvict) < time.Second {
		return
	}
	c.lastEvict = time.Now()

	for k := range c.counters {
		if time.Since(k) >= time.Second*3 {
			delete(c.counters, k)
		}
	}
}
