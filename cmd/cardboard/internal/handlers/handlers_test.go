package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pottery-shop/cmd/cardboard/internal/r2"
)

type fakeStore struct {
	tracks []r2.Track
	ready  bool
}

func (f *fakeStore) ListTracks(ctx context.Context) ([]r2.Track, error) {
	return f.tracks, nil
}

func (f *fakeStore) Ready(ctx context.Context) error {
	if !f.ready {
		return errors.New("not ready")
	}
	return nil
}

func TestListTracksHandler(t *testing.T) {
	store := &fakeStore{
		tracks: []r2.Track{
			{Key: "a.mp3", Title: "A", Artist: "AA", Album: "AAA"},
			{Key: "b.mp3", Title: "B", Artist: "BB", Album: "BBB"},
		},
	}
	h := NewHandler(store, "")
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/cardboard/api/tracks", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got []r2.Track
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(tracks) = %d, want 2", len(got))
	}
	if got[0].Title != "A" {
		t.Errorf("first track title = %q, want A", got[0].Title)
	}
}

func TestHealthHandler(t *testing.T) {
	h := NewHandler(&fakeStore{}, "")
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %q, want status ok", rr.Body.String())
	}
}

func TestReadyHandler(t *testing.T) {
	mux := http.NewServeMux()

	// Not ready
	hNotReady := NewHandler(&fakeStore{ready: false}, "")
	hNotReady.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("not ready status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}

	// Ready
	mux2 := http.NewServeMux()
	hReady := NewHandler(&fakeStore{ready: true}, "")
	hReady.Register(mux2)
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr = httptest.NewRecorder()
	mux2.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("ready status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestStaticAndRedirect(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>SPA</h1>"), 0644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	h := NewHandler(&fakeStore{ready: true}, dir)
	mux := http.NewServeMux()
	h.Register(mux)

	// Static path serves index.html
	req := httptest.NewRequest(http.MethodGet, "/cardboard/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("static status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "SPA") {
		t.Errorf("body = %q, want SPA", rr.Body.String())
	}

	// Bare /cardboard redirects to /cardboard/
	req = httptest.NewRequest(http.MethodGet, "/cardboard", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("redirect status = %d, want %d", rr.Code, http.StatusFound)
	}
	if loc := rr.Header().Get("Location"); loc != "/cardboard/" {
		t.Errorf("Location = %q, want /cardboard/", loc)
	}
}
