# Working Plan: Tests for Event Update, Soft-Delete, and List Filtering

## Objective

Prove that the already-implemented event update, soft-delete, and list-filtering handlers behave correctly in the R3 Intake Go server. Specifically, the tests must verify:

- **Update** (`POST /admin/events/{id}/update`): success path mutates only the editable fields, preserves `status`/`created_by`/`deleted`, and 303-redirects; each validation failure re-renders the `admin` template with HTTP 200 and the exact error message while preserving submitted values; 404 on a missing id and on a soft-deleted event.
- **Soft-delete** (`POST /admin/events/{id}/delete`): sets `deleted=true` and 303-redirects; idempotent on an already-deleted event; 404 on a missing id.
- **List filtering**: `loadAllEvents()` excludes deleted events (admin Events list); `loadEvents(siteID)` excludes deleted events (matrix selector + person attendance); `handleAdminEventManage` 404s on a deleted event.
- **Auth boundary**: unauthenticated and non-admin (case_manager) requests to the update/delete routes are rejected (303 redirect to `/login`).

## Constraints

- Language: Go, standard `testing` package only. No new dependencies.
- Tests run against a real in-process PocketBase booted by `newTestServer(t)` with all migrations applied (including `014_events_deleted.go`).
- Reuse existing helpers — do not reinvent them:
  - `seedToggleData(t, pb)` → `toggleFixtures{site, ev, admin1, iNoSite, iLocated}` (one active site, one active event "Morning Program" 2026-08-01..2026-08-31, one admin user, two intake records).
  - `adminCookie(srv, id)` / `cmCookie(srv, id)` for session cookies.
  - `addCSRFToRequest(req)` for the CSRF cookie + header.
  - `findEventByName(t, srv, name)` to locate an event record by name.
  - `doEventCreate(srv, cookie, form)` as the pattern for POSTing a form with CSRF + cookie.
- All state-changing requests go through `httptest.NewRequest` + `srv.Mux().ServeHTTP(rec, req)` with `addCSRFToRequest` and the appropriate cookie.
- Every POST must carry CSRF (routes are wrapped in `csrfMiddleware`); omit it and the request is rejected before reaching the handler.
- Do not modify production code. Only add test files.

## File Structure

Create one new test file:

```
r3-intake/internal/server/admin_events_update_delete_test.go
```

This keeps the update/delete/filtering coverage in one place, alongside the existing `admin_events_test.go` (create + render) and the shared helpers in `attendance_*_integration_test.go` / `csrf_test.go`. No existing test files need modification.

## Implementation Notes

### Shared setup pattern

Each test follows the same shape:

```go
srv := newTestServer(t)
fx := seedToggleData(t, srv.pb)
admin := adminCookie(srv, fx.admin1)
```

Build requests with a small local helper mirroring `doEventCreate`:

```go
func doEventUpdate(srv *Server, cookie *http.Cookie, id string, form url.Values) *httptest.ResponseRecorder {
    req := httptest.NewRequest(http.MethodPost, "/admin/events/"+id+"/update", strings.NewReader(form.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    addCSRFToRequest(req)
    if cookie != nil { req.AddCookie(cookie) }
    rec := httptest.NewRecorder()
    srv.Mux().ServeHTTP(rec, req)
    return rec
}
```

and an analogous `doEventDelete(srv, cookie, id)`. A `doEventManage(srv, cookie, id)` GET helper covers the manage-404 case.

### Seeding a deleted event

`seedToggleData` only creates an active event. To test deleted-event behavior, create an event (via `doEventCreate` or direct `pb.Save`), then flip the flag and re-save:

```go
rec := findEventByName(t, srv, "Morning Program")
rec.Set("deleted", true)
if err := srv.pb.Save(rec); err != nil { t.Fatalf(...) }
```

This is the canonical way to produce a soft-deleted record for the 404 and filtering tests.

### Test cases

**Update — success**
- POST valid form (`name`, `site`, `start_date`, `end_date`, `description`) to `/admin/events/{ev}/update`.
- Assert `rec.Code == 303` and `Location == "/admin?tab=events"`.
- Reload the record via `findEventByName`; assert `name`, `site`, `start_date`, `end_date`, `description` reflect the new values.
- Assert `status` is unchanged (`"active"`), `created_by` is unchanged, and `deleted` is still `false` (i.e. update does not touch lifecycle/ownership fields).

**Update — validation errors** (each re-renders `admin` template with HTTP 200, `EventError` set to the exact message, and submitted values preserved in the body):
- Empty `name` (or empty `site`) → `"Event name and location are required."`
- Invalid `start_date` or `end_date` (e.g. `"not-a-date"`) → `"Start and end dates must be valid dates."`
- `end_date` before `start_date` → `"End date must be on or after start date."`
- `description` longer than 500 chars → `"Description must be 500 characters or fewer."`
- For each: assert `rec.Code == 200`, body contains the message, and body contains the submitted (trimmed) values. Also assert the record in PB was NOT changed (still original values).

**Update — 404s**
- Missing id (e.g. `"nonexistent"`) → `rec.Code == 404`.
- Soft-deleted event (seeded per above) → `rec.Code == 404`.

**Delete — success**
- POST `/admin/events/{ev}/delete` → `rec.Code == 303`, `Location == "/admin?tab=events"`.
- Reload record; assert `deleted == true`.

**Delete — idempotent**
- Delete once (303), then delete again → still `rec.Code == 303` and record still `deleted == true` (no error, no 404).

**Delete — 404 on missing id**
- POST to `/admin/events/nonexistent/delete` → `rec.Code == 404`.

**List filtering**
- `loadAllEvents()`: seed a deleted event, call `srv.loadAllEvents()`, assert the deleted event is absent and the active one is present.
- `loadEvents(siteID)`: seed a deleted event, call `srv.loadEvents(fx.site)` and `srv.loadEvents("")`, assert the deleted event is absent (and, for the site-scoped call, only that site's events appear).
- `handleAdminEventManage`: GET `/admin/events/{deletedID}/manage` with admin cookie → `rec.Code == 404`. (Also confirm a non-deleted event returns 200.)

**Auth boundary**
- Unauthenticated: POST update and POST delete with no cookie → `rec.Code == 303`, `Location == "/login"`.
- Non-admin: POST update and POST delete with `cmCookie(srv, ...)` (a case_manager) → `rec.Code == 303`, `Location == "/login"`.
- (CSRF is implicitly exercised by every POST; a dedicated negative test asserting a POST without `addCSRFToRequest` is rejected is optional but cheap and valuable.)

### Pitfalls

- `requireRole("admin", ...)` redirects to `/login` (303) for both unauthenticated and wrong-role — assert on that, not on 403.
- The update/delete routes are method-scoped (`"POST /admin/events/{id}/update"`); a GET to the same path is not matched by these routes — use POST.
- `strings.TrimSpace` is applied to all form fields before validation, so a whitespace-only `name` is treated as empty.
- `loadEvents` also requires `status='active'`; a deleted-but-active event must be excluded, and a non-deleted-but-completed event must also be excluded (worth one assertion to lock the combined filter).
- The `admin` template re-render on validation failure requires `Sites`, `Users`, and `Events` to be populated — the handler does this internally, so the 200 assertion is sufficient; no extra setup needed.

## Verification Criteria

From the worktree root, all must pass:

```bash
go build ./...
go vet ./...
go test ./...
```

The new tests must be green alongside the existing suite (no regressions, no new dependencies, no production-code changes).
