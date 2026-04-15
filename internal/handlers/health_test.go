package handlers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"testing"
)

// successDriver is a minimal database/sql driver whose Ping always succeeds.
type successDriver struct{}
type successConn struct{}

func (d successDriver) Open(_ string) (driver.Conn, error) { return successConn{}, nil }
func (c successConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, driver.ErrBadConn
}
func (c successConn) Close() error                         { return nil }
func (c successConn) Begin() (driver.Tx, error)            { return nil, driver.ErrBadConn }
func (c successConn) Ping(_ context.Context) error         { return nil }

func init() {
	sql.Register("success-driver", successDriver{})
}

// newSuccessDB returns a *sql.DB whose PingContext will always succeed.
func newSuccessDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("success-driver", "")
	if err != nil {
		t.Fatalf("sql.Open success-driver: %v", err)
	}
	return db
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	Healthz(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	body := w.Body.String()
	if body != `{"status":"ok"}` {
		t.Errorf(`expected body {"status":"ok"}, got %q`, body)
	}
}

func TestReadyz_DBReachable(t *testing.T) {
	db := newSuccessDB(t)
	defer db.Close()

	handler := ReadyzHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	body := w.Body.String()
	if body != `{"status":"ready"}` {
		t.Errorf(`expected body {"status":"ready"}, got %q`, body)
	}
}

func TestReadyz_DBUnreachable(t *testing.T) {
	db := newSuccessDB(t)
	db.Close() // Close immediately so PingContext fails

	handler := ReadyzHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	body := w.Body.String()
	if body != `{"status":"not ready"}` {
		t.Errorf(`expected body {"status":"not ready"}, got %q`, body)
	}
}
