package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookieName = "r3_session"
	sessionTTL        = 12 * time.Hour

	csrfCookieName = "r3_csrf"
	csrfHeaderName = "X-CSRF-Token"
	csrfFormName   = "csrf_token"
)

type ctxKey string

const ctxCsrfToken ctxKey = "csrf_token"

// isSafeRedirect returns true for same-origin relative paths only.
// It rejects absolute URLs, protocol-relative URLs, and empty values.
func isSafeRedirect(next string) bool {
	if next == "" {
		return false
	}
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return false
	}
	// Reject attempts to smuggle a host into a path (e.g. "/@evil.com").
	if idx := strings.Index(next, "://"); idx != -1 {
		return false
	}
	return true
}

// makeSession issues a signed cookie value for the given user.
func (s *Server) makeSession(u *sessionUser) string {
	payload, _ := json.Marshal(u)
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionKey))
	mac.Write([]byte(b64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return b64 + "." + sig
}

// parseSession verifies the cookie value and returns the user, or nil.
func (s *Server) parseSession(v string) *sessionUser {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return nil
	}
	b64, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionKey))
	mac.Write([]byte(b64))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return nil
	}
	var u sessionUser
	if json.Unmarshal(raw, &u) != nil {
		return nil
	}
	if u.ID == "" {
		return nil
	}
	if u.Issued == 0 || time.Now().Unix()-u.Issued > int64(sessionTTL.Seconds()) {
		return nil
	}
	return &u
}

// setSessionCookie writes the auth cookie.
func (s *Server) setSessionCookie(w http.ResponseWriter, u *sessionUser) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.makeSession(u),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

// clearSessionCookie expires the auth cookie.
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: s.cfg.CookieSecure,
		MaxAge: -1,
	})
}

// requireRole wraps a handler so only authenticated users with the given role
// may proceed; everyone else is redirected to /login.
func (s *Server) requireRole(role string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := s.currentSession(r)
		if u == nil || u.Role != role {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

// requireAuth wraps a handler so any authenticated user (admin or
// case_manager) may proceed; others redirect to /login.
func (s *Server) requireAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := s.currentSession(r)
		if u == nil {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

// handleLogin GET renders the login form; POST validates against PB users and
// sets the session cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		_ = s.tpl.ExecuteTemplate(w, "login", map[string]any{
			"Error":     "",
			"Next":      r.URL.Query().Get("next"),
			"IsAuthed":  false,
			"CsrfToken": requestCtxToken(r),
		})
		return
	}
	_ = r.ParseForm()
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	next := r.FormValue("next")
	if next == "" {
		next = "/"
	}
	if !isSafeRedirect(next) {
		next = "/"
	}

	rec, err := s.pb.FindAuthRecordByEmail("users", email)
	if err != nil || rec == nil || !rec.ValidatePassword(password) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = s.tpl.ExecuteTemplate(w, "login", map[string]any{
			"Error":     "Invalid email or password.",
			"Next":      next,
			"IsAuthed":  false,
			"CsrfToken": requestCtxToken(r),
		})
		return
	}
	role := rec.GetString("role")
	if role != "admin" && role != "case_manager" && role != "intake" {
		role = "case_manager"
	}
	u := &sessionUser{
		ID:     rec.Id,
		Email:  rec.GetString("email"),
		Name:   rec.GetString("name"),
		Role:   role,
		Issued: time.Now().Unix(),
	}
	s.setSessionCookie(w, u)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// handleLogout clears the session and redirects to /login.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// generateCsrfToken returns a random 32-byte hex token.
func generateCsrfToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a timestamp-based token only if crypto/rand fails.
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

// csrfCookie returns the current CSRF token from the request, or an empty
// string if the cookie is missing.
func csrfCookie(r *http.Request) string {
	c, err := r.Cookie(csrfCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// requestCtxToken returns the CSRF token stashed in the request context by
// csrfMiddleware, falling back to the cookie. Used by handlers that render
// the token into forms.
func requestCtxToken(r *http.Request) string {
	if v, ok := r.Context().Value(ctxCsrfToken).(string); ok {
		return v
	}
	return csrfCookie(r)
}

// setCSRFCookie writes the double-submit CSRF cookie.
func (s *Server) setCSRFCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

// csrfMiddleware enforces the double-submit CSRF token on state-changing
// requests. Safe methods only ensure the cookie exists so the next unsafe
// request can carry the token. The token is also accepted as a form field for
// no-JS fallbacks.
func (s *Server) csrfMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := csrfCookie(r)
		if token == "" {
			token = generateCsrfToken()
			s.setCSRFCookie(w, token)
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxCsrfToken, token))

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			h(w, r)
			return
		}

		got := r.Header.Get(csrfHeaderName)
		if got == "" {
			got = r.PostFormValue(csrfFormName)
		}
		if got == "" {
			_ = r.ParseForm()
			got = r.FormValue(csrfFormName)
		}
		if !hmac.Equal([]byte(token), []byte(got)) {
			http.Error(w, "invalid or missing csrf token", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}
