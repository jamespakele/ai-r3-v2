# Working Plan: Add regression test for POST /admin/events route

## Objective

Add regression tests covering the **auth boundary** of the `POST /admin/events` route (the Create Event form action) in the R3 Intake Go server. The route is wrapped in `requireRole("admin", ...)`, and the parent card already covered the happy path (`TestAdminEventCreateRouting`) and validation path (`TestAdminEventCreateValidation`). The remaining gap is proving that unauthenticated and non-admin callers are rejected **before** any event is created.

Concretely, add tests that prove:
1. **Unauthenticated** POST to `/admin/events` (no session cookie) -> `303` redirect to `/login`, and **no** event record is created.
2. **Non-admin** (case_manager role) POST to `/admin/events` -> `303` redirect to `/login`, and **no** event record is created.
3. **(Optional)** GET to `/admin/events` (no trailing slash) does **not** create an event and does **not** 404 - it is redirected to the `/admin/events/` subtree (Go ServeMux auto-redirect, `301`).

## Constraints

- **Language:** Go (module `r3-intake/`, `go.mod`).
- **Framework:** stdlib `testing` + `net/http/httptest` only - no external test framework.
- **Test file:** `r3-intake/internal/server/admin_events_test.go`, package `server` (same package, so unexported helpers/fields are directly accessible).
- **Reuse existing helpers** - do not re-implement server bootstrapping, seeding, cookie creation, or the POST helper.
- **No production code changes** - this is test-only. The route fix (commit `8c544a9`) is already merged into this worktree.
- Build/test commands run from `r3-intake/`: `go build ./... && go vet ./... && go test ./internal/server/`.

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `r3-intake/internal/server/admin_events_test.go` | **Modify** (append) | Add the auth-boundary regression tests. No other files change. |

No new files are required - the tests belong in the existing `admin_events_test.go` alongside the other Create Event tests.

## Implementation Notes

### Route & auth behavior to assert against (verified in source)

- `server.go:129` registers `mux.HandleFunc("POST /admin/events", s.requireRole("admin", s.handleAdminEventAdd))`.
- `requireRole` (`auth.go:77`) calls `s.currentSession(r)`; if `u == nil || u.Role != role` it does `http.Redirect(w, r, "/login", http.StatusSeeOther)` and returns **before** the handler runs. So both the unauthenticated and non-admin cases short-circuit at the boundary - `handleAdminEventAdd` / `adminEventAdd` never execute, and no event is created.
- **Important:** `requireRole` redirects to plain `/login` (no `?next=` query), unlike `requireAuth` which uses `/login?next=...`. Assert `Location == "/login"` exactly.
- `adminEventAdd` (`admin.go:398`) is the only place an `events` record is created; it is unreachable when `requireRole` rejects.

### Test helper functions to reuse (all already in package)

- `newTestServer(t *testing.T) *Server` - boots a real in-process PocketBase with fresh temp data dir. Each test calls this, so servers are isolated per test (no cross-test event-name collisions).
- `seedToggleData(t *testing.T, pb *pocketbase.PocketBase) toggleFixtures` - creates one active site + one admin user; returns `fx.site`, `fx.admin1`. Use `fx.site` as the valid `site` form value and `fx.admin1` as the session user id.
- `adminCookie(srv *Server, id string) *http.Cookie` - admin session cookie.
- `cmCookie(srv *Server, id string) *http.Cookie` - **case_manager** session cookie. It signs a `sessionUser{Role: "case_manager"}` for the given id; pass `fx.admin1` (any id works - `requireRole` only inspects the session role, not the DB record).
- `doEventCreate(srv *Server, cookie *http.Cookie, form url.Values) *httptest.ResponseRecorder` - POSTs the Create Event form to `/admin/events` (no trailing slash). **Pass `nil` for the cookie to simulate an unauthenticated request.**
- `findEventByName(t *testing.T, srv *Server, name string) *core.Record` - returns the first `events` record with the given name, or `nil`. Use to assert no event was created.

### Test design

**Test 1 - `TestAdminEventCreateAuthBoundary` (unauthenticated):**
- `srv := newTestServer(t)`, `fx := seedToggleData(t, srv.pb)`.
- Build a **valid** form (name, site=`fx.site`, valid dates) - the same form that succeeds for an admin - so any event creation would be attributable to the auth gap, not to bad input.
- `rec := doEventCreate(srv, nil, form)`.
- Assert `rec.Code == http.StatusSeeOther` (303) and `rec.Header().Get("Location") == "/login"`.
- Assert `findEventByName(t, srv, "New Event") == nil` (no event created).

**Test 2 - `TestAdminEventCreateNonAdminRejected` (case_manager):**
- Same setup; `cm := cmCookie(srv, fx.admin1)`.
- `rec := doEventCreate(srv, cm, form)` with the same valid form.
- Assert `rec.Code == 303`, `Location == "/login"`.
- Assert `findEventByName(t, srv, "New Event") == nil`.

**Test 3 (optional) - `TestAdminEventCreateGetNoCreate`:**
- Setup with admin cookie.
- Issue a plain `GET /admin/events` (no trailing slash) via `httptest.NewRequest(http.MethodGet, "/admin/events", nil)` + `srv.Mux().ServeHTTP`.
- Assert the response is **not** `404` and **not** `200`-with-event-created. Expected: Go ServeMux auto-redirects `/admin/events` -> `/admin/events/` with `301` (because the `/admin/events/` subtree pattern exists). Assert `rec.Code == http.StatusMovedPermanently` and `Location == "/admin/events/"`.
- Assert `findEventByName(t, srv, "New Event") == nil`.
- **Note:** verify the actual status code when running - if ServeMux behavior differs (e.g. `404`), adjust the assertion to match observed behavior and document it. The core invariant is: GET does not create an event and does not 404.

### Edge cases / pitfalls

- Use a **distinct event name** per test (e.g. `"AuthBoundary Event"`) so a failure is unambiguous; each test's own `newTestServer` already isolates state, but distinct names aid debugging.
- Do **not** assert on the response body for the rejected cases - `requireRole` writes only the redirect, no body content worth checking.
- The `cmCookie` id need not reference a real user; `requireRole` checks only the signed session role. Passing `fx.admin1` is fine and avoids needing a seeded case_manager.
- Keep the form **valid** in the rejection tests so the only reason no event is created is the auth boundary (not validation).

## Verification Criteria

1. `cd r3-intake && go build ./...` - compiles cleanly.
2. `go vet ./...` - no vet warnings.
3. `go test ./internal/server/ -run 'TestAdminEventCreate' -v` - the new auth-boundary tests plus the existing `TestAdminEventCreateRouting` / `TestAdminEventCreateValidation` all pass.
4. `go test ./internal/server/` - full package suite still green (no regressions from the added tests).
5. **Negative check (proves the tests are meaningful):** temporarily comment out the `requireRole` wrapper on `server.go:129` (or change the role to `"case_manager"`) and confirm the unauthenticated/non-admin tests **fail** (event gets created / no redirect). Restore the line afterward. This confirms the tests actually guard the auth boundary.
6. Confirm the optional GET test's asserted status matches real ServeMux behavior (adjust if it reports `404` instead of `301`).
