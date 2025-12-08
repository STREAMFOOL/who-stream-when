package cache

import (
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	cache := New(1 * time.Hour)

	cache.Set("key1", "value1")
	value, found := cache.Get("key1")

	if !found {
		t.Fatal("expected to find key1")
	}

	if value != "value1" {
		t.Errorf("expected value1, got %v", value)
	}
}

func TestCache_GetNonExistent(t *testing.T) {
	cache := New(1 * time.Hour)

	value, found := cache.Get("nonexistent")

	if found {
		t.Error("expected not to find nonexistent key")
	}

	if value != nil {
		t.Errorf("expected nil value, got %v", value)
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	cache := New(100 * time.Millisecond)

	cache.Set("key1", "value1")

	// Should be available immediately
	value, found := cache.Get("key1")
	if !found {
		t.Fatal("expected to find key1 immediately")
	}
	if value != "value1" {
		t.Errorf("expected value1, got %v", value)
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	value, found = cache.Get("key1")
	if found {
		t.Error("expected key1 to be expired")
	}
	if value != nil {
		t.Errorf("expected nil value for expired key, got %v", value)
	}
}

func TestCache_CustomTTL(t *testing.T) {
	cache := New(1 * time.Hour)

	cache.SetWithTTL("key1", "value1", 100*time.Millisecond)

	// Should be available immediately
	value, found := cache.Get("key1")
	if !found {
		t.Fatal("expected to find key1 immediately")
	}
	if value != "value1" {
		t.Errorf("expected value1, got %v", value)
	}

	// Wait for custom TTL expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	value, found = cache.Get("key1")
	if found {
		t.Error("expected key1 to be expired")
	}
}

func TestCache_Delete(t *testing.T) {
	cache := New(1 * time.Hour)

	cache.Set("key1", "value1")
	cache.Delete("key1")

	value, found := cache.Get("key1")
	if found {
		t.Error("expected key1 to be deleted")
	}
	if value != nil {
		t.Errorf("expected nil value, got %v", value)
	}
}

func TestCache_Clear(t *testing.T) {
	cache := New(1 * time.Hour)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	if cache.Size() != 3 {
		t.Errorf("expected size 3, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", cache.Size())
	}

	_, found := cache.Get("key1")
	if found {
		t.Error("expected key1 to be cleared")
	}
}

func TestCache_Cleanup(t *testing.T) {
	cache := New(100 * time.Millisecond)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.SetWithTTL("key3", "value3", 1*time.Hour)

	// Wait for some entries to expire
	time.Sleep(150 * time.Millisecond)

	// Before cleanup, expired entries still count in size
	if cache.Size() != 3 {
		t.Errorf("expected size 3 before cleanup, got %d", cache.Size())
	}

	cache.Cleanup()

	// After cleanup, only non-expired entry remains
	if cache.Size() != 1 {
		t.Errorf("expected size 1 after cleanup, got %d", cache.Size())
	}

	// key3 should still be available
	value, found := cache.Get("key3")
	if !found {
		t.Error("expected key3 to still be available")
	}
	if value != "value3" {
		t.Errorf("expected value3, got %v", value)
	}
}

func TestCache_FallbackOnAPIFailure(t *testing.T) {
	cache := New(100 * time.Millisecond)

	// Simulate storing data from successful API call
	cache.Set("streamer:123", map[string]interface{}{
		"isLive": true,
		"title":  "Test Stream",
	})

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Simulate API failure - try to get expired data
	value, found := cache.Get("streamer:123")

	// In a real scenario, the service layer would handle this
	// by checking if the value is expired and deciding whether to use it
	// This test verifies that expired data is not returned by default
	if found {
		t.Error("expected expired data not to be returned")
	}
	if value != nil {
		t.Errorf("expected nil for expired data, got %v", value)
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	cache := New(1 * time.Hour)

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Set("key", i)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Get("key")
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// If we get here without deadlock or panic, the test passes
}
