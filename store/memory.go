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

func (s *MemStore) Has(_ context.Context, key string) bool {
	_, exists := s.cache.Load(key)
	return exists
}

func (s *MemStore) Set(_ context.Context, item dto.Item) error {
	s.cache.Store(item.GetKey(), item.GetValue())
	return nil
}

func (s *MemStore) SetMany(_ context.Context, items []dto.Item) error {
	for _, item := range items {
		s.cache.Store(item.GetKey(), item.GetValue())
	}
	return nil
}

func (s *MemStore) Get(_ context.Context, key string) (any, error) {
	t, ok := s.cache.Load(key)
	if !ok {
		return nil, errors.New("key not found")
	}
	return t, nil
}

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

func (s *MemStore) Delete(_ context.Context, key string) error {
	s.cache.Delete(key)
	return nil
}

func (s *MemStore) DeleteMany(_ context.Context, keys []string) error {
	for _, key := range keys {
		s.cache.Delete(key)
	}

	return nil
}

func (s *MemStore) Clear() error {
	s.cache = sync.Map{}
	return nil
}
