package store

import (
	"context"
	"errors"
	"sync"
	
	"github.com/devian2011/cache/dto"
)

type MemStore struct {
	cache sync.Map
}

func NewMemStore() *MemStore {
	return &MemStore{}
}

// Has checks the physical existence of a key in the storage backend.
func (s *MemStore) Has(_ context.Context, key string) (bool, error) {
	_, exists := s.cache.Load(key)
	return exists, nil
}

// Set saves a single dto.Item directly into the storage backend.
func (s *MemStore) Set(_ context.Context, item dto.Item) error {
	s.cache.Store(item.GetKey(), item.GetValue())
	return nil
}

// SetMany processes a batch save for multiple dto.Item structures.
func (s *MemStore) SetMany(_ context.Context, items []dto.Item) error {
	for _, item := range items {
		s.cache.Store(item.GetKey(), item.GetValue())
	}
	return nil
}

// Get retrieves the value of a single key. Returns an error if the key is missing.
func (s *MemStore) Get(_ context.Context, key string) (any, error) {
	t, ok := s.cache.Load(key)
	if !ok {
		return nil, errors.New("key not found")
	}
	return t, nil
}

// GetMany batch-reads a slice of keys and returns a map of existing values.
func (s *MemStore) GetMany(_ context.Context, keys []string) (map[string]any, error) {
	result := make(map[string]any, len(keys))
	for _, key := range keys {
		item, ok := s.cache.Load(key)
		if ok {
			result[key] = item
		}
	}

	return result, nil
}

// Delete removes an element by its unique key.
func (s *MemStore) Delete(_ context.Context, key string) error {
	s.cache.Delete(key)
	return nil
}

// DeleteMany handles batch removal for a slice of keys.
func (s *MemStore) DeleteMany(_ context.Context, keys []string) error {
	for _, key := range keys {
		s.cache.Delete(key)
	}

	return nil
}

// Clear completely purges all data from the storage engine.
func (s *MemStore) Clear() error {
	s.cache = sync.Map{}
	return nil
}
