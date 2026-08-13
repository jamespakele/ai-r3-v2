# Working Plan: R3 Attendance Foundation (Story 1.1) — Events + Enrollment + Attendance Schema, Route Skeleton, and Attendance Tab

## Objective

Implement the foundation card (Story 1.1) for the R3 Attendance feature in the Go server (`r3-intake/`). This card establishes the data layer and the minimal serving surface:

1. **Migration `007_events_attendance.js`** — create three PocketBase collections (`events`, `event_enrollment`, `attendance`) with the exact fields from `docs/attendance-prd.html §07`, all rules locked to `null` (Go is the policy layer), correct relations + cascade deletes, and a reverse-order down migration.
2. **New handler file `internal/server/attendance.go`** — initial skeleton in package `server` that registers the Attendance route and renders a minimal placeholder page.
3. **Route registration in `internal/server/server.go`** — `GET /attendance` wired through `requireAuth` (unauthenticated → `/public/intake`).
4. **Attendance tab in the topbar** — a link between Records and Admin that navigates to `GET /attendance`.

This is intentionally the **foundation** only. Downstream sibling cards (in other worktrees) build the full matrix grid UI, HTMX toggles, walk-in modal, events CRUD, enrollment, per-person calendar, and stats cards. **Do not** build any of that here — keep the skeleton compiling and minimal.

## Constraints

- **Language/Framework:** Go 1.25 (`go.mod` module `r3-intake`), stdlib `net/http` with `http.NewServeMux()`; PocketBase **v0.39** embedded via `*pocketbase.PocketBase`.
- **No source edits except the three listed files** (+ the `index.html` template and topbar). This is the foundation card.
- **PB JS API (v0.39):** camelCase methods — `app.findCollectionByNameOrId`, `app.save`, `app.delete`. **There is no `app.dao()`.** Use `new Collection({...})` with plain field objects (`type` discriminator). Follow the guarded idempotent `try/catch` pattern in `005_notes.js`.
- **Data rules:** All three collections get `listRule: null, viewRule: null, createRule: null, updateRule: null, deleteRule: null` — the browser never talks to PB directly; Go is the policy layer.
- **Cascade delete:** `events` → `event_enrollment` + `attendance` (`cascadeDelete:true`); `intake` → `event_enrollment` + `attendance` (`cascadeDelete:true`); `site`/`users`/`created_by`/`recorded_by` relations → `cascadeDelete:false` (restricted).
- **Timezones:** all timestamps/`now` derivations use the existing `var hst = time.FixedZone("HST", -10*60*60)` (already defined in `internal/server/admin.go:16`; same package — reuse it, do not redefine).
- **Handler skeleton scope:** register `GET /attendance` + a minimal `handleMatrix` that (a) renders a basic placeholder template, and (b) redirects unauthenticated users to `/public/intake` (via `requireAuth` or explicit `currentSession` check). No matrix grid, filters, toggle, walk-in, stats, CSV.
- **Go access pattern (server):** `s.pb.FindRecordsByFilter(colId, filter, sort, limit, offset)`, `s.pb.FindRecordById`, `s.pb.Save`, `s.pb.Delete`, `core.NewRecord(col)`. `s.currentSession(r)` returns `*sessionUser` or `nil`. `s.requireAuth(handler)` and `s.requireRole("admin", handler)` live in `auth.go`.
- **Branch:** `wt/t_0dbf9478` (clean). Module import path prefix `r3-intake/...`.

## File Structure

Files to **create**:

| Path | Purpose |
|------|---------|
| `r3-intake/pocketbase/migrations/007_events_attendance.js` | Migration defining `events`, `event_enrollment`, `attendance` (+ guarded down migration). |
| `r3-intake/internal/server/attendance.go` | New handler file (package `server`): `handleMatrix` skeleton + minimal view struct + template render. |

Files to **modify**:

| Path | Change |
|------|--------|
| `r3-intake/internal/server/server.go` | Register route in `Mux()`: `mux.HandleFunc("/attendance", s.requireAuth(s.handleMatrix))`. |
| `r3-intake/internal/assets/public/index.html` | Add `{{define "matrix"}}` placeholder template; add **Attendance** link to the topbar (between Records and Admin) in the `{{define "list"}}` topbar-actions. |

Do **not** create/modify: `internal/server/handlers.go`, `admin.go`, `notes.go`, `auth.go`, `app.css`, or any migration other than `007`. No new template file — add the `matrix` define to the existing embedded `index.html` (it's the single template blob parsed by `New()`).

## Implementation Notes

### 1. Migration `007_events_attendance.js`
- Header `/// <reference path="../pb_data/types.d.ts" />` and a comment noting it requires `sites`, `intake`, `users` from `001_init.js` (mirror `005_notes.js` style).
- Resolve parents once: `app.findCollectionByNameOrId("intake" | "sites" | "users")`.
- Guard each collection with the idempotent pattern:
  ```js
  let events = null;
  try { events = app.findCollectionByNameOrId("events"); } catch (e) {}
  if (!events) { events = new Collection({...}); app.save(events); }
  ```
  Do the same for `event_enrollment` and `attendance`. (The PRD sketch saves unconditionally; wrap each in the guarded pattern to stay idempotent and consistent with `005_notes.js`.)
- **`events`** (type `base`, null rules): `site` relation→sites req `cascadeDelete:false`; `name` text req max200; `start_date` text req max20; `end_date` text req max20; `description` text opt max500; `status` select `["active","completed","cancelled"]` maxSelect1 req; `created_by` relation→users opt `cascadeDelete:false`; `created` autodate onCreate; `updated` autodate onUpdate.
- **`event_enrollment`** (type `base`, null rules): `event` relation→events req `cascadeDelete:true`; `intake` relation→intake req `cascadeDelete:true`; `enrolled_date` text opt max20; `created` autodate onCreate.
- **`attendance`** (type `base`, null rules): `event` relation→events **opt (nullable)** `cascadeDelete:true`; `intake` relation→intake req `cascadeDelete:true`; `site` relation→sites req `cascadeDelete:false`; `recorded_by` relation→users opt `cascadeDelete:false`; `date` text req max20 (YYYY-MM-DD); `status` select `["present","absent","excused","walk_in"]` maxSelect1 req; `check_in_time` text opt max20 (HH:MM); `note` text opt max500; `created` autodate onCreate; `updated` autodate onUpdate.
- **Down migration:** drop in reverse dependency order:
  ```js
  for (const name of ["attendance", "event_enrollment", "events"]) {
    let col; try { col = app.findCollectionByNameOrId(name); } catch (e) {}
    if (col) app.delete(col);
  }
  ```
- Field-type field names must match the PRD exactly (`name`, `type`, `required`, `max`, `values`, `maxSelect`, `collectionId`, `cascadeDelete`, `onCreate`, `onUpdate`).

### 2. `internal/server/attendance.go` (skeleton)
- Package `server`; same package as `server.go`, so the `Server` type needs no import. Reuse `hst` from `admin.go` (do not redeclare — it's already `var hst` in this package).
- Minimal view struct, e.g.:
  ```go
  type AttendanceView struct {
      UserName string
      Role     string
      IsAdmin  bool
  }
  ```
  (Keep it to what the placeholder template needs; later cards extend it.)
- `handleMatrix` skeleton:
  - Option A: rely on `requireAuth` wrapping (recommended) — then inside just build the view from `s.currentSession(r)` and render. Option B: explicit `u := s.currentSession(r); if u == nil { http.Redirect(w, r, "/public/intake", http.StatusSeeOther); return }`. Match the `handleList` precedent in `admin.go` (which redirects to `/public/intake` when unauthenticated).
  - Render the placeholder via `_ = s.tpl.ExecuteTemplate(w, "matrix", view)`.
  - Do **not** add `FindRecordsByFilter("attendance", ...)`, `cycleStatus()`, `buildDateRange()`, matrix rows, or any data-loading logic — those belong to later cards. Keep the function compiling and minimal.
- Do not reference `cycleStatus`/`buildDateRange` (they don't exist yet) — adding calls to nonexistent helpers would break the build.

### 3. `internal/server/server.go`
- In `Mux()`, add one line near the other auth-gated routes (e.g. after the `/notes/` registration):
  ```go
  // Attendance matrix (auth-only)
  mux.HandleFunc("/attendance", s.requireAuth(s.handleMatrix))
  ```
- No imports change (method and type are in-package).

### 4. Template + topbar
- Add a minimal `{{define "matrix"}}<!DOCTYPE html>...` block to `index.html` (mirror the structure of the existing `list`/`notes` templates: topbar with brand + actions + logout, a container with a simple placeholder heading like "Attendance"). Keep it a valid standalone page so `ExecuteTemplate(w, "matrix", view)` renders without error.
- In the `{{define "list"}}` topbar-actions (`index.html` ~line 454), add the Attendance link **between the Records brand and the Admin link**, i.e. before the `{{if .IsAdmin}}<a href="/admin"...>Admin</a>{{end}}` line:
  ```html
  <a href="/attendance" class="btn btn-ghost">Attendance</a>
  ```
  This satisfies "Attendance tab appears in topbar between Records and Admin; clicking navigates to GET /attendance."

### Edge cases / pitfalls
- **`maxSelect:1` + `required:true/false`** are the field attributes that make relations single-value and nullable — keep them correct, especially `attendance.event` being **not required** (nullable) with `cascadeDelete:true`.
- **`select` fields need `maxSelect: 1`** and the `values` array exactly as specified.
- Migration is idempotent (guarded) so re-boot after a partial run won't error.
- Do not redefine `hst` in `attendance.go` — it already exists in the package (would be a redeclaration compile error).
- The embedded template is parsed wholesale by `New()`; an unmatched `{{define "matrix"}}` render call or missing `</html>`/unbalanced block fails template parse → server won't start. Ensure the new define is well-formed.
- Downstream cards will extend `attendance.go` and the `matrix` template — keep the skeleton's struct/template naming aligned with the PRD sketch (`MatrixViewData`) so later cards can grow it without renames. (Use the minimal `AttendanceView` now; a later card can rename/extend — note this in code comment.)

## Verification Criteria

Run from the worktree root (`/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_0dbf9478`):

1. `cd r3-intake && go build ./...` → **passes** (no compile errors; `attendance.go` compiles, template parses at build).
2. `go vet ./...` → **passes** (no vet warnings).
3. `go test ./...` → **passes** (existing tests still green; no new tests required for the skeleton).
4. **Migration correctness (schema review):** `007_events_attendance.js` creates `events`, `event_enrollment`, `attendance` with the exact PRD fields; all three have null list/view/create/update/delete rules; cascade deletes configured (`events→enrollment+attendance`, `intake→enrollment+attendance`, `site→restricted`); migration references `sites`/`intake`/`users` by name; down migration drops in reverse order `attendance → event_enrollment → events`.
5. **Route/tab review:** `Mux()` registers `/attendance` wrapped in `requireAuth`; the Attendance link sits between Records and Admin in the `list` topbar; unauthenticated `/attendance` redirects to `/public/intake` (matching `handleList` behavior).
6. **Scope discipline:** no matrix grid UI, filters, HTMX toggle, walk-in modal, events CRUD, enrollment, per-person calendar, or stats cards built in this card (left to downstream cards).
