# Working Plan: Tests for Consistent Filtering and Data Integrity (R3 Intake Attendance)

## Objective

Add Go integration tests to the existing `r3-intake/internal/server/` test suite that prove the epic's data-model rules hold under the new filtering logic already implemented in `attendance.go`/`person_attendance.go` (uncommitted in this worktree) and the already-committed `015_attendance_remove_site.go` migration (HEAD `6cfc8fe`). The tests must close the coverage gaps in **site-scoped filtering without a specific event**, **empty event-set behavior**, **post-015 schema/write-path integrity**, and the **walk-in write path** — without duplicating assertions the sibling already added. **No source files are modified — tests only.**

## Constraints

- **Go toolchain**: `go build ./... && go vet ./... && go test ./...` must pass from `r3-intake/`.
- **PocketBase v0.39** embedded; JS migrations auto-registered via `jsvm` + `pbmigrations`; migration 015 already committed at HEAD `6cfc8fe`.
- **Timezone**: attendance "today" is derived from `hst` (`time.FixedZone("HST", -10*60*60)` in `admin.go`); walk-in tests must use `time.Now().In(hst).Format("2006-01-02")`.
- **Existing helpers (reuse, don't reinvent)**:
  - `newTestServer(t)` — boots a real in-process PocketBase with all migrations (export test file).
  - `seedRosterData(t, pb) → rosterFixtures{site, site2, ev1, ev2, cm, iInSite1, iInSite2, iOtherSite, iAssignedCM}`; `saveAttendance`, `saveEnrollment`, `cellStatus`, `equalStrings`.
  - `seedExportData(t, pb) → exportFixtures{site1, site2, ev1, ev2, admin1, cm1, i1, i2}`; `adminCookie`, `cmCookie`, `doExport`, `parseCSVBody`.
  - `seedToggleData(t, pb) → toggleFixtures{site, ev, admin1, iNoSite, iLocated}`; `doToggle`.
  - `findAttendance(t, srv, intakeID, date)`, `countAttendance` (person_attendance file); `doPersonAttendance`; `addCSRFToRequest`.
- **Under-test functions**: `loadMatrixRows(u, siteID, dates, eventID, to)`, `loadExportRows(siteID, eventID, from, to)`, `resolveEventIDs(eventID, siteID)`, `handleWalkin`, `handleToggle`, `handlePersonAttendanceDaySave`.
- **Key semantics to assert**:
  - `resolveEventIDs("", siteID)` returns the active-event IDs at that site (empty slice when the site has no active events; `nil` when `siteID==""`).
  - `loadMatrixRows` with empty event set skips the attendance query entirely (`eventIDs != nil && len > 0` guard), so `attMap` stays empty but the roster still renders.
  - `loadExportRows` early-returns `[]ExportRow{}` when the event set is empty.
  - Location is always derived from the **event's site**, never from a stored `attendance.site`.

## File Structure

All work is in `r3-intake/internal/server/`; **no new files, no source edits**.

| File (modify) | Test function to add | Covers |
|---|---|---|
| `attendance_roster_integration_test.go` | `TestMatrixSitedFilteringAllEvents` | Epic item 1 |
| `attendance_roster_integration_test.go` | `TestMatrixSiteNoActiveEvents` | Epic item 2 (matrix) |
| `attendance_roster_integration_test.go` | `TestAttendanceSchemaNoSiteField` | Epic item 3 (schema) |
| `attendance_export_integration_test.go` | `TestExportSiteNoActiveEvents` | Epic item 2 (export) |
| `attendance_toggle_integration_test.go` | `TestWalkinStoresNoSite` | Epic item 5 |

**Explicitly NOT added** (already covered by sibling — verify, do not duplicate):
- Epic item 4 (divergent legacy `site` resolves to event's site in `loadExportRows` `SiteName`): already proven by `TestExportCSVSiteFilter` "site2 only" subtest — att-5 stores a divergent Kona site yet resolves to Waianae via ev2. Also reinforced by the "event wins over site" subtest. Skip.
- Epic item 3 per-write-path `site==""` assertions for **toggle** and **day-save**: already in `TestToggleLocated` and `TestPersonAttendanceDaySaveCreate`. Only the **schema-level** assertion (item 3a) and the **walk-in** write path (item 5) are gaps.

## Implementation Notes

**1. `TestMatrixSitedFilteringAllEvents`** (in roster test file; reuse `seedExportData`).
- Seed with `seedExportData(t, srv.pb)` → site1=Kona (ev1, with att-1/i1 08-01 present, att-2/i1 08-02 walk_in) and ev2=Waianae (att-5: i1 attending ev2 on 08-10 — a site1 intake at a Waianae event). i2 is a Waianae intake.
- Call `rows, err := srv.loadMatrixRows(admin, fx.site1, []string{"2026-08-01","2026-08-02","2026-08-10"}, "", "2026-08-10")` (admin user, siteID=site1, **empty eventID** — the uncovered case).
- Assert:
  - `cellStatus(rows, i1, "2026-08-01") == "present"` (att-1 via ev1).
  - `cellStatus(rows, i1, "2026-08-02") == "walk_in"` (att-2 via ev1).
  - `cellStatus(rows, i1, "2026-08-10") == ""` — **cross-site exclusion**: att-5 is under ev2/Waianae, so it must NOT surface in the site1-scoped (no-event) matrix. This is the exact gap in the existing roster test, which only checked same-site events.
  - `i2` is **not** present in the roster (Waianae intake excluded from Kona site-scope).
- Note: `loadMatrixRows` uses the `to` param only for the dropout threshold, so any end date in-range is fine.

**2. `TestMatrixSiteNoActiveEvents`** (reuse `seedRosterData`).
- `seedRosterData` gives site2 (Hilo) with **no** active events (ev1/ev2 both at site/Kona); `iOtherSite` is the site2 participant.
- Call `rows, err := srv.loadMatrixRows(admin, fx.site2, []string{"2026-08-13"}, "", "2026-08-13")`.
- Assert:
  - `err == nil`; roster still renders: `iOtherSite` appears as a row (not empty).
  - `cellStatus(rows, iOtherSite, "2026-08-13") == ""` (empty event set → attendance query skipped → no cells filled).
- Edge case this locks in: `resolveEventIDs("", site2)` returns an empty slice (not nil), and `loadMatrixRows` treats that as "skip query", distinct from the admin `siteID==""` nil path.

**3. `TestAttendanceSchemaNoSiteField`** (reuse `newTestServer` only).
- After `srv := newTestServer(t)` (all migrations, incl. 015, applied): `attCol, _ := srv.pb.FindCollectionByNameOrId("attendance")`.
- Assert `attCol.Fields.GetByName("site") == nil`. This is the direct post-015 schema proof (item 3a) not yet covered anywhere.

**4. `TestExportSiteNoActiveEvents`** (in export test file; reuse `seedRosterData`, call `loadExportRows` directly).
- Seed `seedRosterData` → site2/Hilo has no active events.
- Call `rows, err := srv.loadExportRows(fx.site2, "", "2026-08-01", "2026-08-31")`.
- Assert `err == nil` and `len(rows) == 0` (early-return empty result when event set is empty).
- Optional but cheap: also assert `loadExportRows("", "", from, to)` (admin, all sites) still returns rows — proving the nil-vs-empty distinction is what gates the early return. (Attendances seeded in roster: iInSite1/ev1, iInSite2/ev2, iOtherSite/ev1.)

**5. `TestWalkinStoresNoSite`** (in toggle test file; reuse `seedToggleData`, `findAttendance`).
- `today := time.Now().In(hst).Format("2006-01-02")` — walk-in always records the current HST day.
- POST `/attendance/walkin` with `site_id=fx.site`, `event_id=fx.ev`, and `intake_id=fx.iLocated` (existing located intake avoids name-creation ambiguity). Use the same request pattern as `TestWalkinRequiresEvent` (form body + CSRF + admin cookie) — or add a small `doWalkin` helper local to the test file following `doToggle`.
- Assert `rec.Code == http.StatusSeeOther` (303 redirect).
- `att := findAttendance(t, srv, fx.iLocated, today)`; assert non-nil, then:
  - `att.GetString("site") == ""` — walk-in write path stores **no** site (the sibling removed `Set("site")`).
  - `att.GetString("event") == fx.ev` and `att.GetString("status") == "walk_in"`.
  - Location resolves via the event: call `srv.loadExportRows("", fx.ev, today, today)` and assert the single row's `SiteName == "Kona"` (fx.site name), proving the location comes from the event's site, not the row.

**Reused fixture caveats / edge cases**
- `seedExportData` attendance rows already carry a `site` via `save` — after migration 015 the field is gone from the schema, so `rec.Set("site", ...)` is a no-op; do not rely on stored site values in fixtures. All location assertions must go through event resolution.
- `findAttendance`/`countAttendance` filter on `intake + date` only — fine for the single-event fixtures used; for the day-save already-covered assertions they remain unaffected.
- Do not modify `saveAttendance`, `seedExportData`, or any existing test — only append new `Test*` functions and (optionally) one `doWalkin` helper.

## Verification Criteria

From `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_9b0a18f0/r3-intake/`:

```bash
go build ./...
go vet ./...
go test ./internal/server/
```

- All existing tests plus the five new tests must pass (baseline currently passes).
- Confirm the five new `Test*` functions are exercised: `go test ./internal/server/ -run 'TestMatrixSitedFilteringAllEvents|TestMatrixSiteNoActiveEvents|TestAttendanceSchemaNoSiteField|TestExportSiteNoActiveEvents|TestWalkinStoresNoSite' -v`.
- Confirm **no source files were modified**: `git status --short` must show only the pre-existing sibling modifications to `attendance.go`, `person_attendance.go`, and the four test files (plus the new additions in the test files), with no new files and no changes to `*.go` production sources.
