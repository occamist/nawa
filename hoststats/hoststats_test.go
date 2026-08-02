package hoststats

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCollect(t *testing.T) {
	stats, err := Collect(t.Context(), "/", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if stats.MemTotal == 0 {
		t.Error("want MemTotal > 0, got 0")
	}
	if stats.MemUsed == 0 || stats.MemUsed > stats.MemTotal {
		t.Errorf("want 0 < MemUsed <= MemTotal, got MemUsed = %d, MemTotal = %d", stats.MemUsed, stats.MemTotal)
	}
	if stats.DiskTotal == 0 {
		t.Error("want DiskTotal > 0, got 0")
	}
	if stats.DiskUsed == 0 || stats.DiskUsed > stats.DiskTotal {
		t.Errorf("want 0 < DiskUsed <= DiskTotal, got DiskUsed = %d, DiskTotal = %d", stats.DiskUsed, stats.DiskTotal)
	}
	if stats.CPUPercent < 0 || stats.CPUPercent > 100 {
		t.Errorf("want 0 <= CPUPercent <= 100, got %f", stats.CPUPercent)
	}
}

func TestCollect_InvalidDiskPath(t *testing.T) {
	_, err := Collect(t.Context(), "/this/path/does/not/exist/nawa-test", 10*time.Millisecond)
	if err == nil {
		t.Fatal("want error for nonexistent disk path, got nil")
	}
	if !strings.Contains(err.Error(), "disk usage") {
		t.Errorf("want error to mention disk usage, got %q", err.Error())
	}
}

func TestCollect_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	start := time.Now()
	_, err := Collect(ctx, "/", 5*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want error for an already-cancelled context, got nil")
	}
	if elapsed > time.Second {
		t.Errorf("Collect took %v to return after context cancellation, want well under the 5s interval", elapsed)
	}
}
