package handlers

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// FIXME: Goroutine leak: stdcopy.StdCopy may block after handler returns
// FIXME: io.Copy and stdcopy.StdCopy errors must not be ignored
func StreamContainerLogs(dc *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID.MatchString(id) {
			http.Error(w, "invalid container id", http.StatusBadRequest)
			return
		}
		tail := r.URL.Query().Get("tail")
		tail = strings.TrimSpace(tail)
		switch tail {
		case "":
			tail = "100"
		case "all":
			// valid
		default:
			n, err := strconv.Atoi(tail)
			if err != nil || n < 1 {
				http.Error(w, "tail must be a positive integer or \"all\"", http.StatusBadRequest)
				return
			}
		}

		rc, err := dc.ContainerLogs(r.Context(), id, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Tail:       tail,
			Timestamps: true,
		})
		if err != nil {
			slog.Error("stream container logs", "id", id, "err", err)
			http.Error(w, "failed to stream logs: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rc.Close()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			slog.Error("streaming not supported", "id", id)
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		info, err := dc.ContainerInspect(r.Context(), id)
		if err != nil {
			slog.Error("inspect container", "id", id, "err", err)
			http.Error(w, "failed to inspect container: "+err.Error(), http.StatusInternalServerError)
			return
		}

		pipeReader, pipeWriter := io.Pipe()
		defer pipeWriter.Close() // unblocks pipeWriter.Write() in the goroutine if the handler exits early
		go func() {
			if info.Config.Tty {
				// TTY containers emit a raw byte stream with no multiplexing headers.
				io.Copy(pipeWriter, rc)
			} else {
				// Non-TTY: demultiplex Docker's binary framed stream
				// (8-byte header per frame: 1-byte stream type, 3-byte padding, 4-byte size).
				stdcopy.StdCopy(pipeWriter, pipeWriter, rc)
			}
			pipeWriter.Close()
		}()

		const TenMB = 10 * 1024 * 1024
		scanner := bufio.NewScanner(pipeReader)
		scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), TenMB)
		for scanner.Scan() {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			if err := scanner.Err(); err != nil {
				http.Error(w, "failed to scan container logs: "+err.Error(), http.StatusInternalServerError)
			}

			line := strings.ReplaceAll(scanner.Text(), "\r", "")
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}
	}
}
