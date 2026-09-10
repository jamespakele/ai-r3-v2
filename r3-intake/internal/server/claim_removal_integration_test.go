package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// Focused acceptance tests for the claim-feature removal: any signed-in user
// may access and work on any intake record, the claim workflow is gone, and
// new records are no longer auto-claimed.

// TestCrossUserIntakeAccess proves a case manager can open and save a
// section on an intake created by another user (the old gate bounced both
// actions to login).
func TestCrossUserIntakeAccess(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	cmID := createUser(t, srv.pb, "cm2@example.com", "CM Two", "case_manager", "cm-password", false)
	admin := adminCookie(srv, fx.admin)
	cm := cmCookie(srv, cmID)

	// Admin creates the record.
	rec := doSectionPost(t, srv, admin, true, "01", validSection01Form(fx, nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("admin create = %d, want 202", rec.Code)
	}
	id := firstIntakeRecord(t, srv).Id

	// Case manager opens it.
	req := httptest.NewRequest(http.MethodGet, "/intake/"+id, nil)
	req.AddCookie(cm)
	view := httptest.NewRecorder()
	srv.Mux().ServeHTTP(view, req)
	if view.Code != http.StatusOK {
		t.Fatalf("cm GET /intake/%s = %d, want 200", id, view.Code)
	}

	// Case manager saves a section on it.
	form := url.Values{"id": {id}, "mentalHealth": {"yes"}}
	save := doSectionPost(t, srv, cm, true, "02", form)
	if save.Code != http.StatusNoContent {
		t.Fatalf("cm POST /section/02 = %d, want 204", save.Code)
	}
	if got := firstIntakeRecord(t, srv).GetString("mentalHealth"); got != "yes" {
		t.Errorf("mentalHealth = %q, want %q", got, "yes")
	}
}

// TestClaimRouteRemoved proves POST /admin/intake/{id}/claim is gone (404).
func TestClaimRouteRemoved(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	rec := doSectionPost(t, srv, admin, true, "01", validSection01Form(fx, nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("admin create = %d, want 202", rec.Code)
	}
	id := firstIntakeRecord(t, srv).Id

	req := httptest.NewRequest(http.MethodPost, "/admin/intake/"+id+"/claim", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	req.AddCookie(admin)
	out := httptest.NewRecorder()
	srv.Mux().ServeHTTP(out, req)
	if out.Code != http.StatusNotFound {
		t.Fatalf("POST claim = %d, want 404", out.Code)
	}
}

// TestListHasNoClaimUI proves the list page renders neither a Claim button
// nor the Assigned column, nor a Claimed status-filter option.
func TestListHasNoClaimUI(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	rec := doSectionPost(t, srv, admin, true, "01", validSection01Form(fx, nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("admin create = %d, want 202", rec.Code)
	}

	out := doList(srv, admin, "")
	if out.Code != http.StatusOK {
		t.Fatalf("list = %d, want 200", out.Code)
	}
	body := out.Body.String()
	if strings.Contains(body, "/claim") {
		t.Errorf("list still renders a claim action")
	}
	if strings.Contains(body, ">Assigned<") {
		t.Errorf("list still renders the Assigned column header")
	}
}

// TestNewRecordNotAutoClaimed proves a record created by a signed-in user is
// no longer auto-claimed: status is the unassigned default and only
// created_by is set.
func TestNewRecordNotAutoClaimed(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)
	admin := adminCookie(srv, fx.admin)

	rec := doSectionPost(t, srv, admin, true, "01", validSection01Form(fx, nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create = %d, want 202", rec.Code)
	}
	saved := firstIntakeRecord(t, srv)
	if got := saved.GetString("status"); got == "claimed" {
		t.Errorf("status = %q, want not claimed (auto-claim removed)", got)
	}
	if got := saved.GetString("assigned_to"); got != "" {
		t.Errorf("assigned_to = %q, want empty", got)
	}
	if got := saved.GetString("created_by"); got != fx.admin {
		t.Errorf("created_by = %q, want %q", got, fx.admin)
	}
}

// TestCaseManagerAnySite proves the claim-based site pinning is gone: a case
// manager may select any site (or All locations) on the attendance matrix.
func TestCaseManagerAnySite(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)

	cm := &sessionUser{ID: fx.cm, Email: "cm@example.com", Name: "Case Manager", Role: "case_manager"}
	id, name := srv.resolveSite(cm, fx.site)
	if id != fx.site || name != "Kona" {
		t.Errorf("resolveSite(site) = (%q, %q), want (%q, Kona)", id, name, fx.site)
	}
	id, name = srv.resolveSite(cm, "")
	if id != "" || name != "All locations" {
		t.Errorf(`resolveSite(no param) = (%q, %q), want ("", All locations)`, id, name)
	}
}

// TestPublicResumeLegacyClaimed pins the public-resume rule: anonymously
// created records (created_by empty) are publicly resumable regardless of
// status; staff-created records require login.
func TestPublicResumeLegacyClaimed(t *testing.T) {
	srv := newTestServer(t)
	fx := seedActiveEvent(t, srv.pb)

	mk := func(status, createdBy string) string {
		col, err := srv.pb.FindCollectionByNameOrId("intake")
		if err != nil {
			t.Fatalf("intake collection: %v", err)
		}
		rec := core.NewRecord(col)
		rec.Set("name", "Resume "+status)
		rec.Set("event", fx.event)
		rec.Set("status", status)
		if createdBy != "" {
			rec.Set("created_by", createdBy)
		}
		if err := srv.pb.Save(rec); err != nil {
			t.Fatalf("save %s: %v", status, err)
		}
		return rec.Id
	}
	anonUnassigned := mk("unassigned", "")
	legacyClaimed := mk("claimed", "")
	staffCreated := mk("unassigned", fx.admin)

	get := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/public/intake?id="+id, nil)
		rec := httptest.NewRecorder()
		srv.Mux().ServeHTTP(rec, req)
		return rec
	}

	if rec := get(anonUnassigned); rec.Code != http.StatusOK {
		t.Errorf("anon unassigned = %d, want 200", rec.Code)
	}
	if rec := get(legacyClaimed); rec.Code != http.StatusOK {
		t.Errorf("legacy claimed = %d, want 200 (publicly resumable)", rec.Code)
	}
	if rec := get(staffCreated); rec.Code != http.StatusSeeOther {
		t.Errorf("staff-created = %d, want 303 to login", rec.Code)
	}
}
