# RESULT — Replace union filter with strict attendance-based filter in admin.go

## What was built
Replaced the union event filter in `handleList` (r3-intake/internal/server/admin.go)
with a strict attendance-only filter:

- When `?event=<id>` is set, the filter part is now ONLY the attendance-derived
  intake IDs: `(id='<id1>' || id='<id2>' || ...)`. The `event='<id>'` home-event
  branch was removed — an intake's home event no longer contributes.
- The attendance query now constrains `date` to the selected event's
  `start_date`..`end_date` range (mirrors the matrix's auto-scoping in
  `parseMatrixFilters`). The event record is loaded via
  `s.pb.FindRecordById("events", eventFilter)`; if the event can't be loaded or
  dates are invalid, the filter degrades to `id=''` (no matches).
- If no in-range attendance records exist, the filter returns no intakes
  (no home-event fallback).

## Files changed
- r3-intake/internal/server/admin.go — strict attendance-only filter + date-range scoping
- r3-intake/internal/server/records_list_attendance_join_integration_test.go — updated
  expectations to strict semantics; added TestListEventFilterConstrainsByDateRange
- r3-intake/internal/server/records_list_integration_test.go — updated subtests to
  strict semantics (no home-event fallback)
- r3-intake/README.md — updated Records event-filter docs to strict attendance-only
- docs/plans/omp-plan-strict-attendance-filter.md — working plan artifact

## Verification
- go build ./... — PASS
- go vet ./... — PASS
- go test ./internal/server/ -run 'TestListEventFilter|TestListNoEventFilter' -count=1 — PASS (8 tests, incl. new date-range test)
- go test ./... -count=1 — PASS except TestExportCSVEventFilter/ev1_only, which
  fails IDENTICALLY at baseline HEAD (verified via git stash): pre-existing
  time-dependent failure (subtest omits from/to, so the default 14-day export
  window excludes ev1's seed dates). Not a regression from this change.

## Acceptance criteria
- Filtering by an event returns only intakes with in-range attendance records.
- Attendance is the source of truth; home-event matching removed.
- Date-range scoping mirrors the matrix auto-scoping.
