package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/occamist/nawa/dockertest"
)

const testImage = "hello-world:latest"

func TestListImages(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	dockertest.PullImage(t, dc, testImage)
	t.Cleanup(dockertest.RemoveImageFunc(t, dc, testImage))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/images", nil)
	w := httptest.NewRecorder()

	ListImages(dc)(w, req)
	if !cmp.Equal(w.Code, http.StatusOK) {
		t.Fatalf("want status = %d, got status = %d\n %s", http.StatusOK, w.Code, w.Body.String())
	}

	var images []listImagesSummary
	if err := json.NewDecoder(w.Body).Decode(&images); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for _, img := range images {
		if slices.Contains(img.RepoTags, testImage) {
			return
		}
	}
	t.Fatalf("%q not found in image list", testImage)
}

func TestPullImage(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	t.Cleanup(dockertest.RemoveImageFunc(t, dc, testImage))

	body, err := json.Marshal(pullImageDetail{Image: "hello-world"})
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/images/pull", bytes.NewReader(body))
	w := httptest.NewRecorder()

	PullImage(dc)(w, req)
	if !cmp.Equal(w.Code, http.StatusOK) {
		t.Fatalf("want status = %d, got status = %d\n %s", http.StatusOK, w.Code, w.Body.String())
	}

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	want := map[string]string{"status": "pulled", "image": testImage}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}

	if _, found := dockertest.LookupImage(t, dc, testImage); !found {
		t.Errorf("image %q not found", testImage)
	}
}

func TestPullImage_InvalidBody(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/images/pull", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()

	PullImage(nil)(w, req)
	if !cmp.Equal(http.StatusBadRequest, w.Code) {
		t.Errorf("want status = %d, got status = %d\n %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	want := "invalid request body\n"
	if !cmp.Equal(want, w.Body.String()) {
		t.Errorf("want body = %s, got body = %s", want, w.Body.String())
	}
}

func TestPullImage_EmptyImageName(t *testing.T) {
	body, err := json.Marshal(pullImageDetail{Image: " ", Tag: "latest"})
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/images/pull", bytes.NewReader(body))
	w := httptest.NewRecorder()

	PullImage(nil)(w, req)
	if !cmp.Equal(http.StatusBadRequest, w.Code) {
		t.Errorf("want status = %d, got status = %d\n %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	want := "image name required\n"
	if !cmp.Equal(want, w.Body.String()) {
		t.Errorf("want body = %s, got body = %s", want, w.Body.String())
	}
}

func TestPullImage_NonExistentImage(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	body, err := json.Marshal(pullImageDetail{Image: "thisdoesnotexist99999", Tag: "latest"})
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/images/pull", bytes.NewReader(body))
	w := httptest.NewRecorder()

	PullImage(dc)(w, req)
	if !cmp.Equal(http.StatusInternalServerError, w.Code) {
		t.Errorf("want status = %d, got status = %d\n %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
	want := "failed to pull image: Error response from daemon: pull access denied for thisdoesnotexist99999, repository does not exist or may require 'docker login': denied: requested access to the resource is denied\n"
	if !cmp.Equal(want, w.Body.String()) {
		t.Errorf("want body = %s, got body = %s", want, w.Body.String())
	}
}

func TestRemoveImage(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	dockertest.PullImage(t, dc, testImage)

	imageID, found := dockertest.LookupImage(t, dc, testImage)
	if !found {
		t.Fatalf("image %q not found after pull", testImage)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/v1/images/"+imageID, nil)
	req.SetPathValue("id", imageID)
	w := httptest.NewRecorder()

	RemoveImage(dc)(w, req)
	if !cmp.Equal(http.StatusNoContent, w.Code) {
		t.Errorf("want status = %d, got status = %d\n %s", http.StatusNoContent, w.Code, w.Body.String())
	}
	if !cmp.Equal("", w.Body.String()) {
		t.Errorf("want empty body, got body = %s", w.Body.String())
	}

	if _, found := dockertest.LookupImage(t, dc, testImage); found {
		t.Cleanup(dockertest.RemoveImageFunc(t, dc, testImage))
		t.Errorf("image %q not removed", testImage)
	}
}

func TestRemoveImage_NotFound(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/v1/images/sha256:000000000000", nil)
	req.SetPathValue("id", "sha256:000000000000")
	w := httptest.NewRecorder()

	RemoveImage(dc)(w, req)
	if !cmp.Equal(http.StatusInternalServerError, w.Code) {
		t.Errorf("want status = %d, got status = %d\n %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
	want := "failed to remove image: Error response from daemon: No such image: sha256:000000000000\n"
	if !cmp.Equal(want, w.Body.String()) {
		t.Errorf("want body = %s, got body = %s", want, w.Body.String())
	}
}

func TestPruneImages(t *testing.T) {
	dc := dockertest.NewClient(t)
	defer func() { _ = dc.Close() }()

	dockertest.PullImage(t, dc, testImage)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/images/prune", nil)
	w := httptest.NewRecorder()

	PruneImages(dc)(w, req)
	if !cmp.Equal(http.StatusOK, w.Code) {
		t.Fatalf("want status = %d, got status = %d\n %s", http.StatusOK, w.Code, w.Body.String())
	}

	var got pruneImagesInfo
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.SpaceReclaimed == 0 {
		t.Errorf("want space_reclaimed > 0, got 0")
	}

	if _, found := dockertest.LookupImage(t, dc, testImage); found {
		t.Cleanup(dockertest.RemoveImageFunc(t, dc, testImage))
		t.Errorf("image %q still present after prune", testImage)
	}
}
