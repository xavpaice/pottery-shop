package handlers

import (
	"database/sql"
	"log"
	"net/http"
)

// Healthz is a liveness probe handler. It returns 200 {"status":"ok"} with no DB interaction.
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// ReadyzHandler returns a readiness probe handler that checks DB reachability via PingContext.
// Returns 200 {"status":"ready"} when DB is reachable, 503 {"status":"not ready"} otherwise.
// Error details are never included in the response body (information disclosure mitigation).
func ReadyzHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := db.PingContext(r.Context()); err != nil {
			log.Printf("readyz: DB ping failed: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not ready"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	}
}
