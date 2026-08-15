package server

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/jsvm"

	"path/filepath"

	"r3-intake/internal/config"
	pbmigrations "r3-intake/pocketbase/migrations"
)

// newTestServer boots a real in-process PocketBase with embedded JS migrations
// extracted to a temp dir and a fresh temp data dir, then builds a Server on
// top of it. This mirrors the runMCP bootstrap pattern so the export handler
// exercises the real schema and data path.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	migrationsDir, err := filepath.Abs(filepath.Join("..", "..", "pocketbase", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	pb := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir:  dataDir + "/pb_data",
		HideStartBanner: true,
	})
	jsvm.MustRegister(pb, jsvm.Config{MigrationsDir: migrationsDir})
	pbmigrations.Register(pb)
	if err := pb.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := pb.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pb.ResetBootstrapState() })

	cfg := config.Config{
		SessionKey:     "test-session-key",
		PBInternalAddr: "127.0.0.1:0",
		PBRootDir:      dataDir,
		DataDir:        dataDir,
	}
	srv, err := New(cfg, pb)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

// exportFixtures holds the record ids created by seedExportData so tests can
// reference them for filter assertions.
type exportFixtures struct {
	site1, site2, ev1, ev2, admin1, cm1, i1, i2 string
}

// seedExportData creates two sites, two events, two users, two intakes, and
// five attendance records spanning 2026-08-01..2026-08-10 with a mix of
// statuses, relations, and field population.
func seedExportData(t *testing.T, pb *pocketbase.PocketBase) exportFixtures {
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

	site1 := save("site1", func() *core.Record {
		r := rec("sites")
		r.Set("name", "Kona")
		r.Set("active", true)
		return r
	}())
	site2 := save("site2", func() *core.Record {
		r := rec("sites")
		r.Set("name", "Waianae")
		r.Set("active", true)
		return r
	}())

	ev1 := save("ev1", func() *core.Record {
		r := rec("events")
		r.Set("site", site1)
		r.Set("name", "Morning Program")
		r.Set("start_date", "2026-08-01")
		r.Set("end_date", "2026-08-31")
		r.Set("status", "active")
		return r
	}())
	ev2 := save("ev2", func() *core.Record {
		r := rec("events")
		r.Set("site", site2)
		r.Set("name", "Job Fair")
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
		r.SetEmail("cm@example.com")
		r.SetPassword("cm-password")
		r.Set("name", "CM One")
		r.Set("role", "case_manager")
		return r
	}())

	i1 := save("i1", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Alice")
		r.Set("event", ev1)
		return r
	}())
	i2 := save("i2", func() *core.Record {
		r := rec("intake")
		r.Set("name", "Bob")
		r.Set("event", ev2)
		return r
	}())

	// Five attendance records.
	save("att-1", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", i1)
		r.Set("site", site1)
		r.Set("event", ev1)
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
		r.Set("site", site1)
		r.Set("event", ev1)
		r.Set("date", "2026-08-02")
		r.Set("status", "walk_in")
		r.Set("recorded_by", admin1)
		r.Set("note", "late, note with, comma")
		return r
	}())
	save("att-3", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", i2)
		r.Set("site", site2)
		r.Set("event", ev2)
		r.Set("date", "2026-08-02")
		r.Set("status", "absent")
		return r
	}())
	save("att-4", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", i2)
		r.Set("site", site2)
		r.Set("event", ev2)
		r.Set("date", "2026-08-05")
		r.Set("status", "excused")
		r.Set("note", "doctor")
		return r
	}())
	save("att-5", func() *core.Record {
		r := rec("attendance")
		r.Set("intake", i1)
		r.Set("site", site1)
		r.Set("event", ev2)
		r.Set("date", "2026-08-10")
		r.Set("status", "present")
		return r
	}())

	return exportFixtures{site1, site2, ev1, ev2, admin1, cm1, i1, i2}
}

func adminCookie(srv *Server, id string) *http.Cookie {
	v := srv.makeSession(&sessionUser{ID: id, Email: "admin@example.com", Name: "Admin One", Role: "admin", Issued: time.Now().Unix()})
	return &http.Cookie{Name: sessionCookieName, Value: v}
}

func cmCookie(srv *Server, id string) *http.Cookie {
	v := srv.makeSession(&sessionUser{ID: id, Email: "cm@example.com", Name: "CM One", Role: "case_manager", Issued: time.Now().Unix()})
	return &http.Cookie{Name: sessionCookieName, Value: v}
}

func doExport(srv *Server, cookie *http.Cookie, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/attendance/export"+query, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Mux().ServeHTTP(rec, req)
	return rec
}

func parseCSVBody(t *testing.T, rec *httptest.ResponseRecorder) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(rec.Body.String()))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	return rows
}

// TestExportCSVPermissions exercises the route-level permission matrix: an
// unauthenticated request and a case_manager both redirect to /login, while an
// admin gets the CSV. A tampered signature is treated as unauthenticated.
func TestExportCSVPermissions(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)

	cases := []struct {
		name   string
		cookie *http.Cookie
		want   int
	}{
		{"unauthenticated", nil, http.StatusSeeOther},
		{"case manager", cmCookie(srv, fx.cm1), http.StatusSeeOther},
		{"tampered signature", &http.Cookie{Name: sessionCookieName, Value: tamperCookie(adminCookie(srv, fx.admin1).Value)}, http.StatusSeeOther},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doExport(srv, tc.cookie, "")
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if loc := rec.Header().Get("Location"); loc != "/login" {
				t.Errorf("Location = %q, want /login", loc)
			}
		})
	}

	t.Run("admin", func(t *testing.T) {
		rec := doExport(srv, adminCookie(srv, fx.admin1), "?event="+fx.ev1)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
			t.Errorf("Content-Type = %q, want text/csv", ct)
		}
		cd := rec.Header().Get("Content-Disposition")
		if !strings.Contains(cd, `attachment; filename="attendance_export_`) || !strings.Contains(cd, ".csv\"") {
			t.Errorf("Content-Disposition = %q, want attachment filename", cd)
		}
		rows := parseCSVBody(t, rec)
		if len(rows) < 2 || !strings.HasPrefix(rows[len(rows)-1][0], "Summary:") {
			t.Errorf("expected summary row, got %v", rows)
		}
	})
}

func tamperCookie(v string) string {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return v
	}
	b := []byte(parts[1])
	if len(b) > 0 {
		if b[0] == 'a' {
			b[0] = 'b'
		} else {
			b[0] = 'a'
		}
	}
	return parts[0] + "." + string(b)
}

// TestExportCSVDateRangeFilter verifies the from/to filter excludes
// out-of-range rows, that a swapped range behaves identically, that >30-day
// ranges are capped to 30 days from the start, and that omitted from/to yields
// a successful response (defaults are relative to HST "today" and not asserted
// exactly).
func TestExportCSVDateRangeFilter(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	hasDate := func(rows [][]string, want string) bool {
		for _, r := range rows[1 : len(rows)-1] {
			if r[3] == want {
				return true
			}
		}
		return false
	}

	t.Run("ordered range", func(t *testing.T) {
		// 2026-08-06..2026-08-12 isolates the 2026-08-10 ev2 record.
		rec := doExport(srv, admin, "?from=2026-08-06&to=2026-08-12&event="+fx.ev2)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		rows := parseCSVBody(t, rec)
		if len(rows) != 3 {
			t.Fatalf("row count = %d, want 3 (header + 1 record + summary)", len(rows))
		}
		if !hasDate(rows, "2026-08-10") {
			t.Errorf("expected 2026-08-10 record present")
		}
		if hasDate(rows, "2026-08-01") || hasDate(rows, "2026-08-05") {
			t.Errorf("out-of-range records should be excluded")
		}
	})

	t.Run("swapped range", func(t *testing.T) {
		rec := doExport(srv, admin, "?from=2026-08-12&to=2026-08-06&event="+fx.ev2)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		rows := parseCSVBody(t, rec)
		if len(rows) != 3 {
			t.Fatalf("row count = %d, want 3", len(rows))
		}
		if !hasDate(rows, "2026-08-10") {
			t.Errorf("swapped range should match ordered range (only 2026-08-10)")
		}
	})

	t.Run("30 day cap", func(t *testing.T) {
		// 2026-07-10..2026-08-31 (52 days) caps to 07-10..08-08, excluding
		// 08-10 but keeping ev2's 08-02 and 08-05 records.
		rec := doExport(srv, admin, "?from=2026-07-10&to=2026-08-31&event="+fx.ev2)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		rows := parseCSVBody(t, rec)
		if hasDate(rows, "2026-08-10") {
			t.Errorf("capped range should exclude 2026-08-10 record")
		}
		if !hasDate(rows, "2026-08-05") || !hasDate(rows, "2026-08-02") {
			t.Errorf("capped range should include records within 30-day window")
		}
	})

	t.Run("defaults", func(t *testing.T) {
		rec := doExport(srv, admin, "?event="+fx.ev2)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
			t.Errorf("Content-Type = %q, want text/csv", ct)
		}
		rows := parseCSVBody(t, rec)
		if len(rows) < 2 || !strings.HasPrefix(rows[len(rows)-1][0], "Summary:") {
			t.Errorf("expected summary row, got %v", rows)
		}
	})
}

// TestExportCSVSiteFilter verifies the legacy ?site query param is ignored:
// the event determines the location, so rows resolve to the event's site
// regardless of the site param.
func TestExportCSVSiteFilter(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rowsFor := func(query string) [][]string {
		t.Helper()
		rec := doExport(srv, admin, query)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		return parseCSVBody(t, rec)
	}

	allSites := func(rows [][]string) map[string]bool {
		m := map[string]bool{}
		for _, r := range rows[1 : len(rows)-1] {
			m[r[1]] = true
		}
		return m
	}

	t.Run("site1 only", func(t *testing.T) {
		// ev1 is at Kona; its records att-1, att-2 resolve to Kona. The date
		// window is pinned so the test is independent of the real clock (the
		// default window is today-13d..today and drifts past att-1's date).
		rows := rowsFor("?site=" + fx.site1 + "&event=" + fx.ev1 + "&from=2026-08-01&to=2026-08-31")
		m := allSites(rows)
		if len(m) != 1 || !m["Kona"] {
			t.Errorf("site1 filter sites = %v, want only Kona", m)
		}
		if len(rows) != 4 { // header + 2 records + summary
			t.Errorf("row count = %d, want 4", len(rows))
		}
	})

	t.Run("site2 only", func(t *testing.T) {
		// ev2 is at Waianae; all its records (att-3, att-4, att-5) resolve to
		// Waianae even though att-5 stored a divergent Kona site.
		rows := rowsFor("?site=" + fx.site2 + "&event=" + fx.ev2 + "&from=2026-08-01&to=2026-08-31")
		m := allSites(rows)
		if len(m) != 1 || !m["Waianae"] {
			t.Errorf("site2 filter sites = %v, want only Waianae", m)
		}
		if len(rows) != 5 { // header + 3 records + summary
			t.Errorf("row count = %d, want 5", len(rows))
		}
	})

	t.Run("event wins over site", func(t *testing.T) {
		// When both site and event are given, the event determines the
		// location: ev2 is at Waianae regardless of the site param.
		rows := rowsFor("?site=" + fx.site1 + "&event=" + fx.ev2 + "&from=2026-08-01&to=2026-08-31")
		m := allSites(rows)
		if len(m) != 1 || !m["Waianae"] {
			t.Errorf("event-wins sites = %v, want only Waianae", m)
		}
		if len(rows) != 5 { // header + 3 records + summary
			t.Errorf("row count = %d, want 5", len(rows))
		}
	})

	t.Run("no site param", func(t *testing.T) {
		// ev2 is at Waianae; omitting site returns all ev2 records (Waianae).
		rows := rowsFor("?event=" + fx.ev2 + "&from=2026-08-01&to=2026-08-31")
		m := allSites(rows)
		if len(m) != 1 || !m["Waianae"] {
			t.Errorf("no site param sites = %v, want only Waianae", m)
		}
		if len(rows) != 5 { // header + 3 records + summary
			t.Errorf("row count = %d, want 5", len(rows))
		}
	})
}

// TestExportCSVEventFilter verifies the ?event filter scopes rows to one
// event, including that no-event rows are excluded when filtering.
func TestExportCSVEventFilter(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	eventsFor := func(query string) map[string]bool {
		t.Helper()
		rec := doExport(srv, admin, query)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		rows := parseCSVBody(t, rec)
		m := map[string]bool{}
		for _, r := range rows[1 : len(rows)-1] {
			m[r[2]] = true
		}
		return m
	}

	t.Run("ev1 only", func(t *testing.T) {
		m := eventsFor("?event=" + fx.ev1)
		if len(m) != 1 || !m["Morning Program"] {
			t.Errorf("ev1 filter events = %v, want only Morning Program", m)
		}
	})

	t.Run("ev2 only", func(t *testing.T) {
		m := eventsFor("?event=" + fx.ev2)
		if len(m) != 1 || !m["Job Fair"] {
			t.Errorf("ev2 filter events = %v, want only Job Fair", m)
		}
	})
}

// TestExportCSVHeaderAndFormatting asserts the full wire output: exact header,
// resolved relation names, title-cased status, verbatim date/note with comma
// round-trip, a trailing summary row, and the response headers.
func TestExportCSVHeaderAndFormatting(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doExport(srv, admin, "?from=2026-08-01&to=2026-08-10&event="+fx.ev1)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment; filename=\"attendance_export_") {
		t.Errorf("Content-Disposition = %q, want attachment", cd)
	}

	rows := parseCSVBody(t, rec)
	wantHeader := []string{"Participant", "Site", "Event", "Date", "Status", "Recorded By", "Check-in Time", "Note"}
	for i, want := range wantHeader {
		if rows[0][i] != want {
			t.Errorf("header[%d] = %q, want %q", i, rows[0][i], want)
		}
	}

	// Locate the Alice 2026-08-01 present record.
	var aliceRow []string
	for _, r := range rows[1 : len(rows)-1] {
		if r[0] == "Alice" && r[3] == "2026-08-01" {
			aliceRow = r
			break
		}
	}
	if aliceRow == nil {
		t.Fatalf("expected Alice 2026-08-01 record")
	}
	if aliceRow[1] != "Kona" {
		t.Errorf("site = %q, want Kona", aliceRow[1])
	}
	if aliceRow[2] != "Morning Program" {
		t.Errorf("event = %q, want Morning Program", aliceRow[2])
	}
	if aliceRow[4] != "Present" {
		t.Errorf("status = %q, want Present", aliceRow[4])
	}
	if aliceRow[5] != "Admin One" {
		t.Errorf("recorded_by = %q, want Admin One", aliceRow[5])
	}
	if aliceRow[6] != formatTime("2026-08-01 20:30:00") {
		t.Errorf("check-in time = %q, want %q", aliceRow[6], formatTime("2026-08-01 20:30:00"))
	}

	// The comma-containing note must round-trip as a single field.
	var commaRow []string
	for _, r := range rows[1 : len(rows)-1] {
		if r[7] == "late, note with, comma" {
			commaRow = r
			break
		}
	}
	if commaRow == nil {
		t.Errorf("expected comma-note record present as single field")
	}

	// Trailing summary row.
	sum := rows[len(rows)-1]
	if !strings.HasPrefix(sum[0], "Summary:") {
		t.Errorf("last row first cell = %q, want Summary prefix", sum[0])
	}
	for i := 1; i < len(sum); i++ {
		if sum[i] != "" {
			t.Errorf("summary col %d = %q, want empty", i, sum[i])
		}
	}
}

// TestExportCSVEmptyResultSet verifies a date range matching no records yields
// a 200 with exactly the header row and an empty summary row.
func TestExportCSVEmptyResultSet(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doExport(srv, admin, "?from=1990-01-01&to=1990-01-31&event="+fx.ev1)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	rows := parseCSVBody(t, rec)
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	if rows[1][0] != "Summary: 0 check-ins, 0 unique participants, 0% avg rate" {
		t.Errorf("summary = %q, want empty summary", rows[1][0])
	}
}

// TestExportCSVRequiresEvent proves that exporting without a selected event is
// rejected with a 400 and the standard event-required message.
func TestExportCSVRequiresEvent(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)
	admin := adminCookie(srv, fx.admin1)

	rec := doExport(srv, admin, "?from=2026-08-01&to=2026-08-10")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (event required)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "an event must be selected before recording attendance") {
		t.Errorf("body missing event-required message")
	}
	if ct := rec.Header().Get("Content-Type"); ct == "text/csv" {
		t.Errorf("must not emit CSV when event is missing")
	}
}

// TestExportCSVNameResolution verifies nameFor handles empty/failed lookups and
// that loadExportRows resolves relation names and raw status against the
// booted PB.
func TestExportCSVNameResolution(t *testing.T) {
	srv := newTestServer(t)
	fx := seedExportData(t, srv.pb)

	if got := srv.nameFor("sites", fx.site1); got != "Kona" {
		t.Errorf("nameFor(sites, site1) = %q, want Kona", got)
	}
	if got := srv.nameFor("sites", ""); got != "" {
		t.Errorf("nameFor(sites, empty) = %q, want empty", got)
	}
	if got := srv.nameFor("sites", "nonexistent"); got != "" {
		t.Errorf("nameFor(sites, nonexistent) = %q, want empty", got)
	}

	rows, err := srv.loadExportRows("", "2026-08-01", "2026-08-05")
	if err != nil {
		t.Fatalf("loadExportRows: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("row count = %d, want 4", len(rows))
	}
	first := rows[0]
	if first.ParticipantName != "Alice" {
		t.Errorf("participant = %q, want Alice", first.ParticipantName)
	}
	if first.SiteName != "Kona" {
		t.Errorf("site = %q, want Kona", first.SiteName)
	}
	if first.EventName != "Morning Program" {
		t.Errorf("event = %q, want Morning Program", first.EventName)
	}
	if first.Status != "present" {
		t.Errorf("status = %q, want raw present", first.Status)
	}
	if first.RecordedByName != "Admin One" {
		t.Errorf("recorded_by = %q, want Admin One", first.RecordedByName)
	}
	if first.CheckInTime != formatTime("2026-08-01 20:30:00") {
		t.Errorf("check-in time = %q, want %q", first.CheckInTime, formatTime("2026-08-01 20:30:00"))
	}
}

// TestExportRowsEventScoping proves loadExportRows with no event returns all
// in-range records, while a specific event scopes to its own records.
func TestExportRowsEventScoping(t *testing.T) {
	srv := newTestServer(t)
	fx := seedRosterData(t, srv.pb)

	// Seed attendance so the queries have rows to return.
	saveAttendance(t, srv.pb, fx.iInSite1, fx.site, fx.ev1, "2026-08-13", "present")
	saveAttendance(t, srv.pb, fx.iInSite2, fx.site, fx.ev2, "2026-08-13", "present")
	saveAttendance(t, srv.pb, fx.iOtherSite, fx.site2, fx.ev1, "2026-08-13", "walk_in")

	// No event (nil event set): all in-range records.
	allRows, err := srv.loadExportRows("", "2026-08-01", "2026-08-31")
	if err != nil {
		t.Fatalf("loadExportRows(all): %v", err)
	}
	if len(allRows) != 3 {
		t.Errorf("all-rows export = %d, want 3 (nil event set must not restrict)", len(allRows))
	}

	// Specific event: only its records.
	ev1Rows, err := srv.loadExportRows(fx.ev1, "2026-08-01", "2026-08-31")
	if err != nil {
		t.Fatalf("loadExportRows(ev1): %v", err)
	}
	if len(ev1Rows) != 2 {
		t.Errorf("ev1 export rows = %d, want 2", len(ev1Rows))
	}
}
