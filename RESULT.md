# RESULT: Diagnose and fix POST /admin/events 404 routing

## Root cause
The "Create Event" form posts to `action="/admin/events"` (no trailing slash).
Go's `http.ServeMux` has a subtree pattern `/admin/events/` registered (for the
GET-only `handleAdminEventManage`). A POST to `/admin/events` therefore got a
**301 redirect** to `/admin/events/`; the browser re-issued it as a **GET**,
which landed on `handleAdminEventManage` (GET-only, empty id) ->
`FindRecordById("events","")` failed -> **404**. The POST never reached
`adminEventAdd`.

## Fix (implemented by omp)
1. `r3-intake/internal/server/server.go` - added explicit method-scoped route
   `mux.HandleFunc("POST /admin/events", s.requireRole("admin", s.handleAdminEventAdd))`
   adjacent to the existing `POST /admin/events/{id}/status` route. The POST is
   now handled directly; no 301 to the GET-only subtree.
2. `r3-intake/internal/server/admin.go` - added a thin `handleAdminEventAdd`
   adapter that fetches the session user and delegates to the existing
   `adminEventAdd` (unchanged). `handleAdminSub` branch left as-is.
3. `r3-intake/internal/assets/public/index.html` - form action unchanged
   (`/admin/events`). Changing it to `/admin/events/` would hit the GET-only
   manage handler and 404 on POST.
4. `r3-intake/internal/server/admin_events_test.go` - added regression tests:
   - `TestAdminEventCreateRouting` - POST -> 303 to `/admin`, event record
     created with `status=active`.
   - `TestAdminEventCreateValidation` - empty name -> 200 re-render with error,
     no record created.

## Verification
- `go build ./...` - clean
- `go vet ./...` - clean
- `go test ./internal/server/` - ok (3.869s), all tests pass including the new
  regression tests and existing `TestAdminEventsRender`.

Regression guard confirmed: `httptest.NewRecorder` does not follow redirects, so
pre-fix the POST returned 301 (not the asserted 303) and the test would fail.

## Artifacts
- Working plan: `docs/plans/omp-plan-fix-post-admin-events-404.md`
- Changed files: `server.go`, `admin.go`, `admin_events_test.go`
