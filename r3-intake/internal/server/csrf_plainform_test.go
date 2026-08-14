package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// postLogout dispatches a POST to /logout through the full mux. If withForm is
// true it includes the csrf_token value as a url-encoded form field and no
// X-CSRF-Token header; if false it includes only the r3_csrf cookie.
func postLogout(t *testing.T, srv *Server, withForm bool) *httptest.ResponseRecorder {
	t.Helper()

	var body string
	if withForm {
		body = "csrf_token=" + testCsrfToken
	}

	req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrfCookieForTest())

	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// TestPlainFormCSRFViaFormField proves a plain HTML form can submit a
// state-changing POST by carrying the token in a hidden csrf_token input.
func TestPlainFormCSRFViaFormField(t *testing.T) {
	srv := newTestServer(t)
	rec := postLogout(t, srv, true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 See Other, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestPlainFormCSRFMissingRejected proves a POST with the r3_csrf cookie but
// neither header nor form field is rejected with 403.
func TestPlainFormCSRFMissingRejected(t *testing.T) {
	srv := newTestServer(t)
	rec := postLogout(t, srv, false)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid or missing csrf token") {
		t.Fatalf("expected csrf error body, got: %s", rec.Body.String())
	}
}
