package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const SessionKey contextKey = "session"

type SessionData struct {
	CartJSON string `json:"cart"`
	Flash    string `json:"flash"`
}

type SessionManager struct {
	Secret     []byte
	CookieName string
}

func NewSessionManager(secret string) *SessionManager {
	return &SessionManager{
		Secret:     []byte(secret),
		CookieName: "pottery_session",
	}
}

func (sm *SessionManager) sign(data string) string {
	mac := hmac.New(sha256.New, sm.Secret)
	mac.Write([]byte(data))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return sig
}

func (sm *SessionManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sd := &SessionData{}
		cookie, err := r.Cookie(sm.CookieName)
		if err == nil && cookie.Value != "" {
			parts := strings.SplitN(cookie.Value, ".", 2)
			if len(parts) == 2 {
				sig := sm.sign(parts[0])
				if hmac.Equal([]byte(sig), []byte(parts[1])) {
					decoded, err := base64.URLEncoding.DecodeString(parts[0])
					if err == nil {
						json.Unmarshal(decoded, sd)
					}
				}
			}
		}
		ctx := context.WithValue(r.Context(), SessionKey, sd)
		r = r.WithContext(ctx)

		// Wrap response writer to save session after handler
		sw := &sessionWriter{ResponseWriter: w, sm: sm, session: sd, written: false}
		next.ServeHTTP(sw, r)
		sw.saveSession()
	})
}

func (sm *SessionManager) Save(w http.ResponseWriter, sd *SessionData) {
	data, _ := json.Marshal(sd)
	encoded := base64.URLEncoding.EncodeToString(data)
	sig := sm.sign(encoded)
	value := fmt.Sprintf("%s.%s", encoded, sig)
	http.SetCookie(w, &http.Cookie{
		Name:     sm.CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})
}

func GetSession(r *http.Request) *SessionData {
	sd, ok := r.Context().Value(SessionKey).(*SessionData)
	if !ok {
		return &SessionData{}
	}
	return sd
}

type sessionWriter struct {
	http.ResponseWriter
	sm      *SessionManager
	session *SessionData
	written bool
}

func (sw *sessionWriter) saveSession() {
	if !sw.written {
		sw.written = true
		sw.sm.Save(sw.ResponseWriter, sw.session)
	}
}
