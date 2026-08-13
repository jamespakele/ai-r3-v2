# Working Plan: Diagnose and fix POST /admin/events 404 routing

## Objective

Make the "Create Event" form actually create an event. Today the form in `index.html` posts to `action="/admin/events"` (no trailing slash). Go's `http.ServeMux` has a subtree pattern `/admin/events/` registered, so a POST to `/admin/events` gets a **301 redirect** to `/admin/events/`; the browser re-issues it as a **GET**, which lands on `handleAdminEventManage` (GET-only, empty id) → `FindRecordById("events","")` fails → **404**. The POST never reaches `adminEventAdd`.

The fix registers an explicit `POST /admin/events` route that calls `adminEventAdd`, so the POST is handled directly and no redirect occurs. The form action stays `/admin/events` (unchanged).

## Constraints

- **Language:** Go 1.25 module (`go.mod` says `go 1.25.0`, toolchain `go1.23.4`).
- **Framework:** Standard library `net/http` with Go 1.22+ method-prefixed patterns (`"POST /path"`), embedded PocketBase server, server-rendered Go templates.
- **Dependencies:** No new dependencies. Reuse existing helpers: `s.currentSession(r)`, `s.requireRole("admin", ...)`, `s.adminEventAdd(w, r, u)`.
- **Scope:** Minimal fix only. No unrelated refactors, no template changes, no form-action changes.
- **Conventions:** Routes registered in `Server.Mux()` in `server.go`. Admin-only routes guarded by `requireRole("admin", ...)`. `adminEventAdd` requires the session user `u *sessionUser`. All timestamps HST via `time.Now().In(hst)`.

## File Structure

**Modify — `r3-intake/internal/server/server.go`** (route registration in `Mux()`):
- Add one new route: `mux.HandleFunc("POST /admin/events", s.requireRole("admin", s.handleAdminEventAdd))` — placed adjacent to the existing `POST /admin/events/{id}/status` route (line ~125) for consistency.

**Modify — `r3-intake/internal/server/admin.go`**:
- Add a thin handler `handleAdminEventAdd(w http.ResponseWriter, r *http.Request)` that fetches the session user and delegates to the existing `adminEventAdd`. Place it near `adminEventAdd` (line ~398) or near the other route handlers.

**Modify — `r3-intake/internal/server/admin_events_test.go`** (or a new `admin_events_routing_test.go`):
- Add a regression test that POSTs to `/admin/events` through `srv.Mux()` and asserts the event is created (no 404, no redirect). See Implementation Notes.

**No change — `r3-intake/internal/assets/public/index.html`**: the form action stays `action="/admin/events"`.

## Implementation Notes

### 1. New route in `server.go`
Register an explicit method-scoped pattern so ServeMux matches the POST directly and never issues the 301:
```go
mux.HandleFunc("POST /admin/events", s.requireRole("admin", s.handleAdminEventAdd))
```
- `requireRole("admin", ...)` already guards admin-only access and redirects non-admins/unauthenticated users to `/login` (auth.go line 77). This matches the existing `POST /admin/events/{id}/status` pattern style.
- Because the pattern is method-scoped (`POST`), a **GET** to `/admin/events` is unaffected — it still falls through to the `/admin/events/` subtree redirect behavior (unchanged). Only the POST is now handled directly.

### 2. New handler in `admin.go`
`adminEventAdd` needs the session user `u`. Add a small adapter handler:
```go
func (s *Server) handleAdminEventAdd(w http.ResponseWriter, r *http.Request) {
    u := s.currentSession(r)
    if u == nil {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }
    s.adminEventAdd(w, r, u)
}
```
- `requireRole` guarantees `u` is non-nil and `Role == "admin"` before this runs, so the nil-check is defensive only (mirrors the guard style used elsewhere). Keep it for safety.
- Do **not** modify `adminEventAdd` itself — it already handles validation, re-rendering the admin page with an error on failure, and redirecting to `/admin` on success. It is reused as-is.
- The existing `case path == "events" && u.Role == "admin": s.adminEventAdd(w, r, u)` branch in `handleAdminSub` (admin.go line 236) can remain — it is now effectively dead for the create-event path (the 301 never reaches it), but removing it is out of scope. Leave it untouched to keep the change minimal.

### 3. Form action — keep `/admin/events` (no slash)
- **Do not** change the form action to `/admin/events/`. A POST to `/admin/events/` would match the `/admin/events/` subtree → `handleAdminEventManage` (GET-only, 404s on non-GET). The explicit `POST /admin/events` route with the form action unchanged is the correct approach.
- The existing `TestAdminEventsRender` asserts `action="/admin/events"` is present in the rendered template — leaving the form unchanged keeps that test green.

### 4. Regression test (recommended)
There is a full integration harness available: `newTestServer(t)` (attendance_export_integration_test.go line 23), `adminCookie(srv, id)` (line 200), and the `seedToggleData`-style seeding pattern. Add a routing test in `admin_events_test.go` (or a new `admin_events_routing_test.go` in the same `server` package) that:
1. Builds a test server via `newTestServer(t)`.
2. Seeds an active site and an admin user (mirror `seedToggleData`).
3. Builds an admin cookie via `adminCookie(srv, adminID)`.
4. POSTs to `/admin/events` with `Content-Type: application/x-www-form-urlencoded` and form values `name`, `site`, `start_date`, `end_date`, `description` (valid dates, e.g. `2026-08-01`/`2026-08-14`).
5. Asserts the response is **not** a 301/302 redirect and **not** a 404 — expect `http.StatusSeeOther` (303) redirect to `/admin` on success (that is `adminEventAdd`'s success path), and assert the event record now exists in the `events` collection (query via `srv.pb`).
6. Optionally assert that a POST with an empty `name` re-renders the admin page (200) with the error message, exercising the validation path through the new route.

This test would have failed before the fix (it would have received the 301 → GET → 404), so it is a true regression guard.

### 5. Edge cases
- **Non-admin / unauthenticated POST:** `requireRole("admin", ...)` redirects to `/login` (303). No change needed.
- **GET to `/admin/events`:** still 301-redirects to `/admin/events/` (existing behavior, unchanged). Only the POST is newly handled.
- **Validation failure:** `adminEventAdd` re-renders the admin template with `EventError` and preserved submitted values — works unchanged through the new route.
- **Method mismatch:** the method-scoped pattern means only POST matches; other methods to `/admin/events` fall through to existing behavior.

## Verification Criteria

1. **`go build ./...`** in `r3-intake/` compiles cleanly.
2. **`go test ./internal/server/`** passes, including the existing `TestAdminEventsRender` (form action unchanged) and the new routing regression test.
3. **Manual check (optional):** start the server, log in as admin, submit the Create Event form, and confirm the event appears in the events table (no 404, no redirect-to-GET).
4. **Regression proof:** the new routing test fails on the pre-fix code (301 → GET → 404) and passes after the fix (event created, 303 → `/admin`).
