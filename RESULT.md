# RESULT — Epic 18: Fix Event / Location / Attendance Schema Relationships

## What was built

This epic delivered the design, schema migration, and filtering logic that make event location derive from the event only.

### 1. Design doc

`docs/plans/omp-plan-event-location-attendance-relationships.md` defines:

- `events.site` is the single source of truth for location.
- `attendance.site` is redundant and must be removed.
- `intake.site` stays as the participant's home site (roster scoping only).

### 2. Schema migration

New Go migration `015_attendance_remove_site.go` (registered in `r3-intake/pocketbase/migrations/migrations.go`) removes the redundant `attendance.site` relation field.

- **Up** (`upAttendanceRemoveSite`): idempotent; logs a `WARN` (does not fail) if a row's stored `site` diverges from its event's `site`; then calls `Fields.RemoveByName("site")` and `app.Save`.
- **Down** (`downAttendanceRemoveSite`): idempotent; re-adds `site` as an optional single-select relation to `sites` and backfills each row's `site` from its event's `site`.
- Unique index `idx_attendance_event_intake_date` on `(event, intake, date)` is unchanged — it does not reference `site`.

### 3. Event-derived location filtering

Updated Go read/write paths so location is always derived from the event:

- `r3-intake/internal/server/attendance.go`:
  - Added `resolveEventIDs(eventID, siteID)` helper: specific event wins; otherwise active events at the site (empty slice when none); `nil` means no restriction (admin, all locations).
  - `loadMatrixRows`: attendance filter is date range plus `event='id1' || event='id2' …`; empty event set skips the query; removed the `(site='' || site='%s')` clause. Roster (`intakeFilter`) unchanged.
  - `loadExportRows`: same event-set resolution; empty set returns empty rows; removed `site='%s'` clause; `SiteName` resolved from the row's event's site via `FindRecordById(eventsCol.Id, event)`.
  - `handleToggle` insert and all three `handleWalkin` write paths no longer call `Set("site", siteID)`. `siteID` is still used for the `Disabled` flag and redirect query string.
- `r3-intake/internal/server/person_attendance.go`: day-detail insert no longer calls `rec.Set("site", intake.GetString("site"))`.

### 4. Tests

- Updated existing tests that asserted on `attendance.site` to assert an empty stored site / event-derived location.
- Added:
  - `TestMatrixSitedFilteringAllEvents`
  - `TestMatrixSiteNoActiveEvents`
  - `TestAttendanceSchemaNoSiteField`
  - `TestExportSiteNoActiveEvents`
  - `TestWalkinStoresNoSite`

## Files changed

- `docs/plans/omp-plan-event-location-attendance-relationships.md` (new)
- `docs/plans/omp-plan-015-attendance-remove-site.md` (new)
- `docs/plans/omp-plan-event-derived-location-filtering.md` (new)
- `docs/plans/omp-plan-attendance-filtering-integrity-tests.md` (new)
- `WORKING_PLAN_event_derived_location_filtering.md` (new)
- `.hermes/plans/working-plan-attendance-filtering-tests.md` (new)
- `r3-intake/pocketbase/migrations/015_attendance_remove_site.go` (new)
- `r3-intake/pocketbase/migrations/migrations.go` (register 015)
- `r3-intake/internal/server/attendance.go`
- `r3-intake/internal/server/person_attendance.go`
- `r3-intake/internal/server/attendance_export_integration_test.go`
- `r3-intake/internal/server/attendance_roster_integration_test.go`
- `r3-intake/internal/server/attendance_toggle_integration_test.go`
- `r3-intake/internal/server/person_attendance_integration_test.go`

## Verification

- `make build` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (internal/server and migrations)

## Scope note

No participant-to-location link remains. `intake.site` is home-site roster scoping only.
