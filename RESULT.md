# RESULT — t_dc1890be: Implement update and soft-delete handlers with list filtering

## What was built
Implemented by omp (omp-plan-execute pipeline) per the MOA working plan.

1. **`POST /admin/events/{id}/update`** (`adminEventUpdate` + `handleAdminEventUpdate` in admin.go)
   - Loads the event, 404s on missing or soft-deleted records.
   - Reads/trims name, site, start_date, end_date, description.
   - Validates exactly like `adminEventAdd` (name+site required, valid dates, end>=start, desc<=500).
   - On failure re-renders the `admin` template with `EventError` + preserved values.
   - On success updates name/site/dates/description (leaves status/created_by/deleted untouched), redirects 303 to `/admin?tab=events`.

2. **`POST /admin/events/{id}/delete`** (`handleAdminEventDelete` in admin.go)
   - Soft-delete: flips `deleted=true`, never hard-deletes. Idempotent no-op if already deleted. Redirects 303 to `/admin?tab=events`.

3. **List filtering** — soft-deleted events excluded everywhere:
   - `loadAllEvents()` (admin.go): filter `1=1` → `deleted=false` (admin Events list).
   - `loadEvents(siteID)` (attendance.go): filter → `status='active' && deleted=false` (matrix selector + person attendance).
   - `handleAdminEventManage` (admin.go): 404 when `rec.GetBool("deleted")`.

4. **Routes** (server.go): registered both new POST routes under `csrfMiddleware(requireRole("admin", ...))`.

## Schema dependency
Copied `014_events_deleted.go` (deleted bool field) from parent worktree t_7d99815c and registered it in `migrations.go` so handlers build/test against the field. No new migration authored here.

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (internal/server ok, 7.2s)

## Files changed
- r3-intake/internal/server/admin.go
- r3-intake/internal/server/attendance.go
- r3-intake/internal/server/server.go
- r3-intake/pocketbase/migrations/migrations.go (register 014)
- r3-intake/pocketbase/migrations/014_events_deleted.go (copied dependency)

## Artifacts
- docs/plans/omp-plan-events-update-softdelete.md

## Notes
- No UI templates or tests added (sibling cards t_a06d5f82 UI, t_2644b842 tests own those).
- Runtime behavioral checks (update/delete/404/CSRF) require a running PB instance + admin session; static build/vet/test confirm clean compile.
