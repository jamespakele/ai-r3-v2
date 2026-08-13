# Working Plan: Event Enrollment and Roster Management (Story 2.2)

## Objective

Implement backend endpoints and UI views to **enroll participants from existing intake records into events** and **manage the event roster** (Story 2.2, Epic 2). This replaces the `event-manage` placeholder (added in Story 2.1) with a real enrollment screen that shows the event header (name, site, dates, status), an "Add participant" name-search + enroll action, and an enrolled-roster table with per-participant attendance stats (days attended, rate %, last present date) and a Remove (soft-delete unenroll) action. Enrollment is a many-to-many junction (`event_enrollment`) so a participant can be in multiple events; unenroll is a soft delete that preserves attendance history.

Covers **FR11** (enroll existing intake participants via search by name; view roster with days attended, rate %, last present), **FR12** (unenroll = soft delete preserving attendance history), and **FR19** (Events accordion in admin — already built in 2.1; this story wires the Manage action to a real screen). **UX-DR7** governs the admin events UI conventions (accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, Public Sans + Lora, 14px card radii, 8px input radii). PRD reference: `docs/attendance-prd.html` §05 **Screen 1b — Event Detail: Enrollment** (lines ~414–460) and the routes table (lines ~846–847).

## Constraints

- **Go is the policy layer.** All PocketBase access goes through `s.pb` in-process; the browser never talks to PB directly. All `events`/`event_enrollment`/`attendance` collection rules are `null` (locked) — enforcement happens in Go.
- **PocketBase v0.39 JS/Go API.** In Go use `s.pb.FindCollectionByNameOrId`, `s.pb.FindRecordsByFilter(col.Id, filter, sort, limit, offset)`, `s.pb.FindRecordById(col.Id, id)`, `s.pb.Save(rec)`, `core.NewRecord(col)`, `rec.Set("field", val)`, `rec.GetString("field")`, `rec.Id`. No `app.dao()`.
- **No native unique constraints in PB.** Enforce idempotency in Go by querying for an existing `event_enrollment` record on `(event, intake)` before creating — never duplicate.
- **Filter escaping:** use `mcpmod.EscapeFilter(s)` from `r3-intake/internal/mcp` (import as `mcpmod "r3-intake/internal/mcp"`) for any user-supplied value interpolated into a PB filter string.
- **Auth:** `requireRole("admin", handler)` wrapper for all new routes (enroll/unenroll are admin-only per PRD). `currentSession(r)` returns `*sessionUser` with `.ID`, `.Name`, `.Role`.
- **Time zone:** use the existing `var hst = time.FixedZone("HST", -10*60*60)` in `r3-intake/internal/server/admin.go`. Dates use `time.Now().In(hst).Format("2006-01-02")`; timestamps use `"2006-01-02 15:04:05"`.
- **Templates:** single embedded `index.html` with multiple `{{define}}` blocks; new blocks go at the end. Reference via `s.tpl.ExecuteTemplate(w, "name", view)`. Bump the `?v=` cache-buster on the stylesheet link when CSS changes.
- **HTMX:** use `hx-post` + `hx-target`/`hx-swap` for partial swaps; handlers return raw HTML fragments. Provide a no-JS fallback (native POST → 303 redirect) where cheap.
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, 14px card radii, 8px input radii. Reuse existing `.btn`, `.btn-ghost`, `.btn-tiny`, `.btn-danger`, `.status-badge`, `.event-status-*`, `.form-grid-4`, `.form-error`, `.walkin-result*` classes.
- **Verification gates:** `go build ./...`, `go vet ./...`, `go test ./...` must pass; template parse must succeed (all defines registered).

## File Structure

All paths relative to repo root `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_9de410fd`.

| File | Change |
|------|--------|
| `r3-intake/internal/server/admin.go` | **Extend.** Add `EnrolledRow` struct (roster row with stats) and `EnrollSearchResult` struct. Replace `handleAdminEventManage` body to load event + roster + render the real screen. Add `handleEventEnroll`, `handleEventUnenroll`, `handleEnrollSearch`, `loadEnrolledRoster`, `loadEnrollmentStats` helpers. |
| `r3-intake/internal/server/server.go` | **Add routes** for `POST /admin/events/{id}/enroll`, `POST /admin/events/{id}/unenroll`, and `GET /admin/events/{id}/enroll-search` (HTMX name search), all wrapped in `s.requireRole("admin", ...)`. |
| `r3-intake/internal/assets/public/index.html` | **Replace** the body of `{{define "event-manage"}}` (currently a placeholder at line ~965) with the full enrollment screen. **Add** `{{define "enroll-search-results"}}` fragment at the end for the HTMX search panel. |
| `r3-intake/internal/assets/public/app.css` | **Add** roster-table, search-panel, and rate-badge CSS classes. Bump the `?v=` cache-buster on the stylesheet link in the event-manage template. |
| `r3-intake/internal/server/admin_events_test.go` | **Extend.** Update the `event-manage` render assertions; add tests for the roster render, enroll/unenroll handler logic, and idempotency. |
| `r3-intake/pocketbase/migrations/007_events_attendance.js` | **No change** (collections already exist). Reference only. |

## Implementation Notes

### 1. Data model — `r3-intake/internal/server/admin.go`

Add a roster row struct (flat, template-ready):

```go
// EnrolledRow is one row of the event roster with attendance stats.
type EnrolledRow struct {
    IntakeID     string
    Name         string
    EnrolledDate string // YYYY-MM-DD from event_enrollment.enrolled_date
    DaysAttended int    // count of present + walk_in attendance records in event range
    TotalDays    int    // elapsed-or-total days in event range (denominator for rate)
    Rate         int    // percent, 0–100
    LastPresent  string // YYYY-MM-DD, "" if none
}

// EnrollSearchResult is one hit in the "Add participant" name search.
type EnrollSearchResult struct {
    ID       string
    Name     string
    SiteName string
    Already  bool // true if already enrolled in this event
}
```

### 2. `handleAdminEventManage` — real enrollment screen (GET `/admin/events/{id}/manage`)

Replace the placeholder body. Keep the existing route parsing (`id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/events/"), "/manage")`). Load the event record via `s.pb.FindRecordById("events", id)`; 404 if missing. Build the view:

- **Header:** event name, site name (via `s.siteNameMap()`), start/end dates, status badge. Buttons: `← All Events` (`/admin`) and `View Matrix` (`/attendance?event={id}`).
- **Tabs:** render "Enrolled (N)" as the active tab; "Attendance History" and "Summary" as inert placeholders (out of scope for this story — do not wire them).
- **Add participant:** a search input + `+ Enroll` button. The input triggers an HTMX GET to `/admin/events/{id}/enroll-search?name=...` targeting a results container (`hx-get`, `hx-target="#enroll-results"`, `hx-trigger="input changed delay:300ms, search"`). Results render as `enroll-search-results` rows; each row is a POST form to `/admin/events/{id}/enroll` with a hidden `intake_id`.
- **Roster table:** columns Participant, Enrolled (date), Days Attended (`N / M`), Rate (badge %), Last Present (date), Remove (button). Rows come from `loadEnrolledRoster`.

Reuse the existing `AdminView` struct (it already carries `UserName`, `Role`, `IsAdmin`, `EventName`); add the roster slice and event header fields to it (or a dedicated `EventManageView` — prefer extending `AdminView` to minimize template churn, matching how `handleAdminEventManage` already uses it).

### 3. `handleEventEnroll` (POST `/admin/events/{id}/enroll`)

- Parse `intake_id` from the form; parse event id from the path (same trim logic).
- Validate: event exists, intake exists (via `s.intakeCollection()` + `FindRecordById`). 404/400 on failure.
- **Idempotency:** query `event_enrollment` for `event='<id>' && intake='<intakeID>'` (both escaped). If a record already exists, treat as success (no-op) — do not create a duplicate.
- Create `core.NewRecord(enrollmentCol)`; set `event`, `intake`, and `enrolled_date = time.Now().In(hst).Format("2006-01-02")`. `s.pb.Save(rec)`.
- Response: HTMX — return the re-rendered roster fragment (or the full `event-manage` screen) so the table updates in place; no-JS fallback → 303 redirect back to `/admin/events/{id}/manage`.

### 4. `handleEventUnenroll` (POST `/admin/events/{id}/unenroll`)

- Parse `intake_id`; find the `event_enrollment` record on `(event, intake)`.
- **Soft delete:** do NOT delete the enrollment record (that would cascade-delete attendance via `cascadeDelete: true`). Instead mark it inactive. Since the migration has no `active`/`deleted` field, add a convention: set a sentinel field. **Recommended:** add a `deleted` boolean field to `event_enrollment` via a new migration `008_event_enrollment_deleted.js` (or reuse an existing optional field). The roster query filters `deleted=false`; `loadEnrolledCount` (used by the admin events list) must also filter `deleted=false` so the enrolled count stays consistent. If adding a migration is undesirable, an alternative is to physically delete the enrollment record — but that violates FR12 (soft delete preserving attendance history) because `cascadeDelete: true` on `event_enrollment.intake`/`event` would cascade to attendance. **Therefore a `deleted` flag migration is required.**
- Response: HTMX — re-render roster fragment; no-JS → 303 redirect.

### 5. `handleEnrollSearch` (GET `/admin/events/{id}/enroll-search`)

- Mirror `handleWalkinSearch` in `attendance.go`: read `?name=`, return empty body if `< 2` chars, cap at 10 results.
- Filter intake records by `name ~ "<escaped>"` **scoped to the event's site** (`site='<eventSite>'`) so only participants at the event location are offered (seamless with existing intake data). Sort `-created`.
- For each hit, set `Already = true` if an `event_enrollment` exists for `(event, intake)` (and not soft-deleted) so the UI can show "Already enrolled" and disable the button.
- Render `enroll-search-results` fragment.

### 6. `loadEnrolledRoster` + `loadEnrollmentStats`

- Query `event_enrollment` for `event='<id>' && deleted=false`, sorted by `enrolled_date,name` (or `-created`). For each, load the intake record (name) and compute stats.
- **Stats computation** (per enrolled participant, scoped to the event's date range `[start_date, end_date]`):
  - Query `attendance` for `intake='<iid>' && event='<id>' && date>='<start>' && date<='<end>'` (mirror the filter in `loadMatrixRows`).
  - `DaysAttended` = count of records with `status` in `{present, walk_in}`.
  - `TotalDays` = number of calendar days in the event range that have elapsed up to today (HST), clamped to `[start, end]`; if the event hasn't started, use 0 (avoid division by zero). Use `end` as the cap so future days aren't counted.
  - `Rate` = `DaysAttended * 100 / TotalDays` (guard `TotalDays == 0` → 0).
  - `LastPresent` = max `date` among present/walk_in records.
- Reuse `s.loadEnrolledCount` for the "Enrolled (N)" tab count, but update it to filter `deleted=false`.

### 7. Routes — `r3-intake/internal/server/server.go`

Register next to the existing `/admin/events/` route (all admin-only):

```go
mux.HandleFunc("/admin/events/", s.requireRole("admin", s.handleAdminEventManage))
mux.HandleFunc("/admin/events/enroll", s.requireRole("admin", s.handleEventEnroll))       // POST
mux.HandleFunc("/admin/events/unenroll", s.requireRole("admin", s.handleEventUnenroll))   // POST
mux.HandleFunc("/admin/events/enroll-search", s.requireRole("admin", s.handleEnrollSearch)) // GET
```

**Route-ordering caution:** Go's `http.ServeMux` matches the longest prefix. `/admin/events/enroll` and `/admin/events/enroll-search` are more specific than `/admin/events/`, so they must be registered (they will win over the catch-all). The existing `handleAdminEventManage` parses the id by trimming `/admin/events/` and `/manage`; the new handlers parse the id from the path segment after `/admin/events/` (e.g. `/admin/events/{id}/enroll`). To keep path parsing simple and unambiguous, prefer the PRD's exact paths `POST /admin/events/{id}/enroll` and `POST /admin/events/{id}/unenroll` — parse the id as the segment between `/admin/events/` and `/enroll`/`/unenroll`. Register these specific patterns; the catch-all `/admin/events/` still handles `/admin/events/{id}/manage`.

### 8. Template + CSS

- **`event-manage` template:** full HTML page (topbar + container), matching the existing placeholder's structure. Add the header block, tabs, search panel, and roster table. Use `{{range .Enrolled}}` for rows. Rate badge: reuse `.status-badge` with a color class based on rate (e.g. `>=70%` green, `40–69%` amber, `<40%` red) — add `.rate-high/.rate-mid/.rate-low` CSS.
- **`enroll-search-results` fragment:** list of `EnrollSearchResult` rows; each is a POST form to `/admin/events/{id}/enroll` with hidden `intake_id`; disable the button when `Already`.
- **CSS:** add `.roster-table`, `.search-panel`, `.rate-high/.rate-mid/.rate-low`, and any tab styles. Bump `?v=` to `?v=5`.

### 9. Edge cases

- **Duplicate enroll:** idempotent — existing `(event, intake)` enrollment is a no-op success.
- **Unenroll already-unenrolled:** treat as no-op success (record already `deleted=true` or absent).
- **Division by zero:** `TotalDays == 0` (event not started) → `Rate = 0`, render `0 / 0` or `—`.
- **Intake deleted after enrollment:** cascade deletes the enrollment (and attendance) via PB rules — roster query simply won't return it; no special handling needed.
- **Event deleted:** cascade deletes enrollments + attendance; `handleAdminEventManage` 404s on missing event.
- **Search scoping:** only intake records at the event's site are offered, so a participant can't be enrolled into an event at a different location (matches "seamless interaction with existing intake data").
- **`loadEnrolledCount` consistency:** must filter `deleted=false` so the admin events list "Enrolled" count matches the roster after unenrolls.
- **Soft-delete migration:** `008_event_enrollment_deleted.js` adds `deleted` (bool, default false) to `event_enrollment`. All existing rows default to `false` (enrolled). Update `loadEnrolledCount` and roster queries to filter `deleted=false`.

## Verification Criteria

1. **Build/vet/test gates:** `go build ./...`, `go vet ./...`, `go test ./...` all pass.
2. **Template parse:** `TestAdminEventsRender` (extended) parses the embedded template and renders `event-manage` with a populated roster without errors; the `enroll-search-results` fragment parses.
3. **Roster render test:** render `event-manage` with `Enrolled` rows and assert the output contains participant names, `DaysAttended / TotalDays`, rate badge text (e.g. `73%`), last-present date, and a Remove button per row.
4. **Enroll handler test:** with a fake/embedded PB, POST `/admin/events/{id}/enroll` with an `intake_id` creates exactly one `event_enrollment` record; a second identical POST does not create a duplicate (idempotency).
5. **Unenroll handler test:** POST `/admin/events/{id}/unenroll` sets `deleted=true` on the enrollment (soft delete) and does NOT delete the attendance records; a second unenroll is a no-op.
6. **Stats test:** given attendance records in the event range, `loadEnrollmentStats` returns the correct `DaysAttended`, `TotalDays`, `Rate`, and `LastPresent`; `TotalDays == 0` yields `Rate == 0` (no panic).
7. **Search test:** `handleEnrollSearch` returns intake records at the event's site matching the name query, marks already-enrolled hits with `Already=true`, and returns empty for `< 2` chars.
8. **Manual QA (per PRD §05 acceptance):** create an event → search and enroll 3 participants by name → roster shows them with correct days-attended/rate/last-present → remove one → it disappears from the roster but its attendance history remains → enrolled count in the admin events list decrements → the participant still appears in the matrix for that event's date range.
9. **Route check:** `POST /admin/events/{id}/enroll` and `/unenroll` are admin-only (non-admin gets 403); the catch-all `/admin/events/` still serves `/admin/events/{id}/manage`.
