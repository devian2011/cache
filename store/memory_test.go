package store

import (
	"context"
	"sync"
	"testing"

	"github.com/devian2011/cache/dto"
)

type mockItem struct {
	key   string
	value any
}

func (m *mockItem) GetKey() string { return m.key }
func (m *mockItem) GetValue() any  { return m.value }

func TestMemStore_BaseOperations(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()

	item := &mockItem{key: "user_1", value: "John"}
	err := s.Set(ctx, item)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := s.Get(ctx, "user_1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val.(string) != "John" {
		t.Errorf("Expected 'John', got %v", val)
	}

	if !s.Has(ctx, "user_1") {
		t.Error("Expected key 'user_1' to exist")
	}
	if s.Has(ctx, "non_existent") {
		t.Error("Expected key 'non_existent' to not exist")
	}

	_, err = s.Get(ctx, "unknown")
	if err == nil || err.Error() != "key not found" {
		t.Errorf("Expected 'key not found' error, got: %v", err)
	}

	err = s.Delete(ctx, "user_1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.Has(ctx, "user_1") {
		t.Error("Key 'user_1' should be deleted")
	}
}

func TestMemStore_BatchOperations(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()

	items := []dto.Item{
		&mockItem{key: "k1", value: "v1"},
		&mockItem{key: "k2", value: "v2"},
		&mockItem{key: "k3", value: "v3"},
	}

	err := s.SetMany(ctx, items)
	if err != nil {
		t.Fatalf("SetMany failed: %v", err)
	}

	res, err := s.GetMany(ctx, []string{"k1", "k3", "k4"})
	if err != nil {
		t.Fatalf("GetMany failed: %v", err)
	}

	if len(res) != 2 {
		t.Errorf("Expected 2 items in result, got %d", len(res))
	}
	if res["k1"] != "v1" || res["k3"] != "v3" {
		t.Errorf("Unexpected GetMany results: %v", res)
	}
	if _, exists := res["k4"]; exists {
		t.Error("Key 'k4' should not be in the result")
	}

	err = s.DeleteMany(ctx, []string{"k1", "k2"})
	if err != nil {
		t.Fatalf("DeleteMany failed: %v", err)
	}

	if s.Has(ctx, "k1") || s.Has(ctx, "k2") {
		t.Error("Keys k1 and k2 should have been deleted")
	}
	if !s.Has(ctx, "k3") {
		t.Error("Key k3 should still exist")
	}
}

func TestMemStore_Clear(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()

	_ = s.Set(ctx, &mockItem{key: "temp", value: 42})

	err := s.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if s.Has(ctx, "temp") {
		t.Error("Store should be empty after Clear")
	}
}

func TestMemStore_Concurrency(t *testing.T) {
	ctx := context.Background()
	s := NewMemStore()

	var wg sync.WaitGroup
	workers := 50
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(2)

		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = s.Set(ctx, &mockItem{
					key:   "key",
					value: workerID,
				})
			}
		}(i)

		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = s.Get(ctx, "key")
				_ = s.Has(ctx, "key")
			}
		}()
	}

	wg.Wait()
}
