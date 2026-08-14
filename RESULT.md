# Epic 12: Event CRUD — Add, Update, and Soft Delete

**Status:** COMPLETE — child story branches merged into `epic/12-event-crud-add-update-and-softdelete-for`.

## Stories implemented

- **t_7d99815c** — Add `deleted` bool field to the `events` collection.
  Adds Go migration `014_events_deleted.go` (idempotent up/down) and registers it
  in `migrations.go`. The flag is the soft-delete mechanism; rows are never
  removed.

- **t_dc1890be** — Event update + soft-delete handlers and list filtering.
  Adds `adminEventUpdate` / `handleAdminEventUpdate` (POST
  `/admin/events/{id}/update`) and `handleAdminEventDelete` (POST
  `/admin/events/{id}/delete`, idempotent, flips `deleted=true`). Update
  validates name/site/dates/description and re-renders the admin page with the
  submitted values preserved on failure. Deleted events are excluded from
  `loadAllEvents` (`deleted=false`), from `loadEvents`
  (`status='active' && deleted=false`), and the manage screen 404s for deleted
  events.

- **t_a06d5f82** — Edit/Delete actions in the admin Events tab.
  Extends `EventRow` with `SiteID` and `Description`, populates them in
  `loadAllEvents`, and adds an inline edit form (name, site, dates,
  description) plus a confirm-gated Delete button per event row in
  `index.html`. The edit form posts to `/admin/events/{id}/update`; the delete
  button submits the hidden form to `/admin/events/{id}/delete`.

- **t_2644b842** — Integration tests for event update, soft-delete, and list
  filtering. Adds `admin_events_update_delete_test.go` covering: successful
  update (mutates only editable fields, preserves lifecycle/ownership, 303
  redirect), all validation failures (200 + exact error + values preserved),
  update/delete 404 for missing records, delete idempotency, deleted events
  excluded from `loadAllEvents` and `loadEvents`, manage screen 404 for deleted
  events, auth boundary (non-admin rejected), and CSRF rejection.

## Files changed

- `r3-intake/internal/server/admin.go` — `EventRow` fields, update/delete
  handlers, deleted guard on manage, `deleted=false` filter in `loadAllEvents`.
- `r3-intake/internal/server/attendance.go` — `deleted=false` filter in
  `loadEvents`.
- `r3-intake/internal/server/server.go` — `POST /admin/events/{id}/update` and
  `POST /admin/events/{id}/delete` routes.
- `r3-intake/internal/assets/public/index.html` — inline Edit form + Delete
  button in the Events tab.
- `r3-intake/internal/server/admin_events_update_delete_test.go` — new
  integration tests.
- `r3-intake/pocketbase/migrations/014_events_deleted.go` — new migration.
- `r3-intake/pocketbase/migrations/migrations.go` — registers migration 014.
- `docs/plans/omp-plan-events-deleted-field.md`,
  `docs/plans/omp-plan-events-update-softdelete.md`,
  `docs/plans/omp-plan-events-edit-delete-ui.md`,
  `docs/plans/omp-plan-events-update-delete-tests.md` — working plans.
- `.hermes/plans/WORKING_PLAN_014_events_deleted.md`,
  `WORKING_PLAN_update_softdelete.md`,
  `WORKING_PLAN_event_update_delete_tests.md`,
  `.hermes/plans/events-edit-delete.md` — working plans.

## Merge resolution notes

- Migration `014_events_deleted.go` and its `migrations.go` registration were
  identical across three child branches; git auto-resolved the duplicates.
- `admin.go` merged cleanly: the `deleted=false` filter from the handler branch
  and the `SiteID`/`Description` `EventRow` fields from the UI branch both
  landed in `loadAllEvents`.
- Each child branch replaced `RESULT.md` with its own story-level summary; the
  file was synthesized into this Epic 12 document.

## Verification

- `make build` — pass (produces `./r3-intake` binary; `cmd/r3-intake/main.go` is gitignored and was restored from the main repo working tree)
- `go vet ./...` — pass
- `go test ./...` — pass (`ok r3-intake/internal/server 11.296s`)
- Targeted event tests — all PASS (update success/validation/404, delete success/idempotent/404, list filtering, manage 404, auth boundary, CSRF rejection)
- Conflict-marker sweep — none
