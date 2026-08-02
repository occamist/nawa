package hoststats

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitForSnapshot(t *testing.T, s *Sampler, timeout time.Duration) Stats {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if stats, err := s.Peek(); err == nil {
			return stats
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("sampler did not produce a snapshot within %v", timeout)
	return Stats{}
}

func TestSampler_SnapshotNotReadyBeforeSampling(t *testing.T) {
	s := NewSampler("/", 10*time.Millisecond)
	if _, err := s.Peek(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("want ErrNotReady before any sampling, got %v", err)
	}
}

func TestSampler_IdlesWithoutSubscribers(t *testing.T) {
	s := NewSampler("/", 10*time.Millisecond)
	go s.Run(t.Context())

	// Never Acquire. If Run ignored the subscriber count it would have
	// produced a sample well within this window at a 10ms interval.
	time.Sleep(150 * time.Millisecond)

	if _, err := s.Peek(); !errors.Is(err, ErrNotReady) {
		t.Errorf("want sampler to stay idle without subscribers, got a snapshot (err = %v)", err)
	}
}

func TestSampler_SamplesAfterAcquire(t *testing.T) {
	s := NewSampler("/", 10*time.Millisecond)
	go s.Run(t.Context())

	s.Acquire()
	defer s.Release()

	stats := waitForSnapshot(t, s, 2*time.Second)
	if stats.MemTotal == 0 {
		t.Error("want MemTotal > 0 in sampled stats, got 0")
	}
}

func TestSampler_SharedSnapshotAcrossSubscribers(t *testing.T) {
	s := NewSampler("/", 10*time.Millisecond)
	go s.Run(t.Context())

	s.Acquire()
	defer s.Release()
	s.Acquire() // simulate a second concurrent SSE connection
	defer s.Release()

	waitForSnapshot(t, s, 2*time.Second)

	stats1, err1 := s.Peek()
	stats2, err2 := s.Peek()
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if stats1 != stats2 {
		t.Errorf("want both subscribers to read the identical cached snapshot, got %+v vs %+v", stats1, stats2)
	}
}

func TestSampler_RunExitsOnContextCancelWhileActive(t *testing.T) {
	s := NewSampler("/", 5*time.Millisecond)
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	s.Acquire()
	waitForSnapshot(t, s, 2*time.Second)

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation while actively sampling")
	}
}

func TestSampler_RunExitsOnContextCancelWhileIdle(t *testing.T) {
	s := NewSampler("/", 5*time.Millisecond)
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond) // let Run settle into the idle branch (no Acquire)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit while idle after context cancellation")
	}
}
