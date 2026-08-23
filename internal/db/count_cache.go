package db

import (
	"sync"
	"time"
)

const countCacheTTL = 30 * time.Second

type ttlCountEntry struct {
	value int64
	exp   time.Time
}

type ttlCountCache struct {
	mu sync.Mutex
	m  map[string]ttlCountEntry
}

func newTTLCountCache() *ttlCountCache {
	return &ttlCountCache{m: make(map[string]ttlCountEntry)}
}

func (c *ttlCountCache) get(key string) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.exp) {
		return 0, false
	}
	return e.value, true
}

func (c *ttlCountCache) put(key string, v int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) > 4096 {
		now := time.Now()
		for k, e := range c.m {
			if now.After(e.exp) {
				delete(c.m, k)
			}
		}
		if len(c.m) > 4096 {
			c.m = make(map[string]ttlCountEntry)
		}
	}
	c.m[key] = ttlCountEntry{value: v, exp: time.Now().Add(countCacheTTL)}
}

func (d *Database) cachedCount(key string, fn func() (int64, error)) (int64, error) {
	if d.counts != nil {
		if v, ok := d.counts.get(key); ok {
			return v, nil
		}
	}
	v, err := fn()
	if err != nil {
		return 0, err
	}
	if d.counts != nil {
		d.counts.put(key, v)
	}
	return v, nil
}
