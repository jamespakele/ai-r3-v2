# RESULT — t_2644b842: Write tests for update, soft-delete, and list filtering

## What was built
Implemented by omp (omp-plan-execute pipeline) per the MOA working plan.

Added `r3-intake/internal/server/admin_events_update_delete_test.go` (package `server`, no production-code changes, no new dependencies). Coverage:

1. **Update** (`POST /admin/events/{id}/update`)
   - Success: mutates only editable fields (name/site/start/end/description), preserves `status`/`created_by`/`deleted`, 303 to /admin?tab=events.
   - Validation: 6 table-driven cases (empty name, empty site, invalid start, invalid end, end-before-start, 501-char description) — each re-renders `admin` with 200, exact error message, submitted values preserved, stored record untouched.
   - 404 on missing id and on soft-deleted event.

2. **Soft-delete** (`POST /admin/events/{id}/delete`)
   - Success: sets `deleted=true`, 303 to /admin?tab=events.
   - Idempotent: re-delete still 303, record stays `deleted=true`.
   - 404 on missing id.

3. **List filtering**
   - `loadAllEvents()` excludes deleted events (admin Events list).
   - `loadEvents(site)` / `loadEvents("")` exclude deleted and completed (combined `status=active && deleted=false`).
   - `handleAdminEventManage` 404s on deleted event, 200 on live event.

4. **Auth boundary**
   - Unauthenticated and case_manager update/delete -> 303 /login, no mutation.
   - CSRF-less POST -> 403.

## Dependency
Copied the parent implementation (admin.go, attendance.go, server.go, migrations.go, 014_events_deleted.go) from worktree t_dc1890be so the tests exercise the real handlers. No production code authored here.

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (internal/server ok, 10.5s)
- New tests run individually: all PASS (11 test funcs, incl. table-driven subtests)

## Files changed
- r3-intake/internal/server/admin_events_update_delete_test.go (new)
- (dependency copies from parent t_dc1890be: admin.go, attendance.go, server.go, migrations.go, 014_events_deleted.go)

## Artifacts
- docs/plans/omp-plan-events-update-delete-tests.md
