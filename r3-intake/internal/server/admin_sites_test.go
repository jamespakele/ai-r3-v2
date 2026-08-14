package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// doSiteUpdate POSTs the Update Site form to /admin/sites/{id}/update.
func doSiteUpdate(srv *Server, cookie *http.Cookie, id string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/admin/sites/"+id+"/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// doSiteDelete POSTs the soft-delete request to /admin/sites/{id}/delete.
func doSiteDelete(srv *Server, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/admin/sites/"+id+"/delete", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// findSiteByName returns the sites record with the given name, or nil.
func findSiteByName(t *testing.T, srv *Server, name string) *core.Record {
	t.Helper()
	col, err := srv.sitesCollection()
	if err != nil {
		t.Fatalf("sites collection: %v", err)
	}
	recs, err := srv.pb.FindRecordsByFilter(col.Id, "name='"+name+"'", "", 10, 0)
	if err != nil {
		t.Fatalf("find site %q: %v", name, err)
	}
	if len(recs) == 0 {
		return nil
	}
	return recs[0]
}

// TestAdminSiteUpdateSuccess proves a valid update mutates name/address and
// 303-redirects to the sites tab.
func TestAdminSiteUpdateSuccess(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{
		"name":    {"Kona Updated"},
		"address": {"123 New St"},
	}
	rec := doSiteUpdate(srv, admin, fx.site, form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (site updated)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin?tab=sites" {
		t.Errorf("Location = %q, want /admin?tab=sites", loc)
	}

	site := findSiteByName(t, srv, "Kona Updated")
	if site == nil {
		t.Fatal("expected site 'Kona Updated' after update, got none")
	}
	if got := site.GetString("address"); got != "123 New St" {
		t.Errorf("address = %q, want 123 New St", got)
	}
	if site.GetBool("deleted") {
		t.Error("deleted = true, want false (update must not soft-delete)")
	}
}

// TestAdminSiteUpdateEmptyName proves an empty name is rejected (no-op redirect).
func TestAdminSiteUpdateEmptyName(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{"name": {"   "}, "address": {"x"}}
	rec := doSiteUpdate(srv, admin, fx.site, form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (no-op redirect)", rec.Code)
	}
	site := findSiteByName(t, srv, "Kona")
	if site == nil {
		t.Fatal("expected site 'Kona' to remain unchanged, got none")
	}
	if got := site.GetString("address"); got != "" {
		t.Errorf("address = %q, want unchanged empty", got)
	}
}

// TestAdminSiteUpdateNotFound proves update 404s on a missing id.
func TestAdminSiteUpdateNotFound(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doSiteUpdate(srv, admin, "nonexistent", url.Values{"name": {"X"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestAdminSiteDeleteSuccess proves soft-delete sets deleted=true and
// 303-redirects.
func TestAdminSiteDeleteSuccess(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doSiteDelete(srv, admin, fx.site)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (site deleted)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin?tab=sites" {
		t.Errorf("Location = %q, want /admin?tab=sites", loc)
	}

	site := findSiteByName(t, srv, "Kona")
	if site == nil {
		t.Fatal("expected site record to remain after soft-delete, got none")
	}
	if !site.GetBool("deleted") {
		t.Error("deleted = false, want true")
	}
}

// TestAdminSiteDeleteIdempotent proves deleting an already-deleted site is a
// no-op that still 303-redirects.
func TestAdminSiteDeleteIdempotent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	if rec := doSiteDelete(srv, admin, fx.site); rec.Code != http.StatusSeeOther {
		t.Fatalf("first delete status = %d, want 303", rec.Code)
	}
	if rec := doSiteDelete(srv, admin, fx.site); rec.Code != http.StatusSeeOther {
		t.Fatalf("second delete status = %d, want 303 (idempotent)", rec.Code)
	}
	site := findSiteByName(t, srv, "Kona")
	if site == nil {
		t.Fatal("expected site record to remain, got none")
	}
	if !site.GetBool("deleted") {
		t.Error("deleted = false, want true")
	}
}

// TestAdminSiteDeleteNotFound proves delete 404s on a missing id.
func TestAdminSiteDeleteNotFound(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doSiteDelete(srv, admin, "nonexistent")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestLoadSitesExcludesDeleted proves the loadSites helper omits soft-deleted
// sites in both active-only and include-inactive modes.
func TestLoadSitesExcludesDeleted(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)

	// Soft-delete the seeded site.
	site := findSiteByName(t, srv, "Kona")
	site.Set("deleted", true)
	if err := srv.pb.Save(site); err != nil {
		t.Fatalf("mark site deleted: %v", err)
	}

	active, err := srv.loadSites(false)
	if err != nil {
		t.Fatalf("loadSites(false): %v", err)
	}
	for _, s := range active {
		if s.ID == fx.site {
			t.Error("loadSites(false) returned a soft-deleted site")
		}
	}

	all, err := srv.loadSites(true)
	if err != nil {
		t.Fatalf("loadSites(true): %v", err)
	}
	for _, s := range all {
		if s.ID == fx.site {
			t.Error("loadSites(true) returned a soft-deleted site")
		}
	}
}
