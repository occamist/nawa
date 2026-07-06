package dockertest

import (
	"context"
	"io"
	"slices"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

func RemoveImageFunc(t *testing.T, dc *client.Client, ref string) func() {
	t.Helper()
	return func() {
		ctx := context.WithoutCancel(t.Context())
		// do not use t.Context() directly here, as it may be canceled before it runs
		_, err := dc.ImageRemove(ctx, ref, image.RemoveOptions{Force: true})
		if err != nil {
			t.Logf("failed to remove image %q: %v", ref, err)
		}
	}
}

func PullImage(t *testing.T, dc *client.Client, ref string) {
	t.Helper()
	rc, err := dc.ImagePull(t.Context(), ref, image.PullOptions{})
	if err != nil {
		t.Fatalf("failed to pull %q: %v", ref, err)
	}
	if _, err := io.Copy(io.Discard, rc); err != nil {
		t.Fatalf("failed to copy pull stream for %q: %v", ref, err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("failed to close pull stream for %q: %v", ref, err)
	}
}

func LookupImage(t *testing.T, dc *client.Client, ref string) (string, bool) {
	t.Helper()
	images, err := dc.ImageList(t.Context(), image.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list images: %v", err)
	}
	for _, img := range images {
		if slices.Contains(img.RepoTags, ref) {
			return img.ID, true
		}
	}
	return "", false
}

func NewClient(t *testing.T) *client.Client {
	t.Helper()
	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	return dc
}
