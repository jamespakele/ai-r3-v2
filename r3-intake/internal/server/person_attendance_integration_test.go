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

// personAttendanceFixtures holds the record ids created by
// seedPersonAttendanceData so tests can reference them.
type personAttendanceFixtures struct {
	site, ev, admin1, cm1, cm2, i1, i2 string
}

// seedPersonAttendanceData creates one site, one event, three users, two
// intakes (i1 assigned to cm1, i2 assigned to cm2), and five attendance records
// for i1 across August 2026 with a gap to exercise streak logic.
func seedPersonAttendanceData(t *testing.T, pb *pocketbase.PocketBase) personAttendanceFixtures {
	t.Helper()
	save := func(name string, rec *core.Record) string {
		t.Helper()
		if err := pb.Save(rec); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
		return rec.Id
	}
	rec := func(name string) *core.Record {
		col, err := pb.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("collection %s: %v", name, err)
		}
		return core.NewRecord(col)
	}

	site := save("site", func() *core.Record {
		r := rec("sites")
		r.Set("name", "Kona")
		r.Set("active", true)
		return r
	}())

	ev := save("ev", func() *core.Record {
		r := rec("events")
		r.Set("site", site)
		r.Set("name", "Morning Program")
		r.Set("start_date", "2026-08-01")
		r.Set("end_date", "2026-08-31")
		r.Set("status", "active")
		return r
	}())

	admin1 := save("admin1", func() *core.Record {
		r := rec("users")
		r.SetEmail("admin@example.com")
		r.SetPassword("admin-password")
		r.Set("name", "Admin One")
		r.Set("role", "admin")
		return r
	}())
	cm1 := save("cm1", func() *core.Record {
		r := rec("users")
		r.SetEmail("cm1@example.com")
		r.SetPassword("cm1-password")
		r.Set("name", "CM One")
		r.Set("role", "case_manager")
		return r
	}())
	cm2 := save("cm2", func() *core.Record {
		r := rec("users")
		r.SetEmail("cm2@example.com")
		r.SetPassword("cm2-password")
		r.Set("name", "CM Two")
		r.Set("role", "case_manager")
		return r
	}())

	i1 := save("i1", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Alice")
		r.Set("site", site)
		r.Set("assigned_to", cm1)
		return r
	}())
	i2 := save("i2", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Bob")
		r.Set("site", site)
		r.Set("assigned_to", cm2)
		return r
	}())

	// Five attendance records for i1 in August 2026, with a gap (08-05..08-09)
	// so the streak stops at the most recent present date.
	save("att-1", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", i1)
		r.Set("site", site)
		r.Set("event", ev)
		r.Set("date", "2026-08-01")
		r.Set("status", "present")
		r.Set("recorded_by", admin1)
		r.Set("check_in_time", "2026-08-01 20:30:00")
		r.Set("note", "on time")
		return r
	}())
	save("att-2", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", i1)
		r.Set("site", site)
		r.Set("date", "2026-08-02")
		r.Set("status", "walk_in")
		r.Set("recorded_by", admin1)
		return r
	}())
	save("att-3", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", i1)
		r.Set("site", site)
		r.Set("date", "2026-08-03")
		r.Set("status", "absent")
		return r
	}())
	save("att-4", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", i1)
		r.Set("site", site)
		r.Set("date", "2026-08-04")
		r.Set("status", "excused")
		r.Set("note", "doctor")
		return r
	}())
	save("att-5", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", i1)
		r.Set("site", site)
		r.Set("date", "2026-08-10")
		r.Set("status", "present")
		return r
	}())

	return personAttendanceFixtures{site, ev, admin1, cm1, cm2, i1, i2}
}

// doPersonAttendance issues a request against the server mux with an optional
// cookie and form body.
func doPersonAttendance(srv *Server, cookie *http.Cookie, method, path string, form url.Values) *httptest.ResponseRecorder {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

// countAttendance returns the number of attendance records for (intake, date).
func countAttendance(t *testing.T, srv *Server, intakeID, date string) int {
	t.Helper()
	col, err := srv.pb.FindCollectionByNameOrId("attendance")
	if err != nil {
		t.Fatalf("attendance collection: %v", err)
	}
	recs, err := srv.pb.FindRecordsByFilter(col.Id,
		"intake='"+intakeID+"' && date='"+date+"'", "", 100, 0)
	if err != nil {
		t.Fatalf("find attendance: %v", err)
	}
	return len(recs)
}

// findAttendance returns the single attendance record for (intake, date), or
// nil if none exists.
func findAttendance(t *testing.T, srv *Server, intakeID, date string) *core.Record {
	t.Helper()
	col, err := srv.pb.FindCollectionByNameOrId("attendance")
	if err != nil {
		t.Fatalf("attendance collection: %v", err)
	}
	recs, err := srv.pb.FindRecordsByFilter(col.Id,
		"intake='"+intakeID+"' && date='"+date+"'", "", 1, 0)
	if err != nil {
		t.Fatalf("find attendance: %v", err)
	}
	if len(recs) == 0 {
		return nil
	}
	return recs[0]
}

func TestPersonAttendanceAuthz(t *testing.T) {
	srv := newTestServer(t)
	fx := seedPersonAttendanceData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)
	cm1 := cmCookie(srv, fx.cm1)

	t.Run("admin views both intakes", func(t *testing.T) {
		for _, id := range []string{fx.i1, fx.i2} {
			rec := doPersonAttendance(srv, admin, "GET", "/intake/"+id+"/attendance", nil)
			if rec.Code != http.StatusOK {
				t.Errorf("admin GET %s = %d, want 200", id, rec.Code)
			}
		}
	})

	t.Run("cm1 views assigned intake", func(t *testing.T) {
		rec := doPersonAttendance(srv, cm1, "GET", "/intake/"+fx.i1+"/attendance", nil)
		if rec.Code != http.StatusOK {
			t.Errorf("cm1 GET i1 = %d, want 200", rec.Code)
		}
	})

	t.Run("cm1 forbidden on other intake", func(t *testing.T) {
		rec := doPersonAttendance(srv, cm1, "GET", "/intake/"+fx.i2+"/attendance", nil)
		if rec.Code != http.StatusForbidden {
			t.Errorf("cm1 GET i2 = %d, want 403", rec.Code)
		}
	})

	t.Run("unauthenticated redirects to login", func(t *testing.T) {
		rec := doPersonAttendance(srv, nil, "GET", "/intake/"+fx.i1+"/attendance", nil)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("unauthenticated = %d, want 303", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/login") {
			t.Errorf("Location = %q, want /login prefix", loc)
		}
	})
}

func TestPersonAttendanceMonthRender(t *testing.T) {
	srv := newTestServer(t)
	fx := seedPersonAttendanceData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doPersonAttendance(srv, admin, "GET", "/intake/"+fx.i1+"/attendance?month=2026-08", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Alice", "Kona",
		"3 of 5 days (60%)",
		"Current streak: 1",
		"Present", "Absent", "Excused", "Walk-in",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("month render missing %q", want)
		}
	}
}

func TestPersonAttendanceDayGet(t *testing.T) {
	srv := newTestServer(t)
	fx := seedPersonAttendanceData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	t.Run("date with record", func(t *testing.T) {
		rec := doPersonAttendance(srv, admin, "GET", "/intake/"+fx.i1+"/attendance/day?date=2026-08-01", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{"Present", "Morning Program", "Admin One", "2026-08-01 20:30:00", "on time"} {
			if !strings.Contains(body, want) {
				t.Errorf("day detail missing %q", want)
			}
		}
	})

	t.Run("date without record", func(t *testing.T) {
		rec := doPersonAttendance(srv, admin, "GET", "/intake/"+fx.i1+"/attendance/day?date=2026-08-15", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "No attendance recorded") {
			t.Errorf("empty day missing empty-state text")
		}
	})
}

func TestPersonAttendanceDaySaveCreate(t *testing.T) {
	srv := newTestServer(t)
	fx := seedPersonAttendanceData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{"date": {"2026-08-20"}, "status": {"present"}, "note": {"hello"}}
	rec := doPersonAttendance(srv, admin, "POST", "/intake/"+fx.i1+"/attendance/day", form)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "person-attendance-calendar") {
		t.Errorf("save response missing calendar fragment")
	}

	// The record exists with site derived from the intake and recorded_by set.
	att := findAttendance(t, srv, fx.i1, "2026-08-20")
	if att == nil {
		t.Fatalf("expected attendance record for 2026-08-20")
	}
	if att.GetString("site") != fx.site {
		t.Errorf("site = %q, want %q", att.GetString("site"), fx.site)
	}
	if att.GetString("recorded_by") != fx.admin1 {
		t.Errorf("recorded_by = %q, want %q", att.GetString("recorded_by"), fx.admin1)
	}
	if att.GetString("status") != "present" || att.GetString("note") != "hello" {
		t.Errorf("status/note = %q/%q, want present/hello", att.GetString("status"), att.GetString("note"))
	}

	// The day fragment now shows the new record.
	day := doPersonAttendance(srv, admin, "GET", "/intake/"+fx.i1+"/attendance/day?date=2026-08-20", nil)
	if !strings.Contains(day.Body.String(), "hello") {
		t.Errorf("day fragment missing new note")
	}
}

func TestPersonAttendanceDaySaveUpdate(t *testing.T) {
	srv := newTestServer(t)
	fx := seedPersonAttendanceData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	// First POST updates the existing 2026-08-01 record.
	form1 := url.Values{"date": {"2026-08-01"}, "status": {"absent"}, "note": {"updated"}}
	if rec := doPersonAttendance(srv, admin, "POST", "/intake/"+fx.i1+"/attendance/day", form1); rec.Code != http.StatusOK {
		t.Fatalf("first POST = %d, want 200", rec.Code)
	}
	// Second POST on the same date.
	form2 := url.Values{"date": {"2026-08-01"}, "status": {"present"}, "note": {"final"}}
	if rec := doPersonAttendance(srv, admin, "POST", "/intake/"+fx.i1+"/attendance/day", form2); rec.Code != http.StatusOK {
		t.Fatalf("second POST = %d, want 200", rec.Code)
	}

	// No duplicate: exactly one record for (i1, 2026-08-01).
	if n := countAttendance(t, srv, fx.i1, "2026-08-01"); n != 1 {
		t.Fatalf("record count = %d, want 1 (no duplicate)", n)
	}
	att := findAttendance(t, srv, fx.i1, "2026-08-01")
	if att.GetString("status") != "present" || att.GetString("note") != "final" {
		t.Errorf("status/note = %q/%q, want present/final", att.GetString("status"), att.GetString("note"))
	}
}

func TestPersonAttendanceDayDelete(t *testing.T) {
	srv := newTestServer(t)
	fx := seedPersonAttendanceData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	form := url.Values{"date": {"2026-08-10"}}
	rec := doPersonAttendance(srv, admin, "POST", "/intake/"+fx.i1+"/attendance/day/delete", form)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete = %d, want 200", rec.Code)
	}
	if n := countAttendance(t, srv, fx.i1, "2026-08-10"); n != 0 {
		t.Errorf("record count after delete = %d, want 0", n)
	}

	// Second delete is an idempotent no-op success.
	rec2 := doPersonAttendance(srv, admin, "POST", "/intake/"+fx.i1+"/attendance/day/delete", form)
	if rec2.Code != http.StatusOK {
		t.Errorf("second delete = %d, want 200", rec2.Code)
	}
}

func TestPersonAttendanceDayValidation(t *testing.T) {
	srv := newTestServer(t)
	fx := seedPersonAttendanceData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	t.Run("invalid date", func(t *testing.T) {
		form := url.Values{"date": {"not-a-date"}, "status": {"present"}}
		rec := doPersonAttendance(srv, admin, "POST", "/intake/"+fx.i1+"/attendance/day", form)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("invalid date = %d, want 400", rec.Code)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		form := url.Values{"date": {"2026-08-20"}, "status": {"bogus"}}
		rec := doPersonAttendance(srv, admin, "POST", "/intake/"+fx.i1+"/attendance/day", form)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("invalid status = %d, want 400", rec.Code)
		}
	})

	t.Run("note too long", func(t *testing.T) {
		form := url.Values{"date": {"2026-08-20"}, "status": {"present"}, "note": {strings.Repeat("x", 501)}}
		rec := doPersonAttendance(srv, admin, "POST", "/intake/"+fx.i1+"/attendance/day", form)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("long note = %d, want 400", rec.Code)
		}
	})
}
