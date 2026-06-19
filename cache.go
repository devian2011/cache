// Package cache provides a wrapper over data stores with support for heavy
// write request deduplication using singleflight and an integrated TTL mechanism.
package cache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/devian2011/cache/dto"
)

// Normalizer defines the interface for standardizing and cleaning up cache keys.
type Normalizer interface {
	// Normalize accepts a raw key and returns its standardized version.
	Normalize(key string) (string, error)
}

// Store describes an abstract data storage engine (e.g., Redis, Memcached, or In-Memory).
type Store interface {
	// Has checks the physical existence of a key in the storage backend.
	Has(ctx context.Context, key string) (bool, error)
	// Get retrieves the value of a single key. Returns an error if the key is missing.
	Get(context.Context, string) (any, error)
	// GetMany batch-reads a slice of keys and returns a map of existing values.
	GetMany(context.Context, []string) (map[string]any, error)
	// Set saves a single dto.Item directly into the storage backend.
	Set(context.Context, dto.Item) error
	// SetMany processes a batch save for multiple dto.Item structures.
	SetMany(context.Context, []dto.Item) error
	// Delete removes an element by its unique key.
	Delete(context.Context, string) error
	// DeleteMany handles batch removal for a slice of keys.
	DeleteMany(context.Context, []string) error
	// Clear completely purges all data from the storage engine.
	Clear() error
}

// Cache wraps a Store implementation, enhancing it with the singleflight pattern and TTL lifecycle management.
type Cache struct {
	ctx   context.Context
	store Store
	sf    *singleflight.Group
	n     Normalizer

	ttlMapMtx *sync.Mutex
	ttlMap    map[string]time.Time
}

// NewCache initializes and returns a pointer to a new Cache instance.
// To activate the automatic TTL cleanup mechanism, make sure to execute
// the c.clearOnTTL() method in a separate goroutine after initialization.
func NewCache(ctx context.Context, store Store, n Normalizer) *Cache {
	c := &Cache{
		ctx:       ctx,
		store:     store,
		sf:        &singleflight.Group{},
		n:         n,
		ttlMapMtx: &sync.Mutex{},
		ttlMap:    make(map[string]time.Time),
	}

	go c.clearOnTTL()

	return c
}

// clearOnTTL is an internal worker loop that scans the ttlMap once per second
// and physically deletes expired keys from the underlying Store engine.
func (c *Cache) clearOnTTL() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(time.Second):
			c.ttlMapMtx.Lock()
			now := time.Now()
			for k, v := range c.ttlMap {
				if now.After(v) {
					c.store.Delete(context.Background(), k)
					delete(c.ttlMap, k)
				}
			}
			c.ttlMapMtx.Unlock()
		}
	}
}

// SetOnce evaluates a heavy operation wrapped inside a dto.ItemOnce container and stores its outcome.
// Utilizing singleflight ensures that if multiple concurrent goroutines trigger SetOnce using identical keys,
// the evaluation function triggers exactly once, effectively shielding the data source from heavy request spikes.
func (c *Cache) SetOnce(ctx context.Context, once dto.ItemOnce) error {
	key, err := c.n.Normalize(once.GetKey())
	if err != nil {
		key = once.GetKey()
	}

	val, err, _ := c.sf.Do(key, func() (any, error) {
		return once.GetValue()()
	})
	if err != nil {
		return err
	}

	return c.store.Set(ctx, &dto.ItemImpl{
		Key:   key,
		Value: val,
	})
}

// SetManyOnce concurrently schedules a batch of dto.ItemOnce evaluation tasks using the SetOnce method.
// It groups identical keys together seamlessly, protecting systems against redundant parallel computations.
func (c *Cache) SetManyOnce(ctx context.Context, onceList []dto.ItemOnce) {
	onceUpdatedList := make([]dto.ItemOnce, 0, len(onceList))
	for _, once := range onceList {
		k, e := c.n.Normalize(once.GetKey())
		if e != nil {
			k = once.GetKey()
		}
		onceUpdatedList = append(onceUpdatedList, &dto.ItemOnceImpl{
			Key: k,
			Fn:  once.GetValue(),
		})
	}

	var wg sync.WaitGroup

	wg.Add(len(onceList))

	for _, once := range onceList {
		go func(o dto.ItemOnce) {
			defer wg.Done()
			_ = c.SetOnce(ctx, o)
		}(once)
	}

	wg.Wait()
}

// Get fetches the value corresponding to a key directly from the underlying Store engine.
func (c *Cache) Get(ctx context.Context, key string) (any, error) {
	key, err := c.n.Normalize(key)
	if err != nil {
		return nil, err
	}
	return c.store.Get(ctx, key)
}

// GetMany pulls a map containing values for a specified slice of keys directly from the Store engine.
// The map will omit keys that do not physically exist inside the store.
func (c *Cache) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	k := make([]string, 0, len(keys))
	for _, key := range keys {
		mk, e := c.n.Normalize(key)
		if e != nil {
			mk = key
		}
		k = append(k, mk)
	}

	return c.store.GetMany(ctx, k)
}

// Set saves an item directly into the underlying Store, skipping singleflight logic or TTL records.
func (c *Cache) Set(ctx context.Context, item dto.Item) error {
	key, err := c.n.Normalize(item.GetKey())
	if err != nil {
		return err
	}
	return c.store.Set(ctx, &dto.ItemImpl{
		Key:   key,
		Value: item.GetValue(),
	})
}

// SetOnceWithTtl evaluates a heavy operation wrapped inside a dto.ItemOnce container,
// saves its outcome into the underlying Store, and sets an expiration timestamp.
// Utilizing singleflight ensures duplicate concurrent calls execute the evaluation function
// exactly once, while the background worker handles eviction after the duration expires.
func (c *Cache) SetOnceWithTtl(ctx context.Context, once dto.ItemOnce, ttl time.Duration) error {
	key, err := c.n.Normalize(once.GetKey())
	if err != nil {
		key = once.GetKey()
	}

	val, err, _ := c.sf.Do(key, func() (any, error) {
		return once.GetValue()()
	})
	if err != nil {
		return err
	}

	return c.SetWithTtl(ctx, &dto.ItemImpl{Key: once.GetKey(), Value: val}, ttl)
}

// SetWithTtl registers an item inside the Store and maps its expiration timestamp within the internal ttlMap.
// Once the specified duration passes, the background worker routine will automatically evict the key.
func (c *Cache) SetWithTtl(ctx context.Context, item dto.Item, duration time.Duration) error {
	key, err := c.n.Normalize(item.GetKey())
	if err != nil {
		return err
	}
	err = c.store.Set(ctx, &dto.ItemImpl{
		Key:   key,
		Value: item.GetValue(),
	})
	if err != nil {
		return err
	}
	c.ttlMapMtx.Lock()
	defer c.ttlMapMtx.Unlock()
	c.ttlMap[item.GetKey()] = time.Now().Add(duration)

	return nil
}

// SetMany persists a slice of elements directly into the underling Store in a batch mode.
func (c *Cache) SetMany(ctx context.Context, items []dto.Item) error {
	iList := make([]dto.Item, 0, len(items))
	for _, item := range items {
		k, e := c.n.Normalize(item.GetKey())
		if e != nil {
			k = item.GetKey()
		}
		iList = append(iList, &dto.ItemImpl{
			Key:   k,
			Value: item.GetValue(),
		})
	}

	return c.store.SetMany(ctx, items)
}

// Delete evicts an element from the Store and explicitly clears out any running singleflight locks for that specific key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	key, err := c.n.Normalize(key)
	if err != nil {
		return err
	}
	c.sf.Forget(key)
	return c.store.Delete(ctx, key)
}

// DeleteMany removes a batch of keys from the Store and safely resets singleflight tracking for each of them.
func (c *Cache) DeleteMany(ctx context.Context, keys []string) error {
	k := make([]string, 0, len(keys))
	for _, key := range keys {
		mk, e := c.n.Normalize(key)
		if e != nil {
			mk = key
		}
		k = append(k, mk)
	}

	for _, key := range k {
		c.sf.Forget(key)
	}
	return c.store.DeleteMany(ctx, k)
}

// Clear wipes the underlying Store and re-instantiates a clean, decoupled singleflight.Group instance.
func (c *Cache) Clear() error {
	c.sf = &singleflight.Group{}
	return c.store.Clear()
}
