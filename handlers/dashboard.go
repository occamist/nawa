package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

type containerCounts struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Stopped int `json:"stopped"`
}

type resourceCounts struct {
	Total  int `json:"total"`
	InUse  int `json:"in_use"`
	Unused int `json:"unused"`
}

type dashboard struct {
	Containers containerCounts `json:"containers"`
	Images     resourceCounts  `json:"images"`
	Volumes    resourceCounts  `json:"volumes"`
	Networks   resourceCounts  `json:"networks"`
}

func stripSha256(id string) string {
	return strings.TrimPrefix(id, "sha256:")
}

func Dashboard(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		containers, err := dc.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			slog.Error("list containers", "err", err)
			http.Error(w, "failed to list containers: "+err.Error(), http.StatusInternalServerError)
			return
		}
		images, err := dc.ImageList(ctx, image.ListOptions{All: false})
		if err != nil {
			slog.Error("list images", "err", err)
			http.Error(w, "failed to list images: "+err.Error(), http.StatusInternalServerError)
			return
		}
		volumes, err := dc.VolumeList(ctx, volume.ListOptions{})
		if err != nil {
			slog.Error("list volumes", "err", err)
			http.Error(w, "failed to list volumes: "+err.Error(), http.StatusInternalServerError)
			return
		}
		networks, err := dc.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			slog.Error("list networks", "err", err)
			http.Error(w, "failed to list networks: "+err.Error(), http.StatusInternalServerError)
			return
		}

		running := 0
		usedImageIDs := make(map[string]struct{})
		usedVolumeNames := make(map[string]struct{})
		usedNetworkNames := make(map[string]struct{})
		for _, c := range containers {
			if c.State == "running" {
				running++
			}
			usedImageIDs[stripSha256(c.ImageID)] = struct{}{}
			for _, m := range c.Mounts {
				if m.Type == mount.TypeVolume {
					usedVolumeNames[m.Name] = struct{}{}
				}
			}
			if c.NetworkSettings != nil {
				for name := range c.NetworkSettings.Networks {
					usedNetworkNames[name] = struct{}{}
				}
			}
		}

		usedImages := 0
		for _, img := range images {
			if _, ok := usedImageIDs[stripSha256(img.ID)]; ok {
				usedImages++
			}
		}

		usedVolumes := 0
		for _, v := range volumes.Volumes {
			if _, ok := usedVolumeNames[v.Name]; ok {
				usedVolumes++
			}
		}

		usedNetworks := 0
		for _, n := range networks {
			if _, ok := usedNetworkNames[n.Name]; ok {
				usedNetworks++
			}
		}

		stats := dashboard{
			Containers: containerCounts{
				Total:   len(containers),
				Running: running,
				Stopped: len(containers) - running,
			},
			Images: resourceCounts{
				Total:  len(images),
				InUse:  usedImages,
				Unused: len(images) - usedImages,
			},
			Volumes: resourceCounts{
				Total:  len(volumes.Volumes),
				InUse:  usedVolumes,
				Unused: len(volumes.Volumes) - usedVolumes,
			},
			Networks: resourceCounts{
				Total:  len(networks),
				InUse:  usedNetworks,
				Unused: len(networks) - usedNetworks,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			slog.Error("encode resource stats", "err", err)
			http.Error(w, "failed to encode resource stats: "+err.Error(), http.StatusInternalServerError)
		}
	}
}
