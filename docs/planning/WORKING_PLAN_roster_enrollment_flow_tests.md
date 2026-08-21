# Working Plan: Add tests for roster rendering and enrollment flow

## Objective

Add white-box `package server` integration tests that exercise the event roster
rendering and the enrollment / unenroll / search HTTP flow against a real
in-process PocketBase. The tests drive the actual routes and handlers
(`POST /admin/events/{id}/enroll`, `POST /admin/events/{id}/unenroll`,
`GET /admin/events/{id}/enroll-search`) through `srv.Mux().ServeHTTP`, verify
the rendered `event-roster` / `enroll-search-results` fragments, and assert on
the resulting `event_enrollment` records in PocketBase.

This card is **test-only**: it must not modify any production code
(`admin.go`, `server.go`, `auth.go`, templates). It fills the integration-test
gaps that the existing pure-render tests (`TestAdminEventsRender`,
`TestEnrollSearchResultsRender`) and pure-math test
(`TestEnrollmentStatsCompute`) do not cover — namely the HTTP round-trip,
idempotency, soft-delete, auth boundary, and no-JS fallback behavior.

## Constraints

- **Tests only.** Do not touch `admin.go`, `server.go`, `auth.go`, or any
  template. Only add a new test file (and, if needed, a local helper inside it).
- **Must pass in THIS worktree** (branch `wt/t_d6f46781`), which still has the
  UNFIXED `deleted=false` filter in `loadEnrolledRoster` (admin.go ~L530) and
  the site-restriction clause in `handleEnrollSearch` (admin.go ~L750). The
  parent epic will merge sibling worktrees later; sibling cards already fixed
  these. Therefore:
  - **Always seed enrollments with an explicit `Set("deleted", false)`.** The
    shared `saveEnrollment` helper (attendance_roster_integration_test.go ~L100)
    does NOT set `deleted`, and PocketBase v0.39 `BoolField` with
    `Required:false` defaults to NULL, so the `deleted=false` filter would
    exclude such records. Add a local helper in the new file that sets
    `deleted=false` (do not modify the shared file).
  - **For search tests, assert only same-site results.** The site-restriction
    clause is present in this worktree and removed by the parent; asserting
    cross-site behavior would be flaky across the merge. Same-site search
    results behave identically in both states.
- **Reuse the existing harness** — do not re-boot PocketBase or re-seed from
  scratch:
  - `newTestServer(t)` (attendance_export_integration_test.go ~L25) → `*Server`
  - `adminCookie(srv, id)` / `cmCookie(srv, id)` (attendance_export_integration_test.go ~L203)
  - `seedRosterData(t, srv.pb)` (attendance_roster_integration_test.go ~L20) →
    `rosterFixtures{site, site2, ev1, ev2, cm, iInSite1, iInSite2, iOtherSite, iAssignedCM}`
  - `saveAttendance(t, pb, intake, site, event, date, status)` (attendance_roster_integration_test.go ~L100)
  - `addCSRFToRequest(req)` (csrf_test.go ~L14)
  - `findAttendance(t, srv, intakeID, date)` (person_attendance_integration_test.go ~L192)
- **HST timezone:** `handleEventEnroll` sets `enrolled_date` to
  `time.Now().In(hst).Format("2006-01-02")`. Assert structural properties
  (presence, format `YYYY-MM-DD`), not exact dates.
- **Follow existing style:** white-box `package server`, table-driven where
  sensible, `t.Helper()` in helpers, descriptive test names, `t.Fatalf` with
  context messages.
- **Do not duplicate** existing coverage: `TestAdminEventsRender`,
  `TestEnrollSearchResultsRender`, `TestEnrollmentStatsCompute`,
  `TestMatrixRosterEventIndependent`.

## File Structure

- **New file:** `r3-intake/internal/server/event_enrollment_flow_test.go`
  (package `server`). Contains all new tests plus a local helper
  `saveActiveEnrollment(t, pb, event, intake)` that creates an
  `event_enrollment` record with `deleted=false` (mirrors `saveEnrollment` but
  sets `deleted` explicitly).
- **No other files created or modified.** Existing test files are left
  untouched.

## Implementation Notes

### Local helper (in the new file)

```go
// saveActiveEnrollment records one event_enrollment row with deleted=false.
// The shared saveEnrollment helper does NOT set deleted, and the roster filter
// (deleted=false) would exclude a NULL deleted value, so tests must set it
// explicitly. This also stays valid after the parent merges the
// (deleted=false || deleted=null) filter fix.
func saveActiveEnrollment(t *testing.T, pb *pocketbase.PocketBase, event, intake string) {
    t.Helper()
    col, err := pb.FindCollectionByNameOrId("event_enrollment")
    if err != nil { t.Fatalf("event_enrollment collection: %v", err) }
    r := core.NewRecord(col)
    r.Set("event", event)
    r.Set("intake", intake)
    r.Set("deleted", false)
    if err := pb.Save(r); err != nil { t.Fatalf("save enrollment: %v", err) }
}
```

### Request helper (in the new file)

```go
// doEnrollPost POSTs to the given path with HTMX header, CSRF, and cookie.
// hx=true sets HX-Request: true (fragment render); hx=false omits it (no-JS
// 303 fallback).
func doEnrollPost(srv *Server, cookie *http.Cookie, path string, form url.Values, hx bool) *httptest.ResponseRecorder {
    req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    if hx { req.Header.Set("HX-Request", "true") }
    addCSRFToRequest(req)
    if cookie != nil { req.AddCookie(cookie) }
    rec := httptest.NewRecorder()
    srv.Mux().ServeHTTP(rec, req)
    return rec
}
```

A parallel `doEnrollSearch(srv, cookie, path)` GET helper (with cookie + CSRF)
is used for the search tests.

### Counting enrollment records

Add a local helper `countEnrollments(t, srv, eventID)` that queries
`event_enrollment` filtered by `event='<id>'` (no `deleted` clause) and returns
`len(recs)`. Use it to assert record counts (idempotency, soft-delete
preservation) independent of the roster filter.

### Tests to add

1. **`TestEnrollFlowEndToEnd`** — `seedRosterData`; admin cookie. POST
   `/admin/events/{ev1}/enroll` with `intake_id=iInSite1`, `HX-Request: true`.
   Assert: status 200; body contains `Alice` (the intake name) and the
   `event-roster` table; `EnrolledCount` incremented (assert the rendered
   fragment shows the participant once). Then verify a single
   `event_enrollment` record exists for `(ev1, iInSite1)` with `deleted=false`
   via `countEnrollments` and a direct record read.

2. **`TestEnrollIdempotent`** — POST enroll for `(ev1, iInSite1)` twice.
   Assert: both return 200; `countEnrollments(t, srv, ev1) == 1`; the roster
   fragment contains `Alice` exactly once (e.g. `strings.Count(body, "Alice")`
   — note the name may also appear in the search fragment only if that was
   rendered; here only the roster fragment is rendered, so a single occurrence
   is expected; prefer asserting the row count via the `enroll-result`/roster
   markup or `strings.Count` of the participant name).

3. **`TestUnenrollSoftDeletes`** — seed an active enrollment via
   `saveActiveEnrollment(t, srv.pb, ev1, iInSite1)`. POST
   `/admin/events/{ev1}/unenroll` with `intake_id=iInSite1`, `HX-Request: true`.
   Assert: status 200; roster fragment shows the empty state
   (`No participants enrolled yet`); `countEnrollments(t, srv, ev1) == 1`
   (record still exists); the record's `deleted` field is now `true` (read it
   back directly). Assert `EnrolledCount` decremented (fragment no longer lists
   the participant).

4. **`TestEnrollSearch`** — `seedRosterData`; admin cookie. GET
   `/admin/events/{ev1}/enroll-search?name=Al` → 200, body contains `Alice`
   (same-site intake) and `+ Enroll`. GET `?name=Bo` → contains `Bob`. GET
   `?name=Ca` → `Carol` is in `site2`, NOT the event's site; in THIS worktree
   the site-restriction clause excludes her, so assert only that the result
   does NOT include `Carol` (do NOT assert cross-site inclusion — that is the
   parent's fix). GET `?name=A` (1 char) → 200, empty body (min 2 chars).
   GET `?name=zz` → 200, body contains `No matching participants`.

5. **`TestEnrollSearchMarksAlreadyEnrolled`** — seed an active enrollment for
   `(ev1, iInSite1)`. GET `?name=Al` → body contains `Alice` and `Already
   enrolled` (disabled button), and does NOT contain `+ Enroll` for that row.

6. **`TestRosterRenderingWithStats`** — seed an active enrollment for
   `(ev1, iInSite1)`; seed attendance via `saveAttendance`:
   `(iInSite1, site, ev1, "2026-08-13", "present")` and
   `(iInSite1, site, ev1, "2026-08-14", "walk_in")`. GET
   `/admin/events/{ev1}/manage` with admin cookie → 200; assert the roster
   fragment shows `Alice`, `2 / <totalDays>` (Days Attended / Total Days), a
   `%` rate badge, and `2026-08-14` as Last Present. Compute expected
   `totalDays` via `daysInRange(start, end, today)` (the same function the
   handler uses) rather than hardcoding, since `today` is HST-relative. Assert
   the rate badge class (`rate-good`/`rate-low`) is present.

7. **`TestEnrollAuthBoundary`** — table-driven:
   - case_manager cookie POST enroll → 303 redirect to `/login` (requireRole).
   - case_manager cookie POST unenroll → 303 to `/login`.
   - case_manager cookie GET enroll-search → 303 to `/login`.
   - no cookie POST enroll → 303 to `/login`.
   Assert `rec.Code == http.StatusSeeOther` and `Location` header ends with
   `/login`. Verify no `event_enrollment` record was created in the
   case_manager enroll case.

8. **`TestEnrollNoJSFallback`** — admin cookie, POST enroll WITHOUT
   `HX-Request` header → 303 redirect to `/admin/events/{ev1}/manage`. Assert
   `rec.Code == http.StatusSeeOther` and `Location` equals
   `/admin/events/{ev1}/manage`. The enrollment record is still created.

### Assertion style

- Use `strings.Contains` on `rec.Body.String()` for fragment content, matching
  the existing render tests.
- For redirects, check `rec.Code` and `rec.Header().Get("Location")`.
- For record-level assertions, read records back via `srv.pb` (e.g.
  `FindRecordsByFilter` on `event_enrollment`) and check the `deleted` field
  with `rec.GetBool("deleted")`.
- Keep date assertions structural: assert `enrolled_date` matches
  `^\d{4}-\d{2}-\d{2}$` (or just that it is non-empty), never an exact date.

## Verification Criteria

- `cd r3-intake && go build ./...` passes.
- `cd r3-intake && go vet ./...` passes.
- `cd r3-intake && go test ./...` passes — including the new
  `TestEnrollFlowEndToEnd`, `TestEnrollIdempotent`, `TestUnenrollSoftDeletes`,
  `TestEnrollSearch`, `TestEnrollSearchMarksAlreadyEnrolled`,
  `TestRosterRenderingWithStats`, `TestEnrollAuthBoundary`,
  `TestEnrollNoJSFallback`.
- Run the new tests specifically:
  `cd r3-intake && go test ./internal/server/ -run 'TestEnroll|TestUnenroll|TestRosterRenderingWithStats' -v`.
- Confirm no production files changed: `git status --short` shows only the new
  test file (and this plan file).
- The tests pass in THIS worktree (with the unfixed `deleted=false` filter and
  site-restricted search) and are written so they remain valid after the parent
  merges the sibling fixes (explicit `deleted=false` seeding; no cross-site
  search assertions).
