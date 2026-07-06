package handlers

import (
	"encoding/json"
	"net/http"
)

func Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			http.Error(w, "Currently server is unhealthy", http.StatusInternalServerError)
		}
	}
}
