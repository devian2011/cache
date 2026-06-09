# Cache Library

The `cache` package is a lightweight caching library for Go designed to simplify everyday caching workflows. It serves as a unified abstraction layer over arbitrary data storage engines (such as In-Memory, Redis, or Memcached) and solves the **Cache Stampede (thundering herd)** problem through built-in `singleflight` coordination.

## Core Features
* **Deduplicated Computation (`SetOnce`)**: If 100 goroutines concurrently request a heavy computation for the exact same key, the actual evaluation function executes exactly once. The remaining 99 goroutines block safely, wait, and share the identical outcome.
* **TTL Lifecycle Management**: Native support for automatic time-based cache eviction.
* **Storage Agnostic**: A highly pluggable `Store` interface lets you shift between a standard local `sync.Map`, Redis, or Memcached seamlessly.

## Installation

To add the library to your Go module, run the following `go get` command inside your project terminal:

```bash
go get https://github.com/devian2011/cache
```

Then, import the core package along with any required sub-packages into your Go source code files:

```go
import (
	"https://github.com/devian2011/cache"
	"https://github.com/devian2011/cache/dto"
	"https://github.com/devian2011/cache/store"
	"https://github.com/devian2011/cache/normalizer"
)
```

## Usage Examples

### 1. Basic Initialization & Setup

```go
package main

import (
	"context"
	"time"

	"https://github.com/devian2011/cache"
	"https://github.com/devian2011/cache/store"      // Your MemStore/RedisStore implementation
	"https://github.com/devian2011/cache/normalizer" // Your Normalizer implementation
)

func main() {
	// Create a root cancellable context for the application lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize dependencies
	memStore := store.NewMemStore()
	norm := normalizer.NewSimpleNormalizer()

	// Create the cache instance
	cacheComponent := cache.NewCache(ctx, memStore, norm)

	// Critical step: Start the background TTL eviction loop in a separate goroutine
	go cacheComponent.clearOnTTL()
}
```

---

### 2. Standard CRUD Operations & TTL

```go
package main

import (
	"context"
	"time"

	"https://github.com/devian2011/cache"
	"https://github.com/devian2011/cache/dto"
)

func handleStandardCache(ctx context.Context, c *cache.Cache) {
	item := &dto.ItemImpl{Key: "user_session:123", Value: "active"}

	// Direct storage saving
	_ = c.Set(ctx, item)

	// Fetching an item
	val, err := c.Get(ctx, "user_session:123")
	if err == nil {
		println(val.(string)) // Output: active
	}

	// Saving with a Time-To-Live duration of 5 minutes
	_ = c.SetWithTtl(ctx, item, 5 * time.Minute)

	// Evicting keys manually
	_ = c.Delete(ctx, "user_session:123")
}
```

---

### 3. Protecting External APIs / Database Spikes via `SetOnce`

This example demonstrates how `SetOnce` deduplicates a heavy query if multiple HTTP requests try to pull the same cold data at the exact same moment.

```go
package main

import (
	"context"
	"time"

	"https://github.com/devian2011/cache"
	"https://github.com/devian2011/cache/dto"
)

func handleHeavyRequest(ctx context.Context, c *cache.Cache, userID string) (any, error) {
	// Wrap the heavy operation inside a dto.ItemOnce struct
	task := &dto.ItemOnceImpl{
		Key: "user_profile:" + userID,
		Fn: func() (any, error) {
			// This block executes ONLY ONCE if hit concurrently by multiple goroutines
			println("--> [!] Fetching profile from database (Heavy Operation)...")
			time.Sleep(2 * time.Second) // Simulate network/DB latency
			
			return "User Data Profile Object", nil
		},
	}

	// SetOnce resolves the data. If already in flight, it blocks and shares the result.
	err := c.SetOnce(ctx, task)
	if err != nil {
		return nil, err
	}

	// Safely retrieve the freshly stored data from cache
	return c.Get(ctx, "user_profile:"+userID)
}
```

---

### 4. Running Concurrent Batch Computations via `SetManyOnce`

Use this pattern when you have a slice of varied heavy tasks that need to run concurrently via a background worker queue without repeating identical jobs.

```go
package main

import (
	"context"
	"time"

	"https://github.com/devian2011/cache"
	"https://github.com/devian2011/cache/dto"
)

func processBatchJobs(ctx context.Context, c *cache.Cache) {
	jobs := []dto.ItemOnce{
		&dto.ItemOnceImpl{
			Key: "report_january",
			Fn:  func() (any, error) { return "January Analytics Data", nil },
		},
		&dto.ItemOnceImpl{
			Key: "report_february",
			Fn:  func() (any, error) { return "February Analytics Data", nil },
		},
		// If an identical key slips into the slice, singleflight intercepts and resolves it once
		&dto.ItemOnceImpl{
			Key: "report_january",
			Fn:  func() (any, error) { return "Duplicate January Analytics Data", nil },
		},
	}

	// SetManyOnce fires up internal goroutines and blocks safely until all jobs are saved
	c.SetManyOnce(ctx, jobs)
	println("All heavy batch jobs processed and cached safely!")
}
```

---

## Custom Extensions & Interfaces

The library is explicitly built around modular contracts. You can override or substitute both the key transformation strategy and the persistent storage driver by implementing the following core interfaces:

### Extending the `Normalizer` Interface
Implement this interface if you want to provide custom query parsing, case-insensitive keys, or specific hashing transformations (e.g., converting broad SQL queries into unique MD5/SHA256 cache fingerprints).

```go
type Normalizer interface {
	Normalize(key string) (string, error)
}
```

*Example Custom Implementation:*
```go
type LowercaseNormalizer struct{}

func (n *LowercaseNormalizer) Normalize(key string) (string, error) {
	if key == "" {
		return "", errors.New("empty cache key provided")
	}
	return strings.ToLower(strings.TrimSpace(key)), nil
}
```

### Extending the `Store` Interface
Implement this interface to connect the cache orchestrator to any storage provider of your choice. This allows you to hook up custom SQL backends, local filesystem caches, or third-party cloud data structures.

```go
type Store interface {
	Has(ctx context.Context, key string) (bool, error)
	Get(ctx context.Context, key string) (any, error)
	GetMany(ctx context.Context, keys []string) (map[string]any, error)
	Set(ctx context.Context, item dto.Item) error
	SetMany(ctx context.Context, items []dto.Item) error
	Delete(ctx context.Context, key string) error
	DeleteMany(ctx context.Context, keys []string) error
	Clear() error
}
```

Simply pass your custom struct matching these interfaces into the `cache.NewCache(...)` constructor to override default behavior seamlessly.

---

## Method Documentation Reference

### Data Operations

#### `Get(ctx context.Context, key string) (any, error)`
Retrieves a single entry directly from the underlying storage backend.
* **Returns:** The stored value or a `key not found` error if missing.

```go
val, err := cacheComponent.Get(ctx, "user:session:100")
if err != nil {
    // Handle cache miss or connection error
    return
}
fmt.Println("Session active:", val)
```

#### `GetMany(ctx context.Context, keys []string) (map[string]any, error)`
Performs a batch read for a slice of keys.
* **Returns:** A `map[string]any` containing only the subset of keys that physically exist.

```go
keys := []string{"config:theme", "config:lang", "config:mode"}
results, err := cacheComponent.GetMany(ctx, keys)
if err != nil {
    log.Fatal(err)
}

if lang, ok := results["config:lang"]; ok {
    fmt.Println("Language set to:", lang)
}
```

#### `Set(ctx context.Context, item dto.Item) error`
Persists a key-value pair immediately into the backend store.
* **Behavior:** Bypasses both the `singleflight` deduplication system and TTL scheduling.

```go
newItem := &dto.ItemImpl{Key: "system:status", Value: "healthy"}
err := cacheComponent.Set(ctx, newItem)
```

#### `SetMany(ctx context.Context, items []dto.Item) error`
Executes a direct batch write operation for a slice of items.

```go
items := []dto.Item{
    &dto.ItemImpl{Key: "metric:cpu", Value: 42.5},
    &dto.ItemImpl{Key: "metric:ram", Value: 78.1},
}
err := cacheComponent.SetMany(ctx, items)
```

---

### Concurrent Protection & Deduplication

#### `SetOnce(ctx context.Context, once dto.ItemOnce) error`
Shields downstream dependencies from load spikes on cold entries. It wraps an expensive evaluation task (e.g., heavy database or external API calls) and coordinates incoming parallel demands.
* **Mechanism:** If multiple goroutines trigger this simultaneously for the same key, the evaluation function executes exactly once. All callers wait and seamlessly share the exact same outcome.

```go
task := &dto.ItemOnceImpl{
    Key: "heavy:db:query:result",
    Fn: func() (any, error) {
        return database.FetchComplexAnalytics()
    },
}

err := cacheComponent.SetOnce(ctx, task)
```

#### `SetManyOnce(ctx context.Context, onceList []dto.ItemOnce)`
Schedules a slice of heavy tasks concurrently via an optimized goroutine pool.
* **Behavior:** Safely clusters overlapping keys during execution to completely isolate backend data engines from redundant computations.

```go
batchTasks := []dto.ItemOnce{
    &dto.ItemOnceImpl{Key: "report:q1", Fn: fetchQ1Report},
    &dto.ItemOnceImpl{Key: "report:q2", Fn: fetchQ2Report},
}
