package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bradfitz/gomemcache/memcache"

	"github.com/devian2011/cache/dto"
)

type MemcachedStore struct {
	client *memcache.Client
}

func NewMemcachedStore(client *memcache.Client) *MemcachedStore {
	return &MemcachedStore{client: client}
}

func (s *MemcachedStore) Has(ctx context.Context, key string) (bool, error) {
	_, err := s.client.Get(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *MemcachedStore) Get(ctx context.Context, key string) (any, error) {
	item, err := s.client.Get(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil, errors.New("key not found")
	} else if err != nil {
		return nil, err
	}

	var value any
	if err := json.Unmarshal(item.Value, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (s *MemcachedStore) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	if len(keys) == 0 {
		return make(map[string]any), nil
	}

	items, err := s.client.GetMulti(keys)
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	for key, item := range items {
		var decoded any
		if err := json.Unmarshal(item.Value, &decoded); err == nil {
			result[key] = decoded
		}
	}

	return result, nil
}

func (s *MemcachedStore) Set(ctx context.Context, item dto.Item) error {
	data, err := json.Marshal(item.GetValue())
	if err != nil {
		return err
	}

	return s.client.Set(&memcache.Item{
		Key:        item.GetKey(),
		Value:      data,
		Expiration: 0, // 0 означает отсутствие автоматического истечения TTL
	})
}

func (s *MemcachedStore) SetMany(ctx context.Context, items []dto.Item) error {
	for _, item := range items {
		if err := s.Set(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *MemcachedStore) Delete(ctx context.Context, key string) error {
	err := s.client.Delete(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil
	}
	return err
}

func (s *MemcachedStore) DeleteMany(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *MemcachedStore) Clear() error {
	return s.client.FlushAll()
}
