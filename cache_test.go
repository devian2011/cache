package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devian2011/cache/dto"
)

type mockItem struct {
	key   string
	value any
}

func (m *mockItem) GetKey() string { return m.key }
func (m *mockItem) GetValue() any  { return m.value }

type mockItemOnce struct {
	key string
	fn  func() (any, error)
}

func (m *mockItemOnce) GetKey() string                { return m.key }
func (m *mockItemOnce) GetValue() func() (any, error) { return m.fn }

type mockNormalizer struct {
	err error
}

func (n *mockNormalizer) Normalize(key string) (string, error) {
	if n.err != nil {
		return "", n.err
	}
	return key, nil
}

type mockStore struct {
	mu    sync.Mutex
	data  map[string]any
	calls map[string]int
}

func newMockStore() *mockStore {
	return &mockStore{
		data:  make(map[string]any),
		calls: make(map[string]int),
	}
}

func (m *mockStore) Has(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockStore) Get(ctx context.Context, key string) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.data[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return val, nil
}

func (m *mockStore) Set(ctx context.Context, item dto.Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[item.GetKey()] = item.GetValue()
	m.calls["Set"]++
	return nil
}

func (m *mockStore) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	return nil, nil
}
func (m *mockStore) SetMany(ctx context.Context, items []dto.Item) error { return nil }
func (m *mockStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}
func (m *mockStore) DeleteMany(ctx context.Context, keys []string) error { return nil }
func (m *mockStore) Clear() error                                        { return nil }

func TestCache_BaseProxy(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	norm := &mockNormalizer{}
	c := NewCache(ctx, store, norm)

	item := &mockItem{key: "test_key", value: "test_val"}
	err := c.Set(ctx, item)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := c.Get(ctx, "test_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if val.(string) != "test_val" {
		t.Errorf("Expected test_val, got %v", val)
	}
}

func TestCache_SetOnce_SingleflightEffect(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	norm := &mockNormalizer{}
	c := NewCache(ctx, store, norm)

	var fnExecutionCount int32
	var wg sync.WaitGroup

	heavyFn := func() (any, error) {
		atomic.AddInt32(&fnExecutionCount, 1)
		time.Sleep(50 * time.Millisecond)
		return "computed_value", nil
	}

	onceItem := &mockItemOnce{
		key: "heavy_task",
		fn:  heavyFn,
	}

	workers := 10
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_ = c.SetOnce(ctx, onceItem)
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&fnExecutionCount) != 1 {
		t.Errorf("heavyFn executed %d times, expected exactly 1", fnExecutionCount)
	}

	storedVal, _ := store.Get(ctx, "heavy_task")
	if storedVal != "computed_value" {
		t.Errorf("Expected 'computed_value' in store, got %v", storedVal)
	}
}

func TestCache_SetManyOnce(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	norm := &mockNormalizer{}
	c := NewCache(ctx, store, norm)

	onceList := []dto.ItemOnce{
		&mockItemOnce{key: "k1", fn: func() (any, error) { return "v1", nil }},
		&mockItemOnce{key: "k2", fn: func() (any, error) { return "v2", nil }},
	}

	done := make(chan bool)
	go func() {
		c.SetManyOnce(ctx, onceList)
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SetManyOnce deadlocked! Check wg.Add count.")
	}
}

func TestCache_TTL_Expiry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := newMockStore()
	norm := &mockNormalizer{}
	c := NewCache(ctx, store, norm)

	go c.clearOnTTL()

	item := &mockItem{key: "short_live", value: "alive"}

	err := c.SetWithTtl(ctx, item, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("SetWithTtl failed: %v", err)
	}

	if exists, _ := store.Has(ctx, "short_live"); !exists {
		t.Fatal("Item should be in store immediately after SetWithTtl")
	}

	time.Sleep(1500 * time.Millisecond)

	exists, _ := store.Has(ctx, "short_live")
	if exists {
		t.Error("Item should have been deleted by TTL background worker")
	}
}

func TestCache_Delete_Forget(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	norm := &mockNormalizer{}
	c := NewCache(ctx, store, norm)

	once1 := &mockItemOnce{key: "key1", fn: func() (any, error) { return "val1", nil }}
	_ = c.SetOnce(ctx, once1)

	err := c.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	var secondTime bool
	once2 := &mockItemOnce{key: "key1", fn: func() (any, error) {
		secondTime = true
		return "val2", nil
	}}

	_ = c.SetOnce(ctx, once2)

	if !secondTime {
		t.Error("Singleflight Forget didn't work: second SetOnce call was skipped")
	}
}
