# RESULT.md — Scope attendance matrix by selected event

## What was built

Fixed the attendance matrix so the **participant roster is identical regardless
of which event is selected** (epic AC #1). The selected event now scopes only the
attendance records shown/edited, never the participant list.

## Root cause

`loadMatrixRows` (attendance.go) had a divergence: when an event was selected it
loaded only enrolled participants + walk-ins for that event; with no event it
loaded the full site/role-scoped roster. This violated AC #1 ("Participant list
renders identically regardless of selected event").

## Changes

- **r3-intake/internal/server/attendance.go** — `loadMatrixRows` now always
  loads the full site/role-scoped roster (case_manager -> `assigned_to`, site ->
  `site`, else `1=1`). Removed the event_enrollment + walk-in union branch. The
  attendance map still scopes by event via `attFilter`'s `&& event='%s'`.
- **r3-intake/internal/assets/public/index.html** — disabled-dot aria-label
  generalized from "Attendance requires a location" to "Attendance unavailable"
  (cells are now disabled for missing location OR missing event).
- **r3-intake/internal/server/attendance_roster_integration_test.go** (new) —
  `TestMatrixRosterEventIndependent` seeds 2 sites, 2 events, admin + case
  manager, 4 intakes, an enrollment, and attendance records (in-site ev1/ev2,
  out-of-site walk-in ev1). Asserts identical ordered roster with/without event,
  out-of-site walk-in never rendered as a row, and event-scoped map population.
- **docs/plans/omp-plan-scope-attendance-matrix-by-event.md** — working plan
  artifact.

## Walk-in behavior

Walk-in-created intakes set `site=siteID`, so they appear in the full roster
naturally. Out-of-scope walk-ins are still recorded in the attendance map but
not rendered as rows — correct for a site/role-scoped roster and keeps the
roster deterministic (no event-dependent divergence).

## Verification

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (internal/server ok, 5.4s)
- `TestMatrixRosterEventIndependent` — PASS (0.17s)
- Existing `TestMatrixContentRender*`, `TestToggle*`, person-attendance, and
  export tests all pass.

## Commit

`45bb720` on `wt/t_a1402c12` (child story branch; parent epic merges).
