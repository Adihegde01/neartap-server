package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

// HealthHandler returns basic server health info
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"service":   "NearTap API",
		"version":   "1.0.0",
		"goVersion": runtime.Version(),
		"time":      time.Now().UTC().Format(time.RFC3339),
	})
}

// NotFoundHandler returns a JSON 404
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   "route not found",
	})
}
