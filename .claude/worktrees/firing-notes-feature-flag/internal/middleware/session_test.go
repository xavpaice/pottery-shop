package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewSessionManager(t *testing.T) {
	sm := NewSessionManager("test-secret")
	if sm.CookieName != "pottery_session" {
		t.Errorf("expected cookie name 'pottery_session', got %q", sm.CookieName)
	}
	if string(sm.Secret) != "test-secret" {
		t.Errorf("expected secret 'test-secret', got %q", string(sm.Secret))
	}
}

func TestSessionManager_SignConsistency(t *testing.T) {
	sm := NewSessionManager("test-secret")
	sig1 := sm.sign("hello")
	sig2 := sm.sign("hello")
	if sig1 != sig2 {
		t.Error("signing the same data should produce the same signature")
	}
}

func TestSessionManager_SignDifferentData(t *testing.T) {
	sm := NewSessionManager("test-secret")
	sig1 := sm.sign("hello")
	sig2 := sm.sign("world")
	if sig1 == sig2 {
		t.Error("different data should produce different signatures")
	}
}

func TestSessionManager_SignDifferentSecrets(t *testing.T) {
	sm1 := NewSessionManager("secret-1")
	sm2 := NewSessionManager("secret-2")
	sig1 := sm1.sign("hello")
	sig2 := sm2.sign("hello")
	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestSessionManager_TamperDetection(t *testing.T) {
	sm := NewSessionManager("test-secret")

	// Create a valid session cookie
	sd := &SessionData{CartJSON: `[{"product_id":1}]`, Flash: "hello"}
	data, _ := json.Marshal(sd)
	encoded := base64.URLEncoding.EncodeToString(data)
	sig := sm.sign(encoded)
	validCookie := encoded + "." + sig

	// Tamper with the data portion
	tampered := &SessionData{CartJSON: `[{"product_id":999}]`, Flash: "hacked"}
	tamperedData, _ := json.Marshal(tampered)
	tamperedEncoded := base64.URLEncoding.EncodeToString(tamperedData)
	tamperedCookie := tamperedEncoded + "." + sig // same sig, different data

	// Valid cookie should load session data
	var loadedSD *SessionData
	handler := sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loadedSD = GetSession(r)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "pottery_session", Value: validCookie})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if loadedSD.CartJSON != `[{"product_id":1}]` {
		t.Errorf("valid cookie: expected cart data, got %q", loadedSD.CartJSON)
	}

	// Tampered cookie should result in empty session
	loadedSD = nil
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "pottery_session", Value: tamperedCookie})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if loadedSD.CartJSON != "" {
		t.Errorf("tampered cookie: expected empty cart, got %q", loadedSD.CartJSON)
	}
}

func TestSessionManager_SaveLoadRoundTrip(t *testing.T) {
	sm := NewSessionManager("round-trip-secret")

	// First request: set session data
	var setCookieValue string
	handler := sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(r)
		session.CartJSON = `[{"product_id":42,"title":"Mug","price":25.00}]`
		session.Flash = "Item added"
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Extract the Set-Cookie
	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "pottery_session" {
			setCookieValue = c.Value
			break
		}
	}
	if setCookieValue == "" {
		t.Fatal("expected pottery_session cookie to be set")
	}

	// Second request: read session data back
	var loadedSD *SessionData
	handler2 := sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loadedSD = GetSession(r)
	}))

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&http.Cookie{Name: "pottery_session", Value: setCookieValue})
	rec2 := httptest.NewRecorder()
	handler2.ServeHTTP(rec2, req2)

	if loadedSD == nil {
		t.Fatal("session data not loaded")
	}
	if loadedSD.CartJSON != `[{"product_id":42,"title":"Mug","price":25.00}]` {
		t.Errorf("expected cart JSON round-trip, got %q", loadedSD.CartJSON)
	}
	if loadedSD.Flash != "Item added" {
		t.Errorf("expected flash 'Item added', got %q", loadedSD.Flash)
	}
}

func TestSessionManager_NoCookie(t *testing.T) {
	sm := NewSessionManager("test-secret")

	var loadedSD *SessionData
	handler := sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loadedSD = GetSession(r)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if loadedSD == nil {
		t.Fatal("expected non-nil session data")
	}
	if loadedSD.CartJSON != "" {
		t.Errorf("expected empty cart, got %q", loadedSD.CartJSON)
	}
}

func TestSessionManager_InvalidCookieFormat(t *testing.T) {
	sm := NewSessionManager("test-secret")

	var loadedSD *SessionData
	handler := sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loadedSD = GetSession(r)
	}))

	// No dot separator
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "pottery_session", Value: "garbage-no-dot"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if loadedSD.CartJSON != "" {
		t.Errorf("expected empty session for invalid cookie, got %q", loadedSD.CartJSON)
	}
}

func TestGetSession_NoContext(t *testing.T) {
	// Calling GetSession on a request with no session in context
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	sd := GetSession(req)
	if sd == nil {
		t.Fatal("expected non-nil session data")
	}
	if sd.CartJSON != "" || sd.Flash != "" {
		t.Error("expected empty session data")
	}
}

func TestSessionManager_CookieProperties(t *testing.T) {
	sm := NewSessionManager("test-secret")

	handler := sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(r)
		session.Flash = "test"
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == "pottery_session" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected pottery_session cookie")
	}
	if !found.HttpOnly {
		t.Error("expected HttpOnly flag")
	}
	if found.Path != "/" {
		t.Errorf("expected path '/', got %q", found.Path)
	}

	// Verify the cookie value has the expected format: base64.signature
	parts := strings.SplitN(found.Value, ".", 2)
	if len(parts) != 2 {
		t.Errorf("expected cookie value to have format 'data.signature', got %q", found.Value)
	}
}
