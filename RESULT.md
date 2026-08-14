# Epic 17: Remove the Event Manage Screen

**Status:** COMPLETE — child story branches merged into `epic/17-remove-the-event-manage-screen-rely-on-m`.

## Stories implemented

- **t_1c442540** — Remove Manage route and enrollment handlers from server.
  Removed 5 route registrations from `server.go` (`GET /admin/events/` subtree,
  `POST /admin/events/{id}/enroll`, `POST /admin/events/{id}/unenroll`,
  `GET /admin/events/{id}/enroll-search`, `POST /admin/events/{id}/status`).
  Removed 5 `AdminView` fields, 3 view types (`EnrolledRow`,
  `EnrollSearchResult`, `EnrollSearchView`), and 11 handler/helper funcs from
  `admin.go`. Dropped the unused `sort` import. Kept `loadEnrolledCount`,
  `loadAllEvents`, event report, and event create/update/delete handlers.

- **t_4ac127a6** — Update admin events tests for removed Manage screen.
  Deleted `event_enrollment_flow_test.go`. Updated `admin_events_test.go` and
  `admin_events_update_delete_test.go` to remove manage-link assertions,
  `event-manage` render blocks, and helpers that exercised removed routes and
  types. Dropped the now-unused `time` import in `admin_events_test.go`. Kept
  event-report render, admin list render, validation-error path, create-route
  tests, auth tests, and all update/delete tests.

- **t_a7679d78** — Remove Event Manage UI and per-event Manage link.
  Removed the Manage anchor from the admin Events tab row-actions in
  `index.html`. Deleted the `event-manage`, `event-roster`, and
  `enroll-search-results` template blocks. Removed the
  `.event-manage-actions` rule and the Event-enrollment CSS section
  (`.enroll-tabs`, `.enroll-tab`, `.search-panel`, `.enroll-results`,
  `.enroll-result`, `.roster-table`, etc.) from `app.css`. Kept
  `.event-readonly`, `.form-grid-2`, `.rate-good`, `.rate-low`.

## Files changed

- `r3-intake/internal/server/server.go` — removed Manage/enrollment routes.
- `r3-intake/internal/server/admin.go` — removed Manage view types, fields,
  handlers, helpers, and the `sort` import.
- `r3-intake/internal/server/admin_events_test.go` — removed manage-link and
  manage-render tests; dropped unused `time` import.
- `r3-intake/internal/server/admin_events_update_delete_test.go` — removed
  `doEventManage` helper and manage 404 test.
- `r3-intake/internal/server/event_enrollment_flow_test.go` — deleted.
- `r3-intake/internal/assets/public/index.html` — removed Manage link and
  manage/roster/enroll-search template blocks.
- `r3-intake/internal/assets/public/app.css` — removed manage-actions and
  enrollment CSS rules.
- `docs/plans/omp-plan-remove-event-manage.md` — working plan (t_1c442540).
- `docs/plans/omp-plan-update-admin-events-tests.md` — working plan (t_4ac127a6).

## Merge resolution notes

- `server.go` and `admin.go` merged cleanly from t_1c442540; no overlapping
  changes with t_4ac127a6 (test-only branch).
- `index.html` and `app.css` merged cleanly from t_a7679d78; only UI
  deletions, no overlap with server or test changes.
- `RESULT.md` was replaced independently by each child branch. Synthesized into
  this Epic 17 document.

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass
- Conflict-marker sweep — none
