package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/occamist/nawa/hoststats"
)

func TestStreamSystemStats(t *testing.T) {
	sampler := hoststats.NewSampler("/", 5*time.Millisecond)
	go sampler.Run(t.Context())

	sampler.Acquire()
	defer sampler.Release()

	// Wait for a sample so the handler has something to emit right away.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := sampler.Peek(); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sampler did not produce a snapshot in time")
		}
		time.Sleep(5 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/stats/stream", nil)
	w := newWriteRecorderOnce()

	done := make(chan struct{})
	go func() {
		handler := StreamHostStats(sampler)
		handler(w, req)
		close(done)
	}()

	select {
	case <-w.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not write any SSE events")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after request context cancellation")
	}

	if got := w.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("want Content-Type = text/event-stream, got %q", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("want Cache-Control = no-cache, got %q", got)
	}

	body := w.Body.String()
	if !strings.HasPrefix(body, "data: ") {
		t.Fatalf("want body to start with an SSE data event, got %q", body)
	}
	for _, field := range []string{`"cpu_percent"`, `"mem_used"`, `"mem_total"`, `"disk_used"`, `"disk_total"`} {
		if !strings.Contains(body, field) {
			t.Errorf("want body to contain %s, got %q", field, body)
		}
	}
}

// nonFlushingRecorder wraps httptest.ResponseRecorder without embedding it,
// so its Flush method isn't called, this makes the handler's http.Flusher fail
type nonFlushingRecorder struct {
	rec *httptest.ResponseRecorder
}

func (w *nonFlushingRecorder) Header() http.Header         { return w.rec.Header() }
func (w *nonFlushingRecorder) Write(b []byte) (int, error) { return w.rec.Write(b) } //nolint:wrapcheck // fine for testing
func (w *nonFlushingRecorder) WriteHeader(code int)        { w.rec.WriteHeader(code) }

func TestStreamSystemStats_StreamingNotSupported(t *testing.T) {
	sampler := hoststats.NewSampler("/", time.Second)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/stats/stream", nil)
	w := &nonFlushingRecorder{rec: httptest.NewRecorder()}

	handler := StreamHostStats(sampler)
	handler(w, req)

	if w.rec.Code != http.StatusInternalServerError {
		t.Errorf("want status = %d, got status = %d\n%s", http.StatusInternalServerError, w.rec.Code, w.rec.Body.String())
	}
}
