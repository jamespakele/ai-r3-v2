package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// doStats GETs the attendance stats fragment with the HTMX header and the
// given query string, returning the recorder.
func doStats(srv *Server, cookie *http.Cookie, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/attendance/stats?"+query, nil)
	req.Header.Set("HX-Request", "true")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// TestStatsEndpoint verifies the /attendance/stats fragment: it is
// auth-gated, renders the stat-cards labels, and reflects the attendance for
// the selected event.
func TestStatsEndpoint(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	t.Run("unauthenticated redirected to login", func(t *testing.T) {
		rec := doStats(srv, nil, "")
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/login?next=") {
			t.Errorf("Location = %q, want /login?next=...", loc)
		}
	})

	t.Run("renders fragment reflecting selected event", func(t *testing.T) {
		// Toggle one cell for the located intake under event fx.ev.
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

		query := "from=2026-08-01&to=2026-08-14&event_id=" + url.QueryEscape(fx.ev)
		rec := doStats(srv, admin, query)
		if rec.Code != http.StatusOK {
			t.Fatalf("stats status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{
			`id="stat-cards"`,
			"Total check-ins", "Active participants", "Avg attendance rate",
			`>1</div><div class="stat-label">Total check-ins`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("stats body missing %q", want)
			}
		}

		// A different event sees none of this attendance (event-scoped).
		other := "from=2026-08-01&to=2026-08-14&event_id=other-event"
		recOther := doStats(srv, admin, other)
		if recOther.Code != http.StatusOK {
			t.Fatalf("stats (other event) status = %d, want 200", recOther.Code)
		}
		if strings.Contains(recOther.Body.String(), `>1</div><div class="stat-label">Total check-ins`) {
			t.Errorf("stats for unrelated event should not count the toggled attendance")
		}
	})

	t.Run("renders fragment when no event selected", func(t *testing.T) {
		query := "from=2026-08-01&to=2026-08-14"
		rec := doStats(srv, admin, query)
		if rec.Code != http.StatusOK {
			t.Fatalf("stats (no event) status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{`id="stat-cards"`, "Total check-ins", "Avg attendance rate"} {
			if !strings.Contains(body, want) {
				t.Errorf("stats (no event) body missing %q", want)
			}
		}
	})
}
