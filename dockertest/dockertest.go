package dockertest

import (
	"context"
	"io"
	"slices"
	"testing"

	"github.com/docker/docker/api/types/container"
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

// RunContainer creates and starts a container from imageRef running cmd, returning its ID.
func RunContainer(t *testing.T, dc *client.Client, imageRef string, cmd []string) string {
	t.Helper()
	cfg := &container.Config{
		Image: imageRef,
		Cmd:   cmd,
	}
	resp, err := dc.ContainerCreate(t.Context(), cfg, nil, nil, nil, "")
	if err != nil {
		t.Fatalf("failed to create container from %q: %v", imageRef, err)
	}
	if err := dc.ContainerStart(t.Context(), resp.ID, container.StartOptions{}); err != nil {
		t.Fatalf("failed to start container %q: %v", resp.ID, err)
	}
	return resp.ID
}

func RemoveContainerFunc(t *testing.T, dc *client.Client, id string) func() {
	t.Helper()
	return func() {
		ctx := context.WithoutCancel(t.Context())
		// do not use t.Context() directly here, as it may be canceled before it runs
		if err := dc.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
			t.Logf("failed to remove container %q: %v", id, err)
		}
	}
}

// WaitContainerExit blocks until the container has stopped running.
func WaitContainerExit(t *testing.T, dc *client.Client, id string) {
	t.Helper()
	statusCh, errCh := dc.ContainerWait(t.Context(), id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("failed waiting for container %q: %v", id, err)
		}
	case <-statusCh:
	}
}

func NewClient(t *testing.T) *client.Client {
	t.Helper()
	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	return dc
}
