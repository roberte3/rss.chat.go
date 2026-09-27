package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HealthResponse is the response for the /health endpoint
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version,omitempty"`
}

// ReadinessResponse is the response for the /ready endpoint
type ReadinessResponse struct {
	Ready     bool            `json:"ready"`
	Timestamp time.Time       `json:"timestamp"`
	Database  bool            `json:"database"`
	Checks    map[string]bool `json:"checks"`
}

// HandleHealth returns a simple 200 OK response to indicate the server is alive
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
	})
}

// HandleReady checks if the server is ready to handle traffic
func HandleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Database connectivity check
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	ready := true
	checks := make(map[string]bool)

	// Check database
	if conn, ok := r.Context().Value("db").(*sql.DB); ok {
		err := conn.PingContext(ctx)
		checks["database"] = err == nil
		if err != nil {
			ready = false
		}
	} else {
		checks["database"] = false
		ready = false
	}

	w.Header().Set("Content-Type", "application/json")
	if !ready {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	dbReady := false
	if dbCheck, ok := checks["database"]; ok {
		dbReady = dbCheck
	}

	response := ReadinessResponse{
		Ready:     ready,
		Timestamp: time.Now().UTC(),
		Database:  dbReady,
		Checks:    checks,
	}
	json.NewEncoder(w).Encode(response)
}

// HandleMetrics exposes Prometheus metrics
func HandleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	promhttp.Handler().ServeHTTP(w, r)
}
