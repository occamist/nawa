package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// validID matches Docker container/image IDs (hex) and names.
var validID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

type containerSummary struct {
	ID      string   `json:"id"`
	Names   []string `json:"names"`
	Image   string   `json:"image"`
	ImageID string   `json:"image_id"`
	Status  string   `json:"status"`
	State   string   `json:"state"`
	Created int64    `json:"created"`
	Ports   []port   `json:"ports"`
}

type port struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
}

func ListContainers(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		containers, err := dc.ContainerList(r.Context(), container.ListOptions{All: true})
		if err != nil {
			slog.Error("list containers", "err", err)
			http.Error(w, "failed to list containers: "+err.Error(), http.StatusInternalServerError)
			return
		}

		summaries := make([]containerSummary, 0, len(containers))
		for _, c := range containers {
			ports := make([]port, 0, len(c.Ports))
			for _, p := range c.Ports {
				ports = append(ports, port{
					IP:          p.IP,
					PrivatePort: p.PrivatePort,
					PublicPort:  p.PublicPort,
					Type:        p.Type,
				})
			}
			summaries = append(summaries, containerSummary{
				ID:      c.ID,
				Names:   c.Names,
				Image:   c.Image,
				ImageID: c.ImageID,
				Status:  c.Status,
				State:   c.State,
				Created: c.Created,
				Ports:   ports,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(summaries); err != nil {
			slog.Error("encode container summaries", "err", err)
			http.Error(w, "failed to encode container summaries: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

func StartContainer(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID.MatchString(id) {
			http.Error(w, "invalid container id", http.StatusBadRequest)
			return
		}
		if err := dc.ContainerStart(r.Context(), id, container.StartOptions{}); err != nil {
			slog.Error("start container", "id", id, "err", err)
			http.Error(w, "failed to start container: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "started"}); err != nil {
			slog.Error("encode start container status", "err", err)
			http.Error(w, "failed to encode container start status: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

func StopContainer(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID.MatchString(id) {
			http.Error(w, "invalid container id", http.StatusBadRequest)
			return
		}
		timeout := 10
		if err := dc.ContainerStop(r.Context(), id, container.StopOptions{Timeout: &timeout}); err != nil {
			slog.Error("stop container", "id", id, "err", err)
			http.Error(w, "failed to stop container: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "stopped"}); err != nil {
			slog.Error("encode stop container status", "err", err)
			http.Error(w, "failed to encode container stop status: "+err.Error(), http.StatusInternalServerError)
		}
	}
}

func RemoveContainer(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID.MatchString(id) {
			http.Error(w, "invalid container id", http.StatusBadRequest)
			return
		}
		err := dc.ContainerRemove(r.Context(), id, container.RemoveOptions{Force: true})
		if err != nil {
			slog.Error("remove container", "id", id, "err", err)
			http.Error(w, "failed to remove container: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
