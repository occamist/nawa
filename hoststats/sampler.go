package hoststats

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

type snapshot struct {
	stats Stats
	err   error
}

// Sampler runs one shared background sampling loop for all subscribers, so N concurrent SSE connections cost one sys read cycle, not N.
type Sampler struct {
	diskPath string
	interval time.Duration

	subscriberCount atomic.Int32
	wake            chan struct{}
	latest          atomic.Pointer[snapshot]
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
		s.latest.Store(&snapshot{stats: stats, err: err})
	}
}
