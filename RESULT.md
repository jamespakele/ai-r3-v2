# RESULT — Story 2.1: Event List and Creation

## What was built
Implemented the admin Events accordion (list + create) for multi-week outreach
programs, per FR10/FR11/UX-DR6 and PRD §05 Screen 1.

- **Events accordion** in the admin settings page (alongside Sites and Users),
  showing a table with columns: Event Name, Location, Dates, Enrolled, Status,
  Actions.
- **Status badges**: active=green, completed=yellow, cancelled=red
  (`.event-status-*` variants).
- **"+ New Event"** form: 4-column grid (Event Name, Location select of active
  sites, Start Date, End Date) + optional Description (max 500) + Create Event
  button. On success creates a record with `status="active"`,
  `created_by=<current user>` and redirects to `/admin`.
- **Validation**: missing required fields, invalid dates, end-before-start, and
  description >500 chars re-render the admin page with an error and preserve
  entered values.
- **Actions**: "Manage" → `/admin/events/{id}/manage` (placeholder stub for
  Story 2.2); "Matrix" → `/attendance?event={id}` (pre-selected in filter).
- **Enrolled count** derived from `event_enrollment` junction (read-only).

## Files changed
- `r3-intake/internal/server/admin.go` — EventRow, AdminView.Events/EventError/
  form fields, loadAllEvents(), loadEnrolledCount(), adminEventAdd(),
  handleAdminEventManage(), wired `events` into handleAdminSub.
- `r3-intake/internal/server/server.go` — registered `/admin/events/` (admin-only).
- `r3-intake/internal/assets/public/index.html` — Events accordion + event-manage
  placeholder template; cache-buster `?v=3`→`?v=4`.
- `r3-intake/internal/assets/public/app.css` — `.event-status-*` badges,
  `.form-grid-4`, `.form-error`.
- `r3-intake/internal/server/admin_events_test.go` — new render test.

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (incl. new TestAdminEventsRender + existing
  TestMatrixContentRender)

## Notes
- No migration changes: `events` collection already exists via 007.
- `loadEvents` (active-only, matrix filter) left untouched; admin list uses new
  `loadAllEvents` (all statuses).
- Enrollment screen is a placeholder stub; full enrollment is Story 2.2.
