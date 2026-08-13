# Epic 8: Create Event still returns an error

**Status:** COMPLETE — child story branches `wt/t_2c810945` (route fix) and `wt/t_a5d0cbfe` (auth boundary regression tests) merged into `epic/8-create-event-still-returns-an-error`.

## Stories implemented

- **8.1 Route fix** — `wt/t_2c810945` — fixed `POST /admin/events` returning 404 by registering an explicit method-scoped route, and added regression tests for the happy path and validation failure.
- **8.2 Auth boundary regression tests** — `wt/t_a5d0cbfe` — added regression tests proving unauthenticated users, non-admin users, and plain GET requests cannot create events through the new route.

## 8.1 Route fix (`wt/t_2c810945`)

### Root cause
The Create Event form posts to `action="/admin/events"` (no trailing slash). Go's `http.ServeMux` had a subtree pattern `/admin/events/` registered for the GET-only `handleAdminEventManage` handler. A POST to `/admin/events` therefore received a **301 redirect** to `/admin/events/`; the browser re-issued it as a **GET**, which landed on `handleAdminEventManage` with an empty id, failed `FindRecordById("events","")`, and returned **404**. The POST never reached `adminEventAdd`.

### Fix
1. `r3-intake/internal/server/server.go` — added an explicit method-scoped route:
   ```go
   mux.HandleFunc("POST /admin/events", s.requireRole("admin", s.handleAdminEventAdd))
   ```
   The POST is now handled directly; no 301 redirect to the GET-only subtree occurs.
2. `r3-intake/internal/server/admin.go` — added a thin `handleAdminEventAdd` adapter that fetches the session user and delegates to the existing `adminEventAdd`. `requireRole("admin", ...)` already guarantees an admin session, so the nil check is defensive only.
3. `r3-intake/internal/assets/public/index.html` — form action kept as `/admin/events` (no trailing slash).
4. `r3-intake/internal/server/admin_events_test.go` — added regression tests:
   - `TestAdminEventCreateRouting` — POST creates an active event and redirects to `/admin`.
   - `TestAdminEventCreateValidation` — empty name re-renders the admin page with the validation error and creates no event.

## 8.2 Auth boundary regression tests (`wt/t_a5d0cbfe`)

### What was built
Added three regression tests covering the auth boundary of the `POST /admin/events` route, which is wrapped in `requireRole("admin", ...)`:

1. `TestAdminEventCreateAuthBoundary` — unauthenticated POST → `303` redirect to `/login`, no event created.
2. `TestAdminEventCreateNonAdminRejected` — `case_manager` session POST → `303` redirect to `/login`, no event created.
3. `TestAdminEventCreateGetNoCreate` — plain `GET /admin/events` (no trailing slash) → `301` redirect to `/admin/events/` subtree, no event created.

These complement the route-fix story's `TestAdminEventCreateRouting` and `TestAdminEventCreateValidation`.

### Files changed
- `r3-intake/internal/server/admin_events_test.go` (+80 lines, appended).
- `docs/plans/omp-plan-admin-events-auth-regression.md` (new).

## Merge resolution notes

- Both child branches diverged from `3635b6b` and shared commit `8c544a9` (the route fix). `wt/t_a5d0cbfe` added two additional commits on top.
- Source files (`server.go`, `admin.go`, `admin_events_test.go`) auto-merged cleanly because the changes were additive.
- `RESULT.md` was modified by both branches with different content. The auto-merge left the auth-test-only version from `b07f7fc`; this file was manually synthesized to document both the route fix and the auth boundary tests.
- Both plan files are preserved:
  - `docs/plans/omp-plan-fix-post-admin-events-404.md`
  - `docs/plans/omp-plan-admin-events-auth-regression.md`

## Verification (after merge)

- `go build ./...` → exit 0
- `go vet ./...` → no issues
- `go test ./internal/server/ -run 'TestAdminEventCreate' -v` → all 5 tests pass
- `go test ./...` → full module green
- Conflict-marker sweep (`grep -RIl` for merge markers, excluding `.git/`) → no conflict markers
