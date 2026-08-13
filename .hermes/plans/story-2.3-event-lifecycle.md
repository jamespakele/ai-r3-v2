# Working Plan: Event Lifecycle and Status Transitions (Story 2.3)

## Objective

Implement the event lifecycle backend logic and UI controls so an admin can transition an event's status through the valid path **active → completed** and **active → cancelled**, with server-side validation that rejects any invalid transition. Once an event is `completed` or `cancelled`, it becomes read-only (enrollment and status-change actions disabled) and a **Report** button appears in the events list for `completed` events.

This satisfies **FR13** (event lifecycle/status management) and **UX-DR8** (read-only terminal states + confirmation on destructive cancel). The browser never talks to PocketBase directly — all policy lives in the Go server layer.

### Acceptance criteria covered
1. `active` + "Complete" → status `completed`, badge turns yellow ("Completed"), event stays in list & matrix.
2. `active` + "Cancel" → status `cancelled`, with a JS confirm prompt **"Mark this event as cancelled?"** before the change.
3. `completed`/`cancelled` → enrollment + status-change actions disabled (read-only); **Report** button appears in events list for `completed` events.
4. Non-admin (`role != "admin"`) attempting a status change → forbidden (redirect to `/` or 403).

## Constraints

- **Language:** Go 1.25 (`go 1.25.0` in `go.mod`).
- **Framework:** net/http `http.ServeMux` (Go 1.22+ method+wildcard patterns available), embedded PocketBase v0.39 (`s.pb`), server-rendered Go `html/template` with multiple `{{define}}` blocks in a single embedded `index.html`.
- **Frontend:** HTMX + Alpine.js, vanilla CSS. No JS build step.
- **Dependencies:** existing only — `mcpmod "r3-intake/internal/mcp"` for `EscapeFilter`; `core "github.com/pocketbase/pocketbase/core"`. No new deps.
- **Auth:** `requireRole("admin", h)` for route gating; `currentSession(r)` returns `*sessionUser` (nil if unauthenticated); `u.Role == "admin"` for in-handler checks.
- **Time:** all timestamps HST via `hst = time.FixedZone("HST", -10*60*60)` (already in `admin.go`).
- **Data model:** `events` collection field `status` is a select of `active/completed/cancelled` (required). All PB rules are null — Go is the policy layer.
- **Verification gates:** `go build ./...`, `go vet ./...`, `go test ./...` must all pass; template parse must succeed.

## File Structure

All paths relative to the repo root `r3-intake/`.

| File | Action | Notes |
|------|--------|-------|
| `internal/server/admin.go` | **Modify** | Add `EventID`, `EventStatus`, `EventStatusError`, `EventEnrolled` fields to `AdminView`; add `handleEventStatus` handler; add `validEventTransition` helper; add `renderEventManage` shared render helper; refactor `handleAdminEventManage` to use it. |
| `internal/server/server.go` | **Modify** | Register `POST /admin/events/{id}/status` route (admin-only); add stub `GET /admin/events/{id}/report` route. |
| `internal/assets/public/index.html` | **Modify** | Update `{{define "admin"}}` events table (Report button for completed); rewrite `{{define "event-manage"}}` with status controls + read-only state + confirm dialog; add `{{define "event-report"}}` stub. |
| `internal/assets/public/app.css` | **Modify** | Add `.event-manage-actions`, `.inline-form`, `.event-readonly` styles (badge variants already exist). |
| `internal/server/admin_events_test.go` | **Modify** | Extend `TestAdminEventsRender` for new manage template fields; add `TestEventStatusTransition` (unit) and `TestEventStatusRoute` (handler-level). |

## Implementation Notes

### 1. Route registration (`server.go`)

Add after the existing event-manage registration (line ~115):

```go
// Event status transitions — admin only
mux.HandleFunc("POST /admin/events/{id}/status", s.requireRole("admin", s.handleEventStatus))
// Report stub (Epic 3 CSV export) — admin only
mux.HandleFunc("GET /admin/events/{id}/report", s.requireRole("admin", s.handleEventReport))
```

Use the Go 1.22+ method+wildcard pattern so `GET` to the status path is not matched. The existing `mux.HandleFunc("/admin/events/", ...)` for `handleAdminEventManage` stays as-is (GET manage). Note: `handleAdminEventManage` currently guards `r.Method != http.MethodGet` itself — keep that.

### 2. View model additions (`admin.go`)

Add to `AdminView`:

```go
EventID           string // id of the event being managed
EventStatus       string // current status of the event being managed
EventStatusError  string // validation/transition error message
EventEnrolled     int    // enrolled count for the managed event
```

### 3. Status transition handler (`admin.go`)

```go
// validEventTransition reports whether moving from -> to is a legal lifecycle step.
func validEventTransition(from, to string) bool {
    switch from {
    case "active":
        return to == "completed" || to == "cancelled"
    default:
        return false // completed/cancelled are terminal; no transitions out
    }
}

// handleEventStatus applies a status change to an event. Only active events may
// transition, and only to completed or cancelled. Admin-only (route-gated).
func (s *Server) handleEventStatus(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    rec, err := s.pb.FindRecordById("events", id)
    if err != nil {
        http.NotFound(w, r)
        return
    }
    u := s.currentSession(r)
    if u == nil || u.Role != "admin" {
        http.Redirect(w, r, "/", http.StatusSeeOther) // AC4: forbidden
        return
    }
    to := strings.TrimSpace(r.FormValue("status"))
    from := rec.GetString("status")
    if !validEventTransition(from, to) {
        // Re-render manage page with error; do NOT mutate.
        s.renderEventManage(w, r, u, rec, "Invalid status transition: "+from+" → "+to)
        return
    }
    rec.Set("status", to)
    if err := s.pb.Save(rec); err != nil {
        s.renderEventManage(w, r, u, rec, "Could not update event status.")
        return
    }
    http.Redirect(w, r, "/admin/events/"+id+"/manage", http.StatusSeeOther)
}
```

**Key decisions:**
- **Validation is server-side and authoritative.** The UI disables buttons for terminal states, but the handler re-validates `validEventTransition` regardless — a crafted POST cannot move `completed → active` or `cancelled → completed`. This is the core of "validation prevents invalid state transitions."
- **`to` is whitelisted** by the switch in `validEventTransition`; any unknown value (e.g. `"deleted"`) is rejected.
- **Non-admin** is double-guarded: route-level `requireRole("admin", ...)` plus an in-handler `u.Role != "admin"` check (defense in depth, matching the existing `adminClaim`/`adminComplete` pattern). Redirect to `/` per AC4.
- **Redirect back to the manage page** on success so the updated badge/read-only state is visible. On validation error, re-render the manage template with `EventStatusError` (no redirect, no mutation).

### 4. Refactor `handleAdminEventManage` → shared render helper

Extract the manage-page rendering so both the GET handler and the error path in `handleEventStatus` can use it:

```go
func (s *Server) renderEventManage(w http.ResponseWriter, r *http.Request, u *sessionUser, rec *core.Record, statusErr string) {
    view := &AdminView{
        UserName:         u.Name,
        Role:             u.Role,
        IsAdmin:          u.Role == "admin",
        EventID:          rec.Id,
        EventName:        rec.GetString("name"),
        EventStatus:      rec.GetString("status"),
        EventEnrolled:    s.loadEnrolledCount(rec.Id),
        EventStatusError: statusErr,
    }
    _ = s.tpl.ExecuteTemplate(w, "event-manage", view)
}
```

`handleAdminEventManage` becomes: GET-only guard → load rec → `u := s.currentSession(r)`; if nil redirect `/login` → call `renderEventManage(w, r, u, rec, "")`.

### 5. Report stub handler (`admin.go`)

```go
// handleEventReport renders a placeholder report page (CSV export ships in Epic 3).
func (s *Server) handleEventReport(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    rec, err := s.pb.FindRecordById("events", id)
    if err != nil {
        http.NotFound(w, r)
        return
    }
    u := s.currentSession(r)
    if u == nil {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }
    view := &AdminView{UserName: u.Name, Role: u.Role, IsAdmin: u.Role == "admin",
        EventID: rec.Id, EventName: rec.GetString("name"), EventStatus: rec.GetString("status")}
    _ = s.tpl.ExecuteTemplate(w, "event-report", view)
}
```

### 6. Template changes (`index.html`)

**`{{define "admin"}}` events table** — add a Report button for completed events in the actions cell (line ~647):

```html
<td class="admin-actions">
  <a href="/admin/events/{{.ID}}/manage" class="btn btn-tiny">Manage</a>
  <a href="/attendance?event={{.ID}}" class="btn btn-tiny">Matrix</a>
  {{if eq .Status "completed"}}<a href="/admin/events/{{.ID}}/report" class="btn btn-tiny">Report</a>{{end}}
</td>
```

**`{{define "event-manage"}}`** — replace the placeholder body with:

```html
<div class="container container-admin">
  <h2 class="section-title">Manage Enrollment — {{.EventName}}</h2>
  <p class="intro">Status: <span class="status-badge event-status-{{.EventStatus}}">{{.EventStatus}}</span>
     · Enrolled: {{.EventEnrolled}}</p>

  {{if .EventStatusError}}<div class="form-error">{{.EventStatusError}}</div>{{end}}

  {{if eq .EventStatus "active"}}
  <div class="event-manage-actions">
    <form method="post" action="/admin/events/{{.EventID}}/status" class="inline-form">
      <input type="hidden" name="status" value="completed">
      <button type="submit" class="btn btn-primary">Complete</button>
    </form>
    <form method="post" action="/admin/events/{{.EventID}}/status" class="inline-form"
          onsubmit="return confirm('Mark this event as cancelled?');">
      <input type="hidden" name="status" value="cancelled">
      <button type="submit" class="btn btn-tiny btn-danger">Cancel</button>
    </form>
  </div>
  {{else}}
  <div class="event-readonly">
    <p class="intro">This event is <strong>{{.EventStatus}}</strong> and is read-only. Enrollment and status changes are disabled.</p>
  </div>
  {{end}}

  <p class="intro">Enrollment management will be added in the next story.</p>
  <a href="/admin" class="btn btn-ghost">Back to Admin</a>
</div>
```

**`{{define "event-report"}}`** — new stub block at the end of the file:

```html
{{define "event-report"}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>R3 Intake — Event Report</title>
<link href="https://fonts.googleapis.com/css2?family=Public+Sans:wght@400;500;600;700&family=Lora:wght@500;600&display=swap" rel="stylesheet">
<link rel="stylesheet" href="/static/app.css?v=5">
</head>
<body>
<div class="page-bg">
  <div class="topbar topbar-admin" data-no-print>
    <div class="topbar-inner">
      <div class="brand"><div class="brand-badge">R3</div>
        <div><div class="brand-title">Restore, Reconnect, Revive</div>
        <div class="brand-sub">Event Report — {{.UserName}} ({{.Role}})</div></div>
      </div>
      <div class="topbar-actions">
        <a href="/admin" class="btn btn-ghost">Back to Admin</a>
        <a href="/logout" class="btn btn-ghost">Sign out</a>
      </div>
    </div>
  </div>
  <div class="container container-admin">
    <h2 class="section-title">Report — {{.EventName}}</h2>
    <p class="intro">Status: <span class="status-badge event-status-{{.EventStatus}}">{{.EventStatus}}</span></p>
    <p class="intro">CSV export will be added in Epic 3.</p>
    <a href="/admin" class="btn btn-ghost">Back to Admin</a>
  </div>
</div>
</body>
</html>{{end}}
```

**Important:** the manage template needs the event ID — `EventID` field on `AdminView`, set in `renderEventManage` (`rec.Id`). The confirm dialog uses inline `onsubmit` (no Alpine needed) — matches the "confirmation prompt appears before the change" AC.

### 7. CSS (`app.css`)

Add:
```css
.event-manage-actions { display: flex; gap: 10px; margin: 16px 0; align-items: center; }
.inline-form { display: inline; margin: 0; }
.event-readonly { margin: 16px 0; padding: 12px 16px; background: #fdf3e3; border: 1px solid #e6d3a8; border-radius: 8px; }
```
Badge variants `.event-status-active/completed/cancelled` already exist (lines 235–237) — no change needed.

### 8. Edge cases

- **Terminal-state POST:** `completed`/`cancelled` events reject any transition via `validEventTransition` (returns false for `from` not `active`). UI hides buttons; server enforces.
- **Same-state POST:** `active → active` is rejected (not in the whitelist). Harmless but blocked.
- **Unknown target status:** rejected by whitelist.
- **Nonexistent event ID:** `FindRecordById` error → 404.
- **Non-admin POST:** route gate redirects to `/login`; in-handler check redirects to `/`. AC4 satisfied.
- **Missing/empty `status` form value:** `to == ""` → rejected by `validEventTransition`.
- **Event remains in list after completion:** `loadAllEvents()` already returns all statuses (no filter change needed). **Matrix:** `loadEvents` filters `status='active'` — keep matrix active-only (completed events are read-only; enrollment disabled). Flag to parent if completed events must appear in matrix.
- **Template parse:** new `{{if eq .Status "completed"}}` and `{{.EventID}}`/`{{.EventStatus}}`/`{{.EventEnrolled}}` fields must exist on `AdminView`/`EventRow` or parse/render fails — the test guards this.

## Verification Criteria

1. **Build/vet/test gates:** `go build ./...`, `go vet ./...`, `go test ./...` all pass from `r3-intake/`.
2. **Template parse:** `TestAdminEventsRender` (extended) parses the embedded template and renders `admin` + `event-manage` + `event-report` without error.
3. **Unit test `TestEventStatusTransition`:** table-driven over `validEventTransition`:
   - `active→completed` = true, `active→cancelled` = true
   - `completed→active` = false, `completed→cancelled` = false, `cancelled→completed` = false, `cancelled→active` = false, `active→active` = false, `active→""` = false, `active→"deleted"` = false
4. **Handler test `TestEventStatusRoute`:** with a fake `Server` (or minimal PB stub), assert:
   - valid `active→completed` POST saves status and redirects to manage page;
   - invalid `completed→active` POST does **not** call Save and re-renders manage with `EventStatusError`;
   - non-admin session → redirect to `/` (or 403) and no Save.
5. **Render assertions (extend `TestAdminEventsRender`):**
   - `admin` output for a `completed` event contains `Report` link; for `active` event does **not**.
   - `event-manage` output for `active` event contains `Complete` and `Cancel` buttons and the confirm string `Mark this event as cancelled?`.
   - `event-manage` output for `completed`/`cancelled` event contains the read-only notice and **no** `Complete`/`Cancel` buttons.
   - `event-manage` output with `EventStatusError` set shows the error text.
6. **Manual smoke (optional):** run server, log in as admin, create event, Complete → badge yellow "Completed", Report button appears, buttons disabled; Cancel on an active event → confirm prompt → status "cancelled", read-only. Log in as case_manager → no status controls / redirect on POST.
