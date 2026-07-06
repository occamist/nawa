package ratelimiter

import (
	"testing"
	"time"
)

func TestLimiterBlocksAfterMaxFailures(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	limiter := New(Config{
		MaxFailures: 2,
		Window:      time.Minute,
		BlockFor:    time.Minute,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	if !limiter.Allow("ip:127.0.0.1") {
		t.Fatal("new key should be allowed")
	}

	limiter.RecordFailure("ip:127.0.0.1")
	if !limiter.Allow("ip:127.0.0.1") {
		t.Fatal("key should be allowed before the failure threshold")
	}

	limiter.RecordFailure("ip:127.0.0.1")
	if limiter.Allow("ip:127.0.0.1") {
		t.Fatal("key should be blocked at the failure threshold")
	}
}

func TestLimiterUnblocksAfterBlockDuration(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	limiter := New(Config{
		MaxFailures: 1,
		Window:      time.Minute,
		BlockFor:    time.Minute,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:admin")
	if limiter.Allow("user:admin") {
		t.Fatal("key should be blocked")
	}

	now = now.Add(time.Minute + time.Nanosecond)
	if !limiter.Allow("user:admin") {
		t.Fatal("key should be allowed after the block duration")
	}
}

func TestLimiterAllowsExactlyAtBlockBoundary(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	blockFor := time.Minute
	limiter := New(Config{
		MaxFailures: 1,
		Window:      time.Hour,
		BlockFor:    blockFor,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:admin")
	now = now.Add(blockFor)

	if !limiter.Allow("user:admin") {
		t.Fatal("key should be allowed exactly at the block boundary")
	}
}

func TestLimiterFailureCounterResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	limiter := New(Config{
		MaxFailures: 2,
		Window:      time.Minute,
		BlockFor:    time.Minute,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:admin")
	now = now.Add(time.Minute + time.Nanosecond)
	limiter.RecordFailure("user:admin")

	if !limiter.Allow("user:admin") {
		t.Fatal("key should remain allowed because the previous failure window expired")
	}
}

func TestLimiterFailureCounterResetsExactlyAtWindowBoundary(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	window := time.Minute
	limiter := New(Config{
		MaxFailures: 2,
		Window:      window,
		BlockFor:    time.Minute,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:admin")
	now = now.Add(window)
	limiter.RecordFailure("user:admin")

	if !limiter.Allow("user:admin") {
		t.Fatal("key should remain allowed because the previous failure expired at the boundary")
	}
}

func TestLimiterFailureWindowSlides(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	window := 15 * time.Minute
	limiter := New(Config{
		MaxFailures: 3,
		Window:      window,
		BlockFor:    time.Minute,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:admin")
	now = now.Add(14 * time.Minute)
	limiter.RecordFailure("user:admin")
	now = now.Add(14 * time.Minute)

	if !limiter.Allow("user:admin") {
		t.Fatal("key should remain tracked inside the extended sliding window")
	}

	limiter.RecordFailure("user:admin")
	if limiter.Allow("user:admin") {
		t.Fatal("key should block when failures accumulate inside the sliding window")
	}
}

func TestLimiterReblocksAfterBlockExpires(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	blockFor := time.Minute
	limiter := New(Config{
		MaxFailures: 2,
		Window:      time.Hour,
		BlockFor:    blockFor,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:admin")
	limiter.RecordFailure("user:admin")
	if limiter.Allow("user:admin") {
		t.Fatal("key should be blocked")
	}

	now = now.Add(blockFor)
	if !limiter.Allow("user:admin") {
		t.Fatal("key should be allowed exactly when the block expires")
	}

	limiter.RecordFailure("user:admin")
	if limiter.Allow("user:admin") {
		t.Fatal("key should re-block immediately because failures remain at the threshold")
	}
}

func TestLimiterRecordFailureExtendsExistingBlock(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	blockFor := time.Minute
	limiter := New(Config{
		MaxFailures: 1,
		Window:      time.Hour,
		BlockFor:    blockFor,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:admin")
	now = now.Add(30 * time.Second)
	limiter.RecordFailure("user:admin")
	now = now.Add(30 * time.Second)

	if limiter.Allow("user:admin") {
		t.Fatal("key should remain blocked because the second failure extended the block")
	}

	now = now.Add(30 * time.Second)
	if !limiter.Allow("user:admin") {
		t.Fatal("key should be allowed after the extended block expires")
	}
}

func TestLimiterResetClearsFailures(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	limiter := New(Config{
		MaxFailures: 1,
		Window:      time.Minute,
		BlockFor:    time.Minute,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:admin")
	limiter.Reset("user:admin")

	if !limiter.Allow("user:admin") {
		t.Fatal("reset key should be allowed")
	}
}

func TestLimiterResetUnknownKey(t *testing.T) {
	limiter := New(Config{
		MaxFailures: 1,
		Window:      time.Minute,
		BlockFor:    time.Minute,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})

	limiter.Reset("user:missing")

	if len(limiter.entries) != 0 {
		t.Fatal("resetting an unknown key should not create an entry")
	}
}

func TestLimiterAllowRemovesExpiredEntry(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	window := time.Minute
	limiter := New(Config{
		MaxFailures: 2,
		Window:      window,
		BlockFor:    time.Minute,
		MaxKeys:     10,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:admin")
	if len(limiter.entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(limiter.entries))
	}

	now = now.Add(window)
	if !limiter.Allow("user:admin") {
		t.Fatal("expired key should be allowed")
	}
	if len(limiter.entries) != 0 {
		t.Fatal("allowing an expired key should remove it from the map")
	}
}

func TestLimiterEvictsOldestAtCapacity(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	limiter := New(Config{
		MaxFailures: 2,
		Window:      time.Hour,
		BlockFor:    time.Hour,
		MaxKeys:     2,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:first")
	now = now.Add(time.Second)
	limiter.RecordFailure("user:second")
	now = now.Add(time.Second)
	limiter.RecordFailure("user:third")

	if len(limiter.entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(limiter.entries))
	}
	if _, ok := limiter.entries["user:first"]; ok {
		t.Fatal("oldest key should be evicted")
	}
}

func TestLimiterDoesNotEvictWhenUpdatingExistingKeyAtCapacity(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	limiter := New(Config{
		MaxFailures: 3,
		Window:      time.Hour,
		BlockFor:    time.Hour,
		MaxKeys:     2,
		MaxKeyLen:   64,
	})
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("user:first")
	now = now.Add(time.Second)
	limiter.RecordFailure("user:second")
	now = now.Add(time.Second)
	limiter.RecordFailure("user:first")

	if len(limiter.entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(limiter.entries))
	}
	if _, ok := limiter.entries["user:second"]; !ok {
		t.Fatal("updating an existing key should not evict another key")
	}
}

func TestLimiterRejectsOverlongKeys(t *testing.T) {
	limiter := New(Config{
		MaxFailures: 1,
		Window:      time.Minute,
		BlockFor:    time.Minute,
		MaxKeys:     10,
		MaxKeyLen:   4,
	})

	if limiter.Allow("too-long") {
		t.Fatal("overlong key should not be allowed")
	}
	if limiter.Allow("") {
		t.Fatal("empty key should not be allowed")
	}

	limiter.RecordFailure("too-long")
	limiter.RecordFailure("")
	if len(limiter.entries) != 0 {
		t.Fatal("invalid keys should not be stored")
	}
}
