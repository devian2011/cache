package store

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/devian2011/cache/dto"
)

func setupTestRedis(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return NewRedisStore(client), mr
}

func TestRedisStore_Has(t *testing.T) {
	ctx := context.Background()
	s, mr := setupTestRedis(t)
	defer mr.Close()

	mr.Set("exist_key", `"value"`)

	has, err := s.Has(ctx, "exist_key")
	if err != nil {
		t.Fatalf("Has failed: %v", err)
	}
	if !has {
		t.Error("expected key to exist")
	}

	has, err = s.Has(ctx, "non_exist_key")
	if err != nil {
		t.Fatalf("Has failed: %v", err)
	}
	if has {
		t.Error("expected key to not exist")
	}
}

func TestRedisStore_Get(t *testing.T) {
	ctx := context.Background()
	s, mr := setupTestRedis(t)
	defer mr.Close()

	mr.Set("string_key", `"hello"`)
	mr.Set("invalid_json", `{"bad_json`)

	val, err := s.Get(ctx, "string_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "hello" {
		t.Errorf("got %v, want hello", val)
	}

	_, err = s.Get(ctx, "non_existent")
	if err == nil || err.Error() != "key not found" {
		t.Errorf("expected 'key not found' error, got %v", err)
	}

	_, err = s.Get(ctx, "invalid_json")
	if err == nil {
		t.Error("expected json unmarshal error, got nil")
	}
}

func TestRedisStore_GetMany(t *testing.T) {
	ctx := context.Background()
	s, mr := setupTestRedis(t)
	defer mr.Close()

	res, err := s.GetMany(ctx, []string{})
	if err != nil {
		t.Fatalf("GetMany empty keys failed: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected empty map, got %v", res)
	}

	mr.Set("key1", `"val1"`)
	mr.Set("key2", `123`)
	mr.Set("key_bad", `[invalid`)

	res, err = s.GetMany(ctx, []string{"key1", "key2", "key_missing", "key_bad"})
	if err != nil {
		t.Fatalf("GetMany failed: %v", err)
	}

	if res["key1"] != "val1" {
		t.Errorf("expected val1, got %v", res["key1"])
	}
	if res["key2"] != float64(123) { // json.Unmarshal парсит числа как float64 по умолчанию
		t.Errorf("expected 123, got %v", res["key2"])
	}
	if _, ok := res["key_missing"]; ok {
		t.Error("expected key_missing to be absent from result map")
	}
	if _, ok := res["key_bad"]; ok {
		t.Error("expected key_bad to be skipped due to invalid json")
	}
}

func TestRedisStore_Set(t *testing.T) {
	ctx := context.Background()
	s, mr := setupTestRedis(t)
	defer mr.Close()

	item := &mockItem{key: "new_key", value: "stored_value"}
	err := s.Set(ctx, item)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	redisVal, err := mr.Get("new_key")
	if err != nil {
		t.Fatalf("miniredis Get failed: %v", err)
	}
	if redisVal != `"stored_value"` {
		t.Errorf("got %s, want %s", redisVal, `"stored_value"`)
	}

	badItem := &mockItem{key: "bad", value: make(chan int)} // Каналы нельзя закодировать в JSON
	err = s.Set(ctx, badItem)
	if err == nil {
		t.Error("expected json marshal error, got nil")
	}
}

func TestRedisStore_SetMany(t *testing.T) {
	ctx := context.Background()
	s, mr := setupTestRedis(t)
	defer mr.Close()

	err := s.SetMany(ctx, []dto.Item{})
	if err != nil {
		t.Fatalf("SetMany empty slice failed: %v", err)
	}

	items := []dto.Item{
		&mockItem{key: "m1", value: "v1"},
		&mockItem{key: "m2", value: "v2"},
	}

	err = s.SetMany(ctx, items)
	if err != nil {
		t.Fatalf("SetMany failed: %v", err)
	}

	if v, _ := mr.Get("m1"); v != `"v1"` {
		t.Errorf("expected v1, got %s", v)
	}
	if v, _ := mr.Get("m2"); v != `"v2"` {
		t.Errorf("expected v2, got %s", v)
	}

	badItems := []dto.Item{
		&mockItem{key: "m3", value: make(chan int)},
	}
	err = s.SetMany(ctx, badItems)
	if err == nil {
		t.Error("expected json marshal error for unmarshallable value, got nil")
	}
}

func TestRedisStore_Delete(t *testing.T) {
	ctx := context.Background()
	s, mr := setupTestRedis(t)
	defer mr.Close()

	mr.Set("del_me", "data")

	err := s.Delete(ctx, "del_me")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mr.Exists("del_me") {
		t.Error("expected key to be deleted from redis")
	}
}

func TestRedisStore_DeleteMany(t *testing.T) {
	ctx := context.Background()
	s, mr := setupTestRedis(t)
	defer mr.Close()

	err := s.DeleteMany(ctx, []string{})
	if err != nil {
		t.Fatalf("DeleteMany empty keys failed: %v", err)
	}

	mr.Set("d1", "data")
	mr.Set("d2", "data")

	err = s.DeleteMany(ctx, []string{"d1", "d2"})
	if err != nil {
		t.Fatalf("DeleteMany failed: %v", err)
	}

	if mr.Exists("d1") || mr.Exists("d2") {
		t.Error("expected keys to be deleted from redis")
	}
}

func TestRedisStore_Clear(t *testing.T) {
	s, mr := setupTestRedis(t)
	defer mr.Close()

	mr.Set("c1", "data")
	mr.Set("c2", "data")

	err := s.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if len(mr.Keys()) != 0 {
		t.Errorf("expected database to be completely flushed, remaining keys: %v", mr.Keys())
	}
}
