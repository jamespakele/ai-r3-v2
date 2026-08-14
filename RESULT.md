# RESULT: Remove Manage route and enrollment handlers from server

## What was built
Removed the Event Manage screen and its enrollment-management handlers from
the Go server (server-side card for Epic 17 "Remove the Event Manage screen").

### server.go (Mux)
- Removed 5 route registrations:
  - `GET /admin/events/` subtree -> `handleAdminEventManage`
  - `POST /admin/events/{id}/enroll` -> `handleEventEnroll`
  - `POST /admin/events/{id}/unenroll` -> `handleEventUnenroll`
  - `GET /admin/events/{id}/enroll-search` -> `handleEnrollSearch`
  - `POST /admin/events/{id}/status` -> `handleEventStatus`
- Kept: POST update, POST delete, POST add, GET report, and
  `eventEnrollmentCollection()`.

### admin.go
- Removed 5 AdminView fields (EventSiteName, EventStatusError, EventEnrolled,
  EnrolledCount, Enrolled); kept EventStatus.
- Removed 3 view types (EnrolledRow, EnrollSearchResult, EnrollSearchView);
  kept EventRow + its Enrolled field.
- Removed 11 handler/helper funcs: validEventTransition, renderEventManage,
  handleAdminEventManage, loadEnrolledRoster, loadEnrollmentStats,
  enrollmentRate, daysInRange, handleEventEnroll, handleEventUnenroll,
  respondRoster, handleEnrollSearch, handleEventStatus.
- Dropped the now-unused `sort` import.
- Kept loadEnrolledCount + loadAllEvents (Events tab "Enrolled" column still
  uses loadEnrolledCount via EventRow.Enrolled), handleEventReport, and the
  update/delete/add handlers.

## Verification
- `go build ./...` — PASSES, zero errors (non-test packages).
- `go vet ./...` — only test-file errors remain (e.g. admin_events_test.go:96
  unknown field EventSiteName). Those belong to sibling card t_4ac127a6
  (Update admin events tests for removed Manage screen), NOT this card.
- Templates (index.html) and *_test.go untouched (separate sibling cards).

## Artifacts
- docs/plans/omp-plan-remove-event-manage.md (the MOA working plan)

## Files changed
- r3-intake/internal/server/server.go
- r3-intake/internal/server/admin.go
