package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/occamist/nawa/dockertest"
)

// WriteRecorderOnce wraps httptest.ResponseRecorder and closes ready the
// first time the handler writes a response body, so tests can wait for a
// streamed line instead of sleeping for a guessed duration.
type WriteRecorderOnce struct {
	*httptest.ResponseRecorder
	once  sync.Once
	ready chan struct{}
}

func newWriteRecorderOnce() *WriteRecorderOnce {
	return &WriteRecorderOnce{
		ResponseRecorder: httptest.NewRecorder(),
		ready:            make(chan struct{}),
	}
}

func (r *WriteRecorderOnce) Write(p []byte) (int, error) {
	n, err := r.ResponseRecorder.Write(p)
	r.once.Do(func() { close(r.ready) })
	return n, err //nolint:wrapcheck // fine for testing
}

func TestStreamContainerLogs_InvalidID(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/containers/bad-id/logs", nil)
	req.SetPathValue("id", "bad id!")
	w := httptest.NewRecorder()

	handler := StreamContainerLogs(nil)
	handler(w, req)
	if !cmp.Equal(w.Code, http.StatusBadRequest) {
		t.Errorf("want status = %d, got status = %d\n%s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if want := "invalid container id\n"; w.Body.String() != want {
		t.Errorf("want body = %q, got body = %q", want, w.Body.String())
	}
}

func TestStreamContainerLogs_InvalidTail(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/containers/deadbeef/logs?tail=notanumber", nil)
	req.SetPathValue("id", "deadbeef")
	w := httptest.NewRecorder()

	handler := StreamContainerLogs(nil)
	handler(w, req)
	if !cmp.Equal(w.Code, http.StatusBadRequest) {
		t.Errorf("want status = %d, got status = %d\n%s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if want := "tail must be a positive integer or \"all\"\n"; w.Body.String() != want {
		t.Errorf("want body = %q, got body = %q", want, w.Body.String())
	}
}

func TestStreamContainerLogs_ContainerNotFound(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/containers/nonexistent0000/logs", nil)
	req.SetPathValue("id", "nonexistent0000")
	w := httptest.NewRecorder()

	handler := StreamContainerLogs(dc)
	handler(w, req)
	if !cmp.Equal(w.Code, http.StatusInternalServerError) {
		t.Errorf("want status = %d, got status = %d\n%s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to stream logs:") {
		t.Errorf("want body to contain %q, got %q", "failed to stream logs:", w.Body.String())
	}
}

// TestStreamContainerLogs_ExitedContainer runs a container to completion, then
// streams its logs. Since the container has already exited, Follow mode should
// just emit the buffered log lines and reach a clean EOF (no error logged).
func TestStreamContainerLogs_ExitedContainer(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	dockertest.PullImage(t, dc, testImage)
	t.Cleanup(dockertest.RemoveImageFunc(t, dc, testImage))

	id := dockertest.RunContainer(t, dc, testImage, nil)
	t.Cleanup(dockertest.RemoveContainerFunc(t, dc, id))
	dockertest.WaitContainerExit(t, dc, id)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/containers/"+id+"/logs", nil)
	req.SetPathValue("id", id)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler := StreamContainerLogs(dc)
		handler(w, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not return for an already-exited container")
	}

	if got := w.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("want Content-Type = text/event-stream, got %q", got)
	}
	if !cmp.Equal(w.Code, http.StatusOK) {
		t.Errorf("want status = %d, got status = %d\n%s", http.StatusOK, w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Fatalf("want body to contain an SSE data event, got %q", body)
	}
	if !strings.Contains(body, "Hello from Docker!") {
		t.Errorf("want body to contain container output, got %q", body)
	}
}

func TestStreamContainerLogs_StreamingNotSupported(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	dockertest.PullImage(t, dc, testImage)
	t.Cleanup(dockertest.RemoveImageFunc(t, dc, testImage))

	id := dockertest.RunContainer(t, dc, testImage, nil)
	t.Cleanup(dockertest.RemoveContainerFunc(t, dc, id))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/containers/"+id+"/logs", nil)
	req.SetPathValue("id", id)
	w := &nonFlushingRecorder{rec: httptest.NewRecorder()}

	handler := StreamContainerLogs(dc)
	handler(w, req)

	if w.rec.Code != http.StatusInternalServerError {
		t.Errorf("want status = %d, got status = %d\n%s", http.StatusInternalServerError, w.rec.Code, w.rec.Body.String())
	}
}

func TestStreamContainerLogs_ClientDisconnect(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	const busybox = "busybox:latest"
	dockertest.PullImage(t, dc, busybox)
	t.Cleanup(dockertest.RemoveImageFunc(t, dc, busybox))

	id := dockertest.RunContainer(t, dc, busybox, []string{"sh", "-c", "while true; do echo tick; sleep 0.05; done"})
	t.Cleanup(dockertest.RemoveContainerFunc(t, dc, id))

	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/containers/"+id+"/logs", nil)
	req.SetPathValue("id", id)
	w := newWriteRecorderOnce()

	done := make(chan struct{})
	go func() {
		handler := StreamContainerLogs(dc)
		handler(w, req)
		close(done)
	}()

	select {
	case <-w.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not stream any log lines")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return promptly after request context cancellation")
	}

	if !strings.Contains(w.Body.String(), "data: ") {
		t.Errorf("want body to contain at least one SSE data event, got %q", w.Body.String())
	}
}
