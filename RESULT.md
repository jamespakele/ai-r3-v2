# RESULT — Require event selection before recording attendance

## What was built
Server-side enforcement that an event must be selected before any attendance
record can be written, plus event-scoped write association. Implemented via the
omp-plan-execute pipeline (MOA plan → omp).

## Changes
- `r3-intake/internal/server/attendance.go`
  - New shared `requireEventID(w, eventID)` gate → 400 "an event must be
    selected before recording attendance" when event_id is empty.
  - `handleToggle`: gate added; idempotency filter now always includes `event`;
    `event` set on both create and update branches.
  - `handleWalkin`: now reads `event_id`; gate added; filter includes `event`;
    `event` set on both branches.
- `r3-intake/internal/server/person_attendance.go`
  - `handlePersonAttendanceDaySave`: gate added; filter includes `event`;
    `event` set on both branches.
  - `PersonDayDetailView`: added `Events []Event` + `SelectedEventID string`.
  - `buildPersonDayDetailView`: loads events, sets SelectedEventID from record.
- `r3-intake/internal/assets/public/index.html`
  - `person-attendance-day`: required `event_id` selector added to both create
    and edit forms (before Status); removed read-only EventName display.
- Tests: `TestRequireEventID`, `TestToggleRequiresEvent`, `TestToggleStoresEvent`,
  `TestWalkinRequiresEvent`, `TestToggleScopesPerEvent` (cross-event
  non-collision), `TestPersonAttendanceDayRequiresEvent`,
  `TestPersonAttendanceDayStoresEvent`, day-detail selector assertions.

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (internal/server ok, 5.5s)

## AC coverage
- AC #3 (attendance cannot be recorded until an event is selected): enforced
  server-side on all three write paths (toggle/walkin/person-day-save) — cannot
  be bypassed by a crafted POST.
- AC #4 (records associated with the event): every write sets `event`; uniqueness
  keyed on `(event, intake, date)`.
- AC #1/#2/#5 owned by parent card t_f9a235ad (matrix scoping + migration 010).

## Artifacts
- Plan: `docs/plans/omp-plan-require-event-selection.md`
- Commit: f47b25c on wt/t_30ac2f54
