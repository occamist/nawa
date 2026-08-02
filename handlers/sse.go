package handlers

import (
	"fmt"
	"net/http"
)

// writeSSEData writes payload as a single Server-Sent Events "data" field and flushes it.
func writeSSEData(w http.ResponseWriter, f http.Flusher, payload []byte) error {
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return fmt.Errorf("failed to write SSE data: %w", err)
	}
	f.Flush()
	return nil
}
