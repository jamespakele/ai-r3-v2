package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// doMatrix GETs the attendance matrix fragment with the HTMX header and the
// given query string, returning the recorder.
func doMatrix(srv *Server, cookie *http.Cookie, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/attendance?"+query, nil)
	req.Header.Set("HX-Request", "true")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// TestMatrixDefaultsToFirstEvent proves that loading /attendance without an
// event query param selects the first active event in the dropdown and scopes
// the matrix to it: the event option is marked selected and the placeholder
// is gone.
func TestMatrixDefaultsToFirstEvent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doMatrix(srv, admin, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<option value="` + fx.ev + `" selected>Morning Program</option>`,
		"Located Alice",
		"NoSite Bob",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("matrix body missing %q", want)
		}
	}
	for _, forbid := range []string{
		"Select an event…",
		"Create an Event to track attendance.",
	} {
		if strings.Contains(body, forbid) {
			t.Errorf("matrix body should not contain %q", forbid)
		}
	}
}

// TestMatrixNoEventsEmptyState proves that with zero active events the matrix
// renders the create-an-event empty state linking to /admin.
func TestMatrixNoEventsEmptyState(t *testing.T) {
	srv := newTestServer(t)
	admin := adminCookie(srv, "admin1")

	rec := doMatrix(srv, admin, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Create an Event to track attendance.",
		`href="/admin"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("matrix body missing %q", want)
		}
	}
	for _, forbid := range []string{
		"Select an event…",
	} {
		if strings.Contains(body, forbid) {
			t.Errorf("matrix body should not contain %q", forbid)
		}
	}
}

// TestStatsDefaultsToFirstEvent proves the stat cards resolve the same
// effective event as the matrix when no event is in the query string: a
// toggle recorded under the default event is reflected by the no-param stats
// call.
func TestStatsDefaultsToFirstEvent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"intake_id": {fx.iLocated},
		"date":      {"2026-08-10"},
		"site_id":   {fx.site},
		"from":      {"2026-08-01"},
		"to":        {"2026-08-14"},
		"event_id":  {fx.ev},
	}
	if rec := doToggle(srv, admin, form); rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d, want 200", rec.Code)
	}

	rec := doStats(srv, admin, "from=2026-08-01&to=2026-08-14")
	if rec.Code != http.StatusOK {
		t.Fatalf("stats status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`id="stat-cards"`,
		`>1</div><div class="stat-label">Total check-ins`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stats body missing %q", want)
		}
	}
}
