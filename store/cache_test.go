package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDiscoveryCache_GetMiss(t *testing.T) {
	cache := newDiscoveryCache(time.Hour)
	_, ok := cache.get("missing")
	assert.False(t, ok)
}

func TestDiscoveryCache_SetAndGet(t *testing.T) {
	cache := newDiscoveryCache(time.Hour)
	data := []string{"svc-a", "svc-b"}
	cache.set("services", data)

	result, ok := cache.get("services")
	assert.True(t, ok)
	assert.Equal(t, data, result.([]string))
}

func TestDiscoveryCache_Expiry(t *testing.T) {
	cache := newDiscoveryCache(1 * time.Millisecond)
	cache.set("services", []string{"svc-a"})

	time.Sleep(5 * time.Millisecond)

	_, ok := cache.get("services")
	assert.False(t, ok, "expected cache entry to be expired")
}

func TestDiscoveryCache_DifferentKeys(t *testing.T) {
	cache := newDiscoveryCache(time.Hour)
	cache.set("services", []string{"svc-a"})
	cache.set("operations:svc-a:", []string{"op-1", "op-2"})

	svc, ok := cache.get("services")
	assert.True(t, ok)
	assert.Equal(t, []string{"svc-a"}, svc.([]string))

	ops, ok := cache.get("operations:svc-a:")
	assert.True(t, ok)
	assert.Equal(t, []string{"op-1", "op-2"}, ops.([]string))
}

func TestDiscoveryCache_Overwrite(t *testing.T) {
	cache := newDiscoveryCache(time.Hour)
	cache.set("services", []string{"old"})
	cache.set("services", []string{"new"})

	result, ok := cache.get("services")
	assert.True(t, ok)
	assert.Equal(t, []string{"new"}, result.([]string))
}
