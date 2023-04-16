package main

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

const DefaultTTL time.Duration = 0

type Cache struct {
	size    int
	ttl     time.Duration
	storage map[string]*list.Element
	lru     *list.List
	mu      sync.RWMutex
}

type Item struct {
	ttl       time.Duration
	key       string
	value     any
	expiresAt time.Time
}

func NewCache(size int, ttl time.Duration) *Cache {
	return &Cache{
		size:    size,
		ttl:     ttl,
		storage: make(map[string]*list.Element, size),
		lru:     list.New(),
	}
}

func (c *Cache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.storage[key]

	var item Item

	if !ok {
		if c.lru.Len() >= c.size {
			old := c.lru.Back()
			c.lru.Remove(old)
			delete(c.storage, old.Value.(Item).key)
		}

		item = Item{
			key:       key,
			value:     value,
			ttl:       ttl,
			expiresAt: time.Now().Add(ttl),
		}

		e = c.lru.PushFront(item)

		c.storage[key] = e

	} else {
		item = e.Value.(Item)

		item.value = value
		item.ttl = ttl

		c.lru.MoveToFront(e)
	}

	if item.ttl > 0 || c.ttl > 0 {
		if c.ttl > 0 && item.ttl == 0 {
			item.ttl = c.ttl
		}
		item.expiresAt = time.Now().Add(item.ttl)
	}

	e.Value = item
}

func (c *Cache) Get(key string) (value any, success bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.storage[key]

	if !ok || (ok && e.Value.(Item).expiresAt.Before(time.Now())) {
		return nil, false
	}

	item := e.Value.(Item)

	if item.ttl > 0 || c.ttl > 0 {
		if c.ttl > 0 && item.ttl == 0 {
			item.ttl = c.ttl
		}
		item.expiresAt = time.Now().Add(item.ttl)
	}

	e.Value = item
	c.lru.MoveToFront(e)

	return item.value, ok
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.storage[key]; ok {
		c.lru.Remove(e)
		delete(c.storage, key)
	}
}

func main() {
	mainRace()
}

func mainRace() {
	cache := NewCache(3, 10*time.Second)
	wg := sync.WaitGroup{}

	cache.Set("name", "Alex", DefaultTTL)
	cache.Set("hobby", "BJJ", DefaultTTL)

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println(cache.Get("name"))
		fmt.Println(cache.Get("hobby"))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		cache.Delete("hobby")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println(cache.Get("hobby"))
		fmt.Println(cache.Get("name"))
	}()

	wg.Wait()
}
