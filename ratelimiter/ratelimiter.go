package ratelimiter

import (
	"strings"
	"sync"
	"time"
)

type Config struct {
	MaxFailures int
	// Window is a sliding failure window. Each failed attempt extends the
	// counter's expiry; once the window passes without another failure, the
	// next failure starts a fresh counter.
	Window    time.Duration
	BlockFor  time.Duration
	MaxKeys   int
	MaxKeyLen int
}

type Limiter struct {
	mu      sync.Mutex
	now     func() time.Time
	cfg     Config
	entries map[string]entry
}

type entry struct {
	failures     int
	blockedUntil time.Time
	expiresAt    time.Time
	lastSeen     time.Time
}

const (
	defaultMaxFailures = 5
	defaultWindow      = 15 * time.Minute
	defaultBlockFor    = 15 * time.Minute
	defaultMaxKeys     = 10000
	defaultMaxKeyLen   = 256
)

func New(c Config) *Limiter {
	if c.MaxFailures <= 0 {
		c.MaxFailures = defaultMaxFailures
	}
	if c.Window <= 0 {
		c.Window = defaultWindow
	}
	if c.BlockFor <= 0 {
		c.BlockFor = defaultBlockFor
	}
	if c.MaxKeys <= 0 {
		c.MaxKeys = defaultMaxKeys
	}
	if c.MaxKeyLen <= 0 {
		c.MaxKeyLen = defaultMaxKeyLen
	}

	return &Limiter{
		now:     time.Now,
		cfg:     c,
		entries: make(map[string]entry),
	}
}

func (l *Limiter) Allow(key string) bool {
	if strings.TrimSpace(key) == "" || len(key) > l.cfg.MaxKeyLen {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	e, ok := l.entries[key]
	if !ok {
		return true
	}
	if !e.expiresAt.After(now) {
		delete(l.entries, key)
		return true
	}
	if e.blockedUntil.After(now) {
		return false
	}
	return true
}

func (l *Limiter) RecordFailure(key string) {
	if strings.TrimSpace(key) == "" || len(key) > l.cfg.MaxKeyLen {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.removeExpired(now)

	e, exists := l.entries[key]
	if !exists {
		l.ensureCapacity(now)
	}
	if !e.expiresAt.After(now) {
		e.failures = 0
	}
	e.failures++
	e.lastSeen = now
	e.expiresAt = now.Add(l.cfg.Window)
	if e.failures >= l.cfg.MaxFailures {
		e.blockedUntil = now.Add(l.cfg.BlockFor)
		if e.blockedUntil.After(e.expiresAt) {
			e.expiresAt = e.blockedUntil
		}
	}
	l.entries[key] = e
}

func (l *Limiter) Reset(key string) {
	if strings.TrimSpace(key) == "" || len(key) > l.cfg.MaxKeyLen {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.entries, key)
}

func (l *Limiter) removeExpired(now time.Time) {
	for key, e := range l.entries {
		if !e.expiresAt.After(now) {
			delete(l.entries, key)
		}
	}
}

func (l *Limiter) ensureCapacity(now time.Time) {
	if len(l.entries) < l.cfg.MaxKeys {
		return
	}

	l.removeExpired(now)
	if len(l.entries) < l.cfg.MaxKeys {
		return
	}

	var oldestKey string
	var oldest time.Time
	for key, e := range l.entries {
		if oldestKey == "" || e.lastSeen.Before(oldest) {
			oldestKey = key
			oldest = e.lastSeen
		}
	}
	delete(l.entries, oldestKey)
}
