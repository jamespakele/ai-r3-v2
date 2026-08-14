# Epic 9: Attendance should be per event

**Status:** COMPLETE — all child story branches merged into `epic/9-attendance-should-be-per-event`.

## Stories implemented

- 9.1 Schema + data migration — `wt/t_f9a235ad` — `attendance.event` becomes required; null-event records backfilled into per-site "Legacy / Unassigned" events via migration `010`.
- 9.2 Matrix roster independence — `wt/t_a1402c12` — `loadMatrixRows` always returns the full site/role-scoped roster; the selected event only scopes attendance records, not participant list.
- 9.3 Require event before recording — `wt/t_30ac2f54` — every attendance write path now returns 400 unless an event is selected.
- 9.4 Design plan — `wt/t_b8758540` — planning artifact captured in `docs/plans/omp-plan-event-scoped-attendance.md` (design history preserved).

## Files changed

- `r3-intake/pocketbase/migrations/010_attendance_event_required.go` (new) — backfills null-event attendance records into per-site "Legacy / Unassigned" events, then flips `attendance.event` to required. Idempotent; down is best-effort (keeps legacy events to avoid cascade-deleting attendance).
- `r3-intake/pocketbase/migrations/002_encryption.go` — registers migration 010.
- `r3-intake/internal/server/attendance.go` — `requireEventID` gate on `handleToggle`/`handleWalkin`/`handleExportCSV`; full-key `(event, intake, date)` filters and always-set `event` on writes; `loadMatrixRows` always loads the full site/role-scoped roster (event only scopes the attendance map); `MatrixViewData.EventRequired` flag.
- `r3-intake/internal/server/person_attendance.go` — event selector state (`PersonAttendanceView.EventID/Events/EventRequired`, `PersonDayDetailView.EventID/Events`); event-scoped calendar queries, day-detail reads, day-save upserts, and day-delete; `requireEventID` gate on day-save.
- `r3-intake/internal/assets/public/index.html` — matrix: "Select an event…" dropdown label, event-required banner, walk-in panel hidden without an event, disabled-dot aria-label "Attendance unavailable"; per-person calendar: event selector + event-preserving prev/next nav + click gating without an event; day-detail forms: required `event_id` selector (create + edit), hidden `event_id` on delete.
- Tests: `attendance_test.go` (`TestRequireEventID`, `TestMatrixContentRenderEventRequired`), `attendance_toggle_integration_test.go` (event-required/store/scoped tests), `attendance_roster_integration_test.go` (new, `TestMatrixRosterEventIndependent`), `person_attendance_integration_test.go` (event-scoped day save/delete/validation + `TestPersonAttendanceDaySaveRequiresEvent`, `TestPersonAttendanceDayRequiresEvent`, `TestPersonAttendanceDayStoresEvent`), `person_attendance_test.go` (event selector render assertions), `attendance_export_integration_test.go` (event-required export, event-scoped filters, `TestExportCSVRequiresEvent`).
- Plans: `docs/plans/omp-plan-event-scoped-attendance.md`, `docs/plans/omp-plan-scope-attendance-matrix-by-event.md`, `docs/plans/omp-plan-require-event-selection.md`, `WORKING_PLAN_require_event_selection.md`.

## Merge resolution notes

- `wt/t_f9a235ad` and `wt/t_a1402c12` merged cleanly (additive); `wt/t_30ac2f54` overlapped handlers/templates/tests and was resolved to the superset: event-scoping from f9a/a140 + `requireEventID` gate from t_30ac2f54 + corrected `event_id` form field naming (visible required selector, not hidden input).
- `docs/plans/omp-plan-event-scoped-attendance.md` had an add/add conflict between the design-only plan (t_b8758540) and the implementation plan (f9a/a140). Kept the implementation version; appended a "Design history" section with t_b8758540's pre-change state, backfill decision, and target schema notes.
- `RESULT.md` was replaced by each story branch; synthesized into this epic-level document.

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass
- Conflict-marker sweep — none
