package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// doUserDelete POSTs the soft-delete user mutation for the given user id with
// the supplied session cookie (nil for unauthenticated). CSRF is always
// attached so failures are attributable to auth, not CSRF.
func doUserDelete(srv *Server, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	form := url.Values{}
	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+id+"/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// createUser creates a users record with the given fields and returns its id.
// deleted is written explicitly so the record's visibility under the
// deleted=false filters is deterministic (the field is Required:false, so an
// unset value stores NULL, not false).
func createUser(t *testing.T, pb *pocketbase.PocketBase, email, name, role, password string, deleted bool) string {
	t.Helper()
	col, err := pb.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}
	rec := core.NewRecord(col)
	rec.SetEmail(email)
	rec.Set("name", name)
	rec.Set("role", role)
	rec.SetPassword(password)
	rec.Set("deleted", deleted)
	if err := pb.Save(rec); err != nil {
		t.Fatalf("save user %s: %v", name, err)
	}
	return rec.Id
}

// setDeleted flips the deleted flag on an existing user record.
func setDeleted(t *testing.T, pb *pocketbase.PocketBase, id string, deleted bool) {
	t.Helper()
	rec, err := pb.FindRecordById("users", id)
	if err != nil {
		t.Fatalf("find user %s: %v", id, err)
	}
	rec.Set("deleted", deleted)
	if err := pb.Save(rec); err != nil {
		t.Fatalf("save user %s: %v", id, err)
	}
}

// getDeleted re-fetches a user record and returns its deleted flag.
func getDeleted(t *testing.T, pb *pocketbase.PocketBase, id string) bool {
	t.Helper()
	rec, err := pb.FindRecordById("users", id)
	if err != nil {
		t.Fatalf("find user %s: %v", id, err)
	}
	return rec.GetBool("deleted")
}

func TestAdminUserDeleteSoftDeletesCaseManager(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	cm := createUser(t, srv.pb, "cm@example.com", "CM One", "case_manager", "cm-password", false)

	rec := doUserDelete(srv, adminCookie(srv, fx.admin1), cm)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin?tab=users" {
		t.Fatalf("expected redirect to /admin?tab=users, got %q", loc)
	}
	if !getDeleted(t, srv.pb, cm) {
		t.Fatal("expected case_manager to be soft-deleted")
	}
}

func TestAdminUserDeleteSelfDeleteGuardrail(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)

	rec := doUserDelete(srv, adminCookie(srv, fx.admin1), fx.admin1)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin?tab=users" {
		t.Fatalf("expected redirect to /admin?tab=users, got %q", loc)
	}
	if getDeleted(t, srv.pb, fx.admin1) {
		t.Fatal("expected admin1 NOT to be deleted (self-delete guardrail)")
	}
}

func TestAdminUserDeleteLastAdminGuardrail(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	// admin2 is the only non-deleted admin besides admin1; mark admin2 deleted
	// so the count of non-deleted admins is exactly 1 (admin1, which is NULL
	// under deleted=false until set explicitly below).
	admin2 := createUser(t, srv.pb, "admin2@example.com", "Admin Two", "admin", "admin2-password", false)
	setDeleted(t, srv.pb, admin2, true)
	// admin1 must be visible to the deleted=false count query.
	setDeleted(t, srv.pb, fx.admin1, false)

	rec := doUserDelete(srv, adminCookie(srv, fx.admin1), admin2)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin?tab=users" {
		t.Fatalf("expected redirect to /admin?tab=users, got %q", loc)
	}
	if !getDeleted(t, srv.pb, admin2) {
		t.Fatal("expected admin2 to remain deleted (last-admin guardrail refused re-delete)")
	}
}

func TestAdminUserDeleteAnotherAdminWhenTwoAdmins(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	// Two non-deleted admins: admin1 (set explicitly) and admin2.
	setDeleted(t, srv.pb, fx.admin1, false)
	admin2 := createUser(t, srv.pb, "admin2@example.com", "Admin Two", "admin", "admin2-password", false)

	rec := doUserDelete(srv, adminCookie(srv, fx.admin1), admin2)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin?tab=users" {
		t.Fatalf("expected redirect to /admin?tab=users, got %q", loc)
	}
	if !getDeleted(t, srv.pb, admin2) {
		t.Fatal("expected admin2 to be soft-deleted (two admins allows deletion)")
	}
}

func TestAdminUserDeleteNonExistentUser(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	rec := doUserDelete(srv, adminCookie(srv, fx.admin1), "nonexistent-id")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAdminUserDeleteUnauthenticated(t *testing.T) {
	srv := newTestServer(t)
	cm := createUser(t, srv.pb, "cm@example.com", "CM One", "case_manager", "cm-password", false)

	rec := doUserDelete(srv, nil, cm)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/login") {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
	if getDeleted(t, srv.pb, cm) {
		t.Fatal("expected no deletion for unauthenticated request")
	}
}

func TestAdminUserDeleteNonAdminForbidden(t *testing.T) {
	srv := newTestServer(t)
	cm := createUser(t, srv.pb, "cm@example.com", "CM One", "case_manager", "cm-password", false)

	// A case_manager is authenticated but not admin; handleAdminSub's default
	// case returns 404 (no matching admin-gated route).
	rec := doUserDelete(srv, cmCookie(srv, cm), cm)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if getDeleted(t, srv.pb, cm) {
		t.Fatal("expected no deletion for non-admin request")
	}
}

func TestSoftDeletedUserExcludedFromLoadUsers(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	setDeleted(t, srv.pb, fx.admin1, false)
	cm := createUser(t, srv.pb, "cm@example.com", "CM One", "case_manager", "cm-password", false)

	// Soft-delete the case_manager via the handler.
	rec := doUserDelete(srv, adminCookie(srv, fx.admin1), cm)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}

	rows := srv.loadUsers()
	for _, row := range rows {
		if row.ID == cm {
			t.Fatalf("expected soft-deleted user %s to be excluded from loadUsers", cm)
		}
	}
}

func TestSoftDeletedCaseManagerExcludedFromLoadCaseManagers(t *testing.T) {
	srv := newTestServer(t)
	fx := seedToggleData(t, srv.pb)
	cm1 := createUser(t, srv.pb, "cm1@example.com", "CM One", "case_manager", "cm1-password", false)
	cm2 := createUser(t, srv.pb, "cm2@example.com", "CM Two", "case_manager", "cm2-password", false)

	// Soft-delete cm1 via the handler.
	rec := doUserDelete(srv, adminCookie(srv, fx.admin1), cm1)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}

	managers := srv.loadCaseManagers()
	foundCM1, foundCM2 := false, false
	for _, m := range managers {
		if m.ID == cm1 {
			foundCM1 = true
		}
		if m.ID == cm2 {
			foundCM2 = true
		}
	}
	if foundCM1 {
		t.Fatalf("expected soft-deleted case_manager %s to be excluded from loadCaseManagers", cm1)
	}
	if !foundCM2 {
		t.Fatalf("expected non-deleted case_manager %s to be present in loadCaseManagers", cm2)
	}
}

func TestSoftDeletedUserCannotLogin(t *testing.T) {
	srv := newTestServer(t)
	createUser(t, srv.pb, "deleted@example.com", "Deleted User", "case_manager", "known-password", true)

	form := url.Values{}
	form.Set("email", "deleted@example.com")
	form.Set("password", "known-password")
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addCSRFToRequest(req)
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid email or password.") {
		t.Fatalf("expected generic login error, got body: %s", rec.Body.String())
	}
}
