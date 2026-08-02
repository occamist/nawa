package hoststats

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

type snapshot struct {
	stats Stats
	err   error
}

// netCounter is the last cumulative byte counters Run observed, used to turn them into a bytes/sec rate between successive samples.
type netCounter struct {
	sent, recv uint64
}

// Sampler runs one shared background sampling loop for all subscribers, so N concurrent SSE connections cost one sys read cycle, not N.
type Sampler struct {
	diskPath string
	interval time.Duration

	subscriberCount atomic.Int32
	wake            chan struct{}
	latest          atomic.Pointer[snapshot]

	// prevNet is only read/written from within Run's single goroutine, so it needs no synchronization.
	prevNet *netCounter
}

func NewSampler(diskPath string, interval time.Duration) *Sampler {
	return &Sampler{diskPath: diskPath, interval: interval, wake: make(chan struct{}, 1)}
}

// Acquire/Release only track subscriber count; they own no goroutine.
func (s *Sampler) Acquire() {
	if s.subscriberCount.Add(1) == 1 {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

func (s *Sampler) Release() {
	s.subscriberCount.Add(-1)
}

var ErrNotReady = errors.New("stats not sampled yet")

func (s *Sampler) Peek() (Stats, error) {
	snap := s.latest.Load()
	if snap == nil {
		return Stats{}, ErrNotReady
	}
	return snap.stats, snap.err
}

// Run drives sampling for the lifetime of ctx. Call it once, e.g. `go sampler.Run(ctx)`.
// It idles when there are no subscribers.
func (s *Sampler) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if s.subscriberCount.Load() == 0 {
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			}
			continue
		}

		stats, err := Collect(ctx, s.diskPath, s.interval) // blocks ~interval
		if ctx.Err() != nil {
			return
		}

		if err == nil {
			stats.NetSentRate, stats.NetRecvRate, err = s.netStats(ctx)
		}
		s.latest.Store(&snapshot{stats: stats, err: err})
	}
}

// netStats reports bytes/sec sent and received since the previous call, based on cumulative counters.
// Elapsed time is approximated as s.interval, since Collect already blocks for ~interval per call.
func (s *Sampler) netStats(ctx context.Context) (sentRate, recvRate float64, err error) {
	counters, err := net.IOCountersWithContext(ctx, false)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to compute network io counters: %w", err)
	}
	if len(counters) == 0 {
		return 0, 0, nil
	}

	rateSince := func(cur, prev uint64, elapsedSeconds float64) float64 {
		if cur < prev {
			return 0
		}
		return float64(cur-prev) / elapsedSeconds
	}

	current := &netCounter{sent: counters[0].BytesSent, recv: counters[0].BytesRecv}
	if prev := s.prevNet; prev != nil {
		sentRate = rateSince(current.sent, prev.sent, s.interval.Seconds())
		recvRate = rateSince(current.recv, prev.recv, s.interval.Seconds())
	}
	s.prevNet = current
	return sentRate, recvRate, nil
}
