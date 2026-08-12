package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookieName = "r3_session"
	sessionTTL        = 12 * time.Hour
)

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
	return &u
}

// setSessionCookie writes the auth cookie.
func (s *Server) setSessionCookie(w http.ResponseWriter, u *sessionUser) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.makeSession(u),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

// clearSessionCookie expires the auth cookie.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", HttpOnly: true,
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
			"Error":    "",
			"Next":     r.URL.Query().Get("next"),
			"IsAuthed": false,
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

	rec, err := s.pb.FindAuthRecordByEmail("users", email)
	if err != nil || rec == nil || !rec.ValidatePassword(password) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = s.tpl.ExecuteTemplate(w, "login", map[string]any{
			"Error":    "Invalid email or password.",
			"Next":     next,
			"IsAuthed": false,
		})
		return
	}
	role := rec.GetString("role")
	if role != "admin" && role != "case_manager" && role != "intake" {
		role = "case_manager"
	}
	u := &sessionUser{
		ID:    rec.Id,
		Email: rec.GetString("email"),
		Name:  rec.GetString("name"),
		Role:  role,
	}
	s.setSessionCookie(w, u)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// handleLogout clears the session and redirects to /login.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
