package runner

import "sync"

func NewConcurrentMap[K comparable, V any]() *ConcurrentMap[K, V] {
	return &ConcurrentMap[K, V]{
		mu:    &sync.Mutex{},
		inner: map[K]V{},
	}
}

type ConcurrentMap[K comparable, V any] struct {
	mu    *sync.Mutex
	inner map[K]V
}

func (c *ConcurrentMap[K, V]) Get(key K) V {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.inner[key]
}

func (c *ConcurrentMap[K, V]) MaybeGet(key K) (val V, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	val, ok = c.inner[key]
	return
}

func (c *ConcurrentMap[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inner[key] = value
}

func (c *ConcurrentMap[K, _]) Contains(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.inner[key]
	return ok
}

func (c *ConcurrentMap[K, _]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()
	ret := make([]K, 0, len(c.inner))
	for k := range c.inner {
		ret = append(ret, k)
	}
	return ret
}

func (c *ConcurrentMap[K, V]) Map() map[K]V {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := map[K]V{}
	for k, v := range c.inner {
		m[k] = v
	}
	return m
}

func (c *ConcurrentMap[K, V]) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inner = map[K]V{}
}
