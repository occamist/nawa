package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/occamist/nawa/hoststats"
)

const tickerInterval = 1 * time.Second

func StreamHostStats(sampler *hoststats.Sampler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			slog.Error("streaming not supported")
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		sampler.Acquire()
		defer sampler.Release()

		ticker := time.NewTicker(tickerInterval)
		defer ticker.Stop()

		for {
			stats, err := sampler.Peek()
			switch {
			case errors.Is(err, hoststats.ErrNotReady):
				// first sample isn't in yet; wait for the next tick.
			case err != nil:
				slog.Error("collect system stats", "err", err)
				http.Error(w, "failed to collect system stats: "+err.Error(), http.StatusInternalServerError)
				return
			default:
				payload, err := json.Marshal(stats)
				if err != nil {
					slog.Error("failed to marshal system stats", "err", err)
					return
				}
				if err := writeSSEData(w, flusher, payload); err != nil {
					slog.Error("failed to write stats", "stats", string(payload), "err", err)
					return
				}
			}

			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
			}
		}
	}
}
