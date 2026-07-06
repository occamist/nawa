package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type listImagesSummary struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repo_tags"`
	Size     int64    `json:"size"`
	Created  int64    `json:"created"`
}

func ListImages(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		images, err := dc.ImageList(r.Context(), image.ListOptions{All: false})
		if err != nil {
			http.Error(w, "failed to list images: "+err.Error(), http.StatusInternalServerError)
			return
		}

		summaries := make([]listImagesSummary, 0, len(images))
		for _, img := range images {
			s := listImagesSummary{
				ID:       img.ID,
				RepoTags: img.RepoTags,
				Size:     img.Size,
				Created:  img.Created,
			}
			if len(s.RepoTags) == 0 && len(img.RepoDigests) > 0 { // untagged image
				imgName, _, ok := strings.Cut(img.RepoDigests[0], "@")
				if ok {
					s.RepoTags = []string{imgName + ":" + "<none>"}
				}
			}
			summaries = append(summaries, s)
		}

		if err := json.NewEncoder(w).Encode(summaries); err != nil {
			http.Error(w, "failed to encode the list images response: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

type pullImageDetail struct {
	Image string `json:"image"`
	Tag   string `json:"tag"`
}

func PullImage(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const OneMB = 1 << 20
		var req pullImageDetail
		r.Body = http.MaxBytesReader(w, r.Body, OneMB)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Image) == "" {
			http.Error(w, "image name required", http.StatusBadRequest)
			return
		}

		tag := req.Tag
		if tag == "" {
			tag = "latest"
		}
		ref := req.Image + ":" + tag

		rc, err := dc.ImagePull(r.Context(), ref, image.PullOptions{})
		if err != nil {
			http.Error(w, "failed to pull image: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rc.Close()

		dec := json.NewDecoder(rc)
		for {
			var resp struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			}

			err := dec.Decode(&resp)
			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil {
				http.Error(w, "failed to decode the pull image stream", http.StatusInternalServerError)
				return
			}

			if resp.Error != "" {
				http.Error(w, "failed to pull image: "+resp.Error, http.StatusInternalServerError)
				return
			}
		}

		if err := json.NewEncoder(w).Encode(map[string]string{"status": "pulled", "image": ref}); err != nil {
			http.Error(w, "failed to encode the pull image response: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

func RemoveImage(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := dc.ImageRemove(r.Context(), r.PathValue("id"), image.RemoveOptions{
			Force:         false,
			PruneChildren: true,
		})
		if err != nil {
			http.Error(w, "failed to remove image: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type pruneImagesInfo struct {
	SpaceReclaimed uint64 `json:"space_reclaimed"`
}

func PruneImages(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// dangling=false prunes all images not used by a container, not just untagged leftovers.
		report, err := dc.ImagesPrune(r.Context(), filters.NewArgs(filters.Arg("dangling", "false")))
		if err != nil {
			http.Error(w, "failed to prune images: "+err.Error(), http.StatusInternalServerError)
			return
		}

		info := pruneImagesInfo{SpaceReclaimed: report.SpaceReclaimed}
		if err := json.NewEncoder(w).Encode(info); err != nil {
			http.Error(w, "failed to encode the prune images response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
