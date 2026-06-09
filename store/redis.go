package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"

	"github.com/devian2011/cache/dto"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Has(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (s *RedisStore) Get(ctx context.Context, key string) (any, error) {
	data, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, errors.New("key not found")
	} else if err != nil {
		return nil, err
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (s *RedisStore) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	if len(keys) == 0 {
		return make(map[string]any), nil
	}

	vals, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	for i, val := range vals {
		if val == nil {
			continue
		}

		str, ok := val.(string)
		if !ok {
			continue
		}

		var decoded any
		if err := json.Unmarshal([]byte(str), &decoded); err == nil {
			result[keys[i]] = decoded
		}
	}

	return result, nil
}

func (s *RedisStore) Set(ctx context.Context, item dto.Item) error {
	data, err := json.Marshal(item.GetValue())
	if err != nil {
		return err
	}

	return s.client.Set(ctx, item.GetKey(), data, 0).Err()
}

func (s *RedisStore) SetMany(ctx context.Context, items []dto.Item) error {
	if len(items) == 0 {
		return nil
	}

	pairs := make([]any, 0, len(items)*2)
	for _, item := range items {
		data, err := json.Marshal(item.GetValue())
		if err != nil {
			return err
		}
		pairs = append(pairs, item.GetKey(), data)
	}

	return s.client.MSet(ctx, pairs...).Err()
}

func (s *RedisStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *RedisStore) DeleteMany(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(ctx, keys...).Err()
}

func (s *RedisStore) Clear() error {
	return s.client.FlushDB(context.Background()).Err()
}
