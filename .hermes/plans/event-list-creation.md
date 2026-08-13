# Working Plan: Event List and Creation (Story 2.1)

## Objective

Implement the backend endpoints and UI components to **create multi-week outreach programs (events)** and **display the events list** in the admin settings page. An admin navigates to `/admin`, sees a new **"Events"** accordion section alongside "Sites" and "Users", expands it to view a table of all events (with status badges and actions), and uses a "+ New Event" form to create an event. This story covers list + create only; the enrollment screen (`/admin/events/{id}/manage`) is a stub/placeholder for Story 2.2.

## Constraints

- **Go is the policy layer.** All PocketBase data access goes through `s.pb` in-process; the browser never talks to PB directly. The `events` collection rules are `null` (locked) — enforcement happens in Go.
- **Events collection already exists** via `r3-intake/pocketbase/migrations/007_events_attendance.js`. Fields: `site` (relation, required), `name` (text, required, max 200), `start_date` (text, required, max 20), `end_date` (text, required, max 20), `description` (text, optional, max 500), `status` (select: active/completed/cancelled, required), `created_by` (relation to users, optional), `created`/`updated` autodates. **No migration changes needed.**
- **Admin-only.** The Events section and create mutation are gated by `requireRole("admin", ...)` / `u.Role == "admin"` checks, matching the Sites/Users patterns.
- **`loadEvents(siteID)` currently returns only `status='active'`.** The admin list must show **all** events (active, completed, cancelled) — so the admin list needs a new loader (or a parameter) that does **not** filter by status. The existing `loadEvents` (active-only) stays for the matrix filter.
- **Enrolled count** is derived from the `event_enrollment` junction collection (count of records where `event = <id>`). This is a read-only count for display; no enrollment writes in this story.
- **Filter escaping:** use `mcpmod.EscapeFilter(s)` (import `mcpmod "r3-intake/internal/mcp"`) for any user-supplied value interpolated into a PB filter string.
- **Templates:** single embedded `index.html` with multiple `{{define}}` blocks; new blocks go at the end. Reference via `s.tpl.ExecuteTemplate(w, "name", view)`.
- **Time zone:** all timestamps display in HST via the existing `var hst = time.FixedZone("HST", -10*60*60)` in `r3-intake/internal/server/admin.go`. Dates are stored as plain `YYYY-MM-DD` text (no timezone conversion needed for date-only fields).
- **Verification gates:** `go build ./...`, `go vet ./...`, `go test ./...` must pass; template parse must succeed (all defines registered).

## File Structure

All paths relative to repo root `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_9c800987`.

| File | Change |
|------|--------|
| `r3-intake/internal/server/admin.go` | **Extend.** Add `EventRow` struct, `Events []EventRow` field on `AdminView`, `loadAllEvents()` + `loadEnrolledCount()` helpers, `adminEventAdd` handler, and a new `handleAdminEventManage` stub. Wire the create mutation into `handleAdminSub`. |
| `r3-intake/internal/server/server.go` | **Add routes.** Register `/admin/events/{id}/manage` (admin-only) and the `/admin/events` create POST (via existing `handleAdminSub`). |
| `r3-intake/internal/assets/public/index.html` | **Extend.** Add an "Events" accordion section to the `{{define "admin"}}` template (table + "+ New Event" form). Add a `{{define "event-manage"}}` placeholder template. Bump the `?v=` cache-buster on the stylesheet link. |
| `r3-intake/internal/assets/public/app.css` | **Add.** Status badge variants for event statuses (active/completed/cancelled) and a 4-column form grid class. |
| `r3-intake/pocketbase/migrations/007_events_attendance.js` | **No change** (collections already exist). Reference only. |

## Implementation Notes

### 1. Data model — `r3-intake/internal/server/admin.go`

Add an `EventRow` struct (flat view for the admin table) and a new field on `AdminView`:

```go
type EventRow struct {
    ID          string
    Name        string
    SiteName    string
    StartDate   string // YYYY-MM-DD
    EndDate     string // YYYY-MM-DD
    Enrolled    int    // count from event_enrollment
    Status      string // active | completed | cancelled
}
```

Add `Events []EventRow` to `AdminView` (alongside `Sites` and `Users`).

### 2. Loaders — `r3-intake/internal/server/admin.go`

- **`loadAllEvents() ([]EventRow, error)`** — new loader that returns **all** events regardless of status (unlike `loadEvents` in `attendance.go` which filters `status='active'`). Query `eventsCollection()` with filter `1=1`, sort `-start_date,name`, limit 1000. For each record, resolve `SiteName` via `s.siteNameMap()` (fallback `"—"`), and set `Enrolled` via `loadEnrolledCount(rec.Id)`.
- **`loadEnrolledCount(eventID string) int`** — query the `event_enrollment` collection with filter `event='<escaped id>'`, limit 1000, and return `len(recs)`. (Count is small; a full fetch is acceptable and matches existing patterns. No pagination needed for this story.)
- **`handleAdminSettings`** — add `view.Events = must(s.loadAllEvents())` after the existing `view.Sites` / `view.Users` assignments.

### 3. Create mutation — `r3-intake/internal/server/admin.go`

Add `adminEventAdd(w, r, u *sessionUser)`:

- Read form values: `name`, `site`, `start_date`, `end_date`, `description`.
- **Validation** (all required): `name` non-empty (trim), `site` non-empty, `start_date` and `end_date` parse as `2006-01-02` via `time.Parse`. If any required field is missing/invalid, re-render the admin page with a validation error (see §5) instead of creating.
- **Date sanity:** if both dates parse and `end_date < start_date`, treat as a validation error (swap is NOT acceptable for events — a program must have a valid range).
- **Description:** optional; trim; enforce `max 500` chars (truncate or reject — prefer reject with a message, matching the collection's `max: 500`).
- **Create:** `rec := core.NewRecord(col)`; set `name`, `site`, `start_date`, `end_date`, `description`, `status = "active"`, `created_by = u.ID` (the current session user). `_ = s.pb.Save(rec)`.
- **Redirect:** `http.Redirect(w, r, "/admin", http.StatusSeeOther)` so the page reloads and shows the new event in the list (matches `adminSiteAdd`).

Wire into `handleAdminSub` switch (admin-only):

```go
case path == "events" && u.Role == "admin":
    s.adminEventAdd(w, r, u)
```

### 4. Routes — `r3-intake/internal/server/server.go`

- The create POST is handled by the existing `mux.HandleFunc("/admin/", s.requireAuth(s.handleAdminSub))` — no new route needed for create (the `handleAdminSub` switch dispatches it).
- Add the manage stub route (admin-only), next to the `/admin` registration:

```go
mux.HandleFunc("/admin/events/", s.requireRole("admin", s.handleAdminEventManage))
```

`handleAdminEventManage` (in `admin.go`) parses the event ID from the path, loads the event name (via `s.pb.FindRecordById("events", id)`), and renders the `{{define "event-manage"}}` placeholder template. It is a stub for Story 2.2 — it should render a simple page with the event name and a "Back to Admin" link, plus a note that enrollment management is coming in a later story. If the event is not found, `http.NotFound`.

### 5. Template — `r3-intake/internal/assets/public/index.html`

**Events accordion** (insert after the Users accordion, inside the `{{if .IsAdmin}}` block in `{{define "admin"}}`):

- Accordion header: `<span class="accordion-title">Events</span>` with the same chevron SVG + toggle `onclick` used by Sites/Users.
- Table with columns: **Event Name, Location, Dates, Enrolled, Status, Actions**.
  - **Dates:** render as `{{.StartDate}} – {{.EndDate}}` (en dash).
  - **Status badge:** `<span class="status-badge status-{{.Status}}">{{.Status}}</span>` — reuses the existing `.status-badge` base class; add color variants in CSS (see §6).
  - **Actions:** two buttons per row:
    - **Manage** → `<a class="btn btn-tiny" href="/admin/events/{{.ID}}/manage">Manage</a>` (enrollment screen stub).
    - **Matrix** → `<a class="btn btn-tiny" href="/attendance?event={{.ID}}">Matrix</a>` — the matrix already reads `?event=` and pre-selects it in the filter (`{{if eq $.EventID .ID}}selected{{end}}` in the `matrix-content` template).
  - Empty state row: `{{if not .Events}}<tr><td colspan="6" class="empty-state">No events yet.</td></tr>{{end}}`.
- **"+ New Event" form** (below the table, `class="inline-form"` or a new grid class):
  - A **4-column grid** (Event Name text, Location select, Start Date date input, End Date date input) plus a full-width **Description** textarea (optional, `maxlength="500"`), and a **Create Event** button (`class="btn btn-primary"`).
  - Location select populated from **active sites only**: `{{range .Sites}}{{if .Active}}<option value="{{.ID}}">{{.Name}}</option>{{end}}{{end}}`. (Note: `view.Sites` in `handleAdminSettings` is loaded with `loadSites(true)` — includeInactive — so the template must filter `.Active` to show only active sites in the select.)
  - Form `method="post" action="/admin/events"`.
  - **Validation error display:** add an `EventError string` field to `AdminView`. When set, render a `.form-error`/`.alert` message above the form (e.g. `{{if .EventError}}<div class="form-error">{{.EventError}}</div>{{end}}`). On validation failure, `adminEventAdd` re-renders the `admin` template with `view.EventError` set and the previously-entered values preserved (repopulate the form fields from the submitted values so the user doesn't lose input).

**`{{define "event-manage"}}`** — new placeholder template at the end of the file: topbar (reuse `topbar-admin`), a heading with the event name, a "Back to Admin" link, and a muted note that enrollment management ships in a later story.

### 6. CSS — `r3-intake/internal/assets/public/app.css`

- **Status badge variants** (mirror the existing `.status-*` colors):
  - `.status-active { background: #eef3ea; color: #3f6b34; }` (green)
  - `.status-completed { background: #fdf3e3; color: #8a6a1e; }` (yellow/amber)
  - `.status-cancelled { background: #fbeeec; color: #8f3a2e; }` (red)
  - (These match the existing palette used by `.status-unassigned`/`.status-claimed`/`.status-completed`.)
- **Form grid:** add `.form-grid-4 { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }` with a responsive collapse to 1 column on small screens (mirror the existing `@media (max-width: 620px)` pattern). The description textarea spans full width (`.grid-full`).
- **Error message:** add `.form-error { color: #b3261e; font: 500 13px 'Public Sans', sans-serif; margin: 8px 0; }`.
- Bump the `?v=` cache-buster on the stylesheet link in the admin template (e.g. `?v=4`) so the new CSS is fetched.

### 7. Edge cases

- **`loadEvents` vs `loadAllEvents`:** do NOT reuse `loadEvents` for the admin list — it filters `status='active'` and would hide completed/cancelled events. The admin list must show all statuses.
- **Active sites only in the select:** `view.Sites` is loaded with `includeInactive=true` for the Sites table; the Events form select must filter `.Active` in the template so inactive sites can't be chosen.
- **Validation preserves input:** on a validation error, re-render with the submitted `name`/`site`/`start_date`/`end_date`/`description` values so the user can correct rather than retype.
- **End < start:** reject (do not silently swap) — a program must have a valid date range.
- **Description length:** enforce `max 500` in Go (reject with a message) in addition to the HTML `maxlength="500"`.
- **Enrolled count:** read-only; computed from `event_enrollment`. If the junction collection is missing/errors, default to 0 (don't fail the whole page).
- **Manage stub:** `/admin/events/{id}/manage` must 404 if the event doesn't exist; otherwise render the placeholder. No enrollment logic in this story.

## Verification Criteria

1. **Build/vet/test:** `go build ./...`, `go vet ./...`, `go test ./...` all pass from the repo root.
2. **Template parse:** the server starts and the `admin` template parses (all `{{define}}` blocks registered, including the new `event-manage`).
3. **Admin page renders:** navigating to `/admin` as an admin shows the "Events" accordion alongside "Sites" and "Users"; expanding it shows the table with columns Event Name, Location, Dates, Enrolled, Status, Actions.
4. **Status badges:** active events show a green "Active" badge, completed show yellow "Completed", cancelled show red "Cancelled".
5. **Create flow:** clicking "+ New Event" shows the 4-column grid (Event Name, Location select of active sites, Start Date, End Date) + Description (max 500) + "Create Event" button. Submitting with all required fields creates a record with `status="active"` and `created_by=<current user>`, then redirects to `/admin` where the new event appears in the list.
6. **Validation:** submitting with a missing required field (or invalid/end-before-start dates) shows a validation error and preserves the entered values; no record is created.
7. **Actions:** "Manage" navigates to `/admin/events/{id}/manage` (stub renders); "Matrix" navigates to `/attendance?event={id}` with that event pre-selected in the filter dropdown.
8. **Enrolled count:** the Enrolled column shows the count of `event_enrollment` records for each event (0 for new events).
