package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"pottery-shop/cmd/cardboard/internal/r2"
)

// Store abstracts the R2-backed track store for handlers and testing.
type Store interface {
	ListTracks(ctx context.Context) ([]r2.Track, error)
	Ready(ctx context.Context) error
}

// Handler serves the Cardboard API and static SPA.
type Handler struct {
	store  Store
	static http.Handler
}

// NewHandler returns a handler wired to the given store and static directory.
func NewHandler(store Store, staticDir string) *Handler {
	return &Handler{
		store:  store,
		static: http.StripPrefix("/cardboard/", http.FileServer(http.Dir(staticDir))),
	}
}

// Register adds all cardboard routes to the provided mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/readyz", h.readyz)
	mux.HandleFunc("/cardboard/api/tracks", h.listTracks)
	mux.HandleFunc("/cardboard/", h.serveStatic)
	mux.HandleFunc("/cardboard", h.redirectToCardboard)
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.store.Ready(r.Context()); err != nil {
		log.Printf("readyz check failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"not ready"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) listTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tracks, err := h.store.ListTracks(r.Context())
	if err != nil {
		log.Printf("list tracks failed: %v", err)
		http.Error(w, "failed to list tracks", http.StatusInternalServerError)
		return
	}
	if tracks == nil {
		tracks = []r2.Track{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(tracks); err != nil {
		log.Printf("encode tracks failed: %v", err)
	}
}

func (h *Handler) serveStatic(w http.ResponseWriter, r *http.Request) {
	// Ensure only GET/HEAD requests are served by the file server.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.static.ServeHTTP(w, r)
}

func (h *Handler) redirectToCardboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.Redirect(w, r, "/cardboard/", http.StatusFound)
}
