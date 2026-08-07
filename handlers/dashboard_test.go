package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/occamist/nawa/dockertest"
)

func TestDashboard(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	dockertest.PullImage(t, dc, testImage)
	t.Cleanup(dockertest.RemoveImageFunc(t, dc, testImage))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/dashboard", nil)
	w := httptest.NewRecorder()

	handler := Dashboard(dc)
	handler(w, req)
	if !cmp.Equal(w.Code, http.StatusOK) {
		t.Fatalf("want status = %d, got status = %d\n %s", http.StatusOK, w.Code, w.Body.String())
	}

	var d dashboard
	if err := json.NewDecoder(w.Body).Decode(&d); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if d.Containers.Total != d.Containers.Running+d.Containers.Stopped {
		t.Errorf("containers total mismatch: total=%d running=%d stopped=%d",
			d.Containers.Total, d.Containers.Running, d.Containers.Stopped)
	}
	if d.Images.Total != d.Images.InUse+d.Images.Unused {
		t.Errorf("images total mismatch: total=%d in_use=%d unused=%d",
			d.Images.Total, d.Images.InUse, d.Images.Unused)
	}
	if d.Volumes.Total != d.Volumes.InUse+d.Volumes.Unused {
		t.Errorf("volumes total mismatch: total=%d in_use=%d unused=%d",
			d.Volumes.Total, d.Volumes.InUse, d.Volumes.Unused)
	}
	if d.Networks.Total != d.Networks.InUse+d.Networks.Unused {
		t.Errorf("networks total mismatch: total=%d in_use=%d unused=%d",
			d.Networks.Total, d.Networks.InUse, d.Networks.Unused)
	}
}
