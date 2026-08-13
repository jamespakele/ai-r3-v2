# RESULT — Story 2.3: Event Lifecycle and Status Transitions

## What was built
Implemented backend logic and UI controls for event status transitions
(active → completed / cancelled), per FR13 and UX-DR8, with server-side
validation that rejects invalid state transitions.

- **Status transition handler** `handleEventStatus` (POST /admin/events/{id}/status,
  admin-only): only `active → completed|cancelled` is legal. Terminal states
  (completed/cancelled) reject all transitions. Target status is whitelisted;
  unknown/empty/same-state values are rejected. Non-admin is double-guarded
  (route `requireRole("admin")` + in-handler `u.Role != "admin"` → redirect to `/`).
- **`validEventTransition(from, to)`** helper — single source of truth for the
  lifecycle state machine.
- **`renderEventManage`** shared render helper (refactored from the GET-only
  `handleAdminEventManage`) so both the GET page and the error path reuse it.
- **Event manage screen** rewritten: status badge + enrolled count, Complete /
  Cancel buttons for `active` events, read-only notice for terminal states,
  JS confirm prompt "Mark this event as cancelled?" on Cancel, error display.
- **Report button** appears in the events list only for `completed` events,
  linking to a new `event-report` placeholder stub (CSV export ships in Epic 3).
- **CSS**: `.event-manage-actions`, `.event-readonly` (badge variants already existed).

## Files changed
- `r3-intake/internal/server/admin.go` — AdminView fields (EventID, EventStatus,
  EventStatusError, EventEnrolled), validEventTransition, renderEventManage,
  handleEventStatus, handleEventReport.
- `r3-intake/internal/server/server.go` — registered POST /admin/events/{id}/status
  and GET /admin/events/{id}/report (admin-only).
- `r3-intake/internal/assets/public/index.html` — Report button for completed events;
  rewritten event-manage template; new event-report stub; cache-buster ?v=4→?v=5.
- `r3-intake/internal/assets/public/app.css` — .event-manage-actions, .event-readonly.
- `r3-intake/internal/server/admin_events_test.go` — TestEventStatusTransition (9 cases)
  + extended TestAdminEventsRender (Report presence/absence, active controls,
  terminal read-only, error rendering, report stub).

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (TestEventStatusTransition + TestAdminEventsRender + existing)

## Notes
- Built on Story 2.1 (Event List and Creation) work, which was applied to this
  branch from the parent worktree (t_9c800987) since it was uncommitted there.
- Matrix keeps `loadEvents` active-only (completed events are read-only; enrollment
  disabled). Flagged to parent if completed events must appear in matrix.
- Report page is a placeholder; CSV export is Epic 3.
