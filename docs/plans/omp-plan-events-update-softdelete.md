# Working Plan

## Objective

Add two admin-only event mutation handlers to `r3-intake/internal/server/admin.go`
and exclude soft-deleted events from every event list/selector so a deleted event
is invisible and unmanageable:

1. `POST /admin/events/{id}/update` — update an event's `name`, `site`,
   `start_date`, `end_date`, `description`. Validate exactly like
   `adminEventAdd`. On validation failure re-render the admin page with the
   error and preserved values. On success save and redirect to
   `/admin?tab=events`.
2. `POST /admin/events/{id}/delete` — soft-delete an event by setting
   `deleted=true` (NOT a hard delete). Redirect to `/admin?tab=events`.

List filtering (soft-deleted events excluded):
- `loadAllEvents()` in admin.go (admin Events list)
- `loadEvents(siteID)` in attendance.go (attendance matrix event selector +
  person attendance)
- `handleAdminEventManage` (event-manage page) — a soft-deleted event must 404 /
  not be manageable

The `deleted` bool field already exists on the events collection via migration
`014_events_deleted.go` (registered in `migrations.go`). No new migration is
needed. This card implements ONLY handlers + list filtering — no UI templates,
no tests (sibling cards own those).

## Constraints

- PocketBase v0.39 Go API: `s.pb.FindCollectionByNameOrId`, `s.pb.Save`,
  `s.pb.FindRecordById`, `s.pb.FindRecordsByFilter`. No `app.dao()`.
- Records: `core.NewRecord(col)`, `rec.Set("field", val)`, `rec.GetString`,
  `rec.GetBool`, `rec.Id`.
- Filter escaping: `mcpmod.EscapeFilter(s)` from `r3-intake/internal/mcp`
  (import as `mcpmod`) for any user-supplied value in a PB filter string.
- Auth: admin routes wrapped with `s.requireRole("admin", handler)` in
  `server.go` Mux(), which guarantees a non-nil admin session. Use
  `r.PathValue("id")` for the id segment (matches `handleEventStatus`).
- CSRF: all non-safe methods pass through `s.csrfMiddleware`; the new routes
  must be registered inside it (same as existing admin mutations).
- All timestamps HST; dates are `YYYY-MM-DD` strings parsed with
  `time.Parse("2006-01-02", ...)`.
- Do NOT add UI templates or tests — sibling cards (t_a06d5f82 UI,
  t_2644b842 tests) own those. Do NOT add a migration.
- The `deleted` field is a bool; PB filter syntax `deleted=false` /
  `deleted=true`. Existing records default to `false` (Required:false).

## File Structure

- `r3-intake/internal/server/admin.go` — add `handleAdminEventUpdate` and
  `handleAdminEventDelete`; change `loadAllEvents` filter; add a `deleted`
  guard in `handleAdminEventManage`.
- `r3-intake/internal/server/attendance.go` — change `loadEvents` filter to
  exclude `deleted=true`.
- `r3-intake/internal/server/server.go` — register the two new routes in
  `Mux()`.
- `r3-intake/pocketbase/migrations/014_events_deleted.go` — already present;
  no change.

## Implementation Notes

### 1. `POST /admin/events/{id}/update` — `handleAdminEventUpdate`

Signature matches the existing admin-mutation pattern:
`func (s *Server) handleAdminEventUpdate(w http.ResponseWriter, r *http.Request, u *sessionUser)`
(like `adminEventAdd`), wrapped by `requireRole("admin", ...)`.

Steps:
1. `id := r.PathValue("id")`; `rec, err := s.pb.FindRecordById("events", id)`.
   If `err != nil` → `http.NotFound(w, r)` (mirror `handleEventStatus`).
2. If `rec.GetBool("deleted")` is true → `http.NotFound(w, r)` (a soft-deleted
   event is not updatable).
3. Read + trim form values: `name`, `site`, `start_date`, `end_date`,
   `description` (same field names as `adminEventAdd`).
4. Build an `AdminView` with `UserName`, `Role`, `IsAdmin: true`, and the
   preserved values in `EventName/EventSite/EventStart/EventEnd/EventDescription`
   (so the re-render keeps the user's input).
5. Validate exactly like `adminEventAdd`:
   - `name == "" || site == ""` → "Event name and location are required."
   - `startErr != nil || endErr != nil` → "Start and end dates must be valid dates."
   - `endT.Before(startT)` → "End date must be on or after start date."
   - `len(desc) > 500` → "Description must be 500 characters or fewer."
6. On error: set `view.EventError = errMsg`, load `view.Sites = must(s.loadSites(true))`,
   `view.Users = s.loadUsers()`, `view.Events = must(s.loadAllEvents())`, then
   `_ = s.tpl.ExecuteTemplate(w, "admin", view)` and return. (Identical to the
   `adminEventAdd` error path so the admin page re-renders with the error and
   preserved values.)
7. On success: `rec.Set("name", name)`, `rec.Set("site", site)`,
   `rec.Set("start_date", start)`, `rec.Set("end_date", end)`,
   `rec.Set("description", desc)`; `_ = s.pb.Save(rec)`; then
   `http.Redirect(w, r, "/admin?tab=events", http.StatusSeeOther)`.

Do NOT touch `status`, `created_by`, or `deleted` on update.

### 2. `POST /admin/events/{id}/delete` — `handleAdminEventDelete`

Signature: `func (s *Server) handleAdminEventDelete(w http.ResponseWriter, r *http.Request, u *sessionUser)`.

Steps:
1. `id := r.PathValue("id")`; `rec, err := s.pb.FindRecordById("events", id)`.
   If `err != nil` → `http.NotFound(w, r)`.
2. If already `rec.GetBool("deleted")` → treat as no-op success (idempotent);
   still redirect to `/admin?tab=events`.
3. `rec.Set("deleted", true)`; `_ = s.pb.Save(rec)`.
4. `http.Redirect(w, r, "/admin?tab=events", http.StatusSeeOther)`.

This is a SOFT delete — never call `s.pb.Delete(rec)` / `app.delete`. The record
row stays; only the `deleted` flag flips to true.

### 3. List filtering

- `loadAllEvents()` (admin.go, ~L864): change filter from `"1=1"` to
  `"deleted=false"`. Keep sort `"-start_date,name"`, limit 1000, offset 0.
- `loadEvents(siteID)` (attendance.go, ~L416): change base filter from
  `"status='active'"` to `"status='active' && deleted=false"`. When
  `siteID != ""`, append ` && site='<escaped>'` as today. This keeps
  the matrix event selector and person-attendance event list free of deleted
  events.
- `handleAdminEventManage` (admin.go, ~L505): after the successful
  `FindRecordById`, add `if rec.GetBool("deleted") { http.NotFound(w, r); return }`
  so a soft-deleted event's manage page 404s and cannot be managed.

### 4. Route registration (server.go Mux(), ~L118-137)

Add, alongside the existing `POST /admin/events/{id}/status` route:

- `mux.HandleFunc("POST /admin/events/{id}/update", s.csrfMiddleware(s.requireRole("admin", s.handleAdminEventUpdate)))`
- `mux.HandleFunc("POST /admin/events/{id}/delete", s.csrfMiddleware(s.requireRole("admin", s.handleAdminEventDelete)))`

Both are method-scoped `POST` patterns with a `{id}` path segment, so they match
directly (no 301 subtree redirect trap) and are CSRF-protected. They coexist
with the GET-only `/admin/events/` subtree and the existing
`POST /admin/events/{id}/status` route; Go ServeMux longest-prefix matching
disambiguates them.

## Verification Criteria

- `go build ./...` and `go vet ./...` pass in `r3-intake/`.
- `POST /admin/events/{id}/update` with valid values updates the record and
  redirects (303) to `/admin?tab=events`; the record's `name`, `site`,
  `start_date`, `end_date`, `description` reflect the new values; `status`,
  `created_by`, `deleted` are unchanged.
- `POST /admin/events/{id}/update` with a missing name/site, invalid dates,
  end-before-start, or a >500-char description re-renders the admin page with
  `EventError` set and the submitted values preserved in the form fields.
- `POST /admin/events/{id}/delete` sets `deleted=true` on the record (row still
  exists — NOT hard-deleted) and redirects (303) to `/admin?tab=events`.
- A soft-deleted event is absent from the admin Events list (`loadAllEvents`),
  the attendance matrix event selector, and person-attendance event lists
  (`loadEvents`).
- `GET /admin/events/{id}/manage` for a soft-deleted event returns 404.
- `POST /admin/events/{id}/update` and `POST /admin/events/{id}/delete` for a
  soft-deleted event return 404.
- Non-admin requests to the new routes are rejected by `requireRole`.
- Requests without a valid CSRF token to the new POST routes are rejected (403)
  by `csrfMiddleware`.
- No new migration, template, or test file is added by this card.
