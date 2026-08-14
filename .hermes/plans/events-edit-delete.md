# Working Plan

## Objective

Add **Edit** and **Delete** actions to the admin **Events** tab in the R3 Intake Go app:

1. **Edit** — per event row, an Edit button that toggles an inline edit form (mirroring the existing Users-tab pattern) posting to `POST /admin/events/{id}/update` with fields `name`, `site`, `start_date`, `end_date`, `description`. The form must pre-fill the site `<select>` (selected option) and the description `<textarea>`.
2. **Delete** — per event row, a Delete button that shows a `confirm()` dialog and posts to `POST /admin/events/{id}/delete` (soft-delete).

The backend handlers already exist on the sibling branch (`t_dc1890be`) and are **not** in this worktree. This card implements **only** the UI plus the `EventRow` struct additions needed to pre-fill the edit form. No handler code.

## Constraints

- **No backend handler work** — `/admin/events/{id}/update` and `/admin/events/{id}/delete` already exist (registered under `csrfMiddleware` + `requireRole`). The UI must simply target those routes.
- **Minimal, focused changes** — no drive-by refactors. Only `internal/server/admin.go` (struct + loader) and `internal/assets/public/index.html` (Events tab UI).
- **Mirror the Users-tab inline-edit pattern** — a hidden `<tr id="edit-...">` toggled by an Edit button, containing a `<form>` posting to the update route.
- **Conventions (AGENTS.md)** — PocketBase v0.39 JS API, HST timezone, Public Sans + Lora accent `#b5502e`. Templates use `{{define}}` blocks in `index.html`.
- **Soft-delete** — `deleted=true`; `loadAllEvents()` already excludes `deleted=true`, so deleted events disappear from the list automatically after the 303 redirect.
- **CSRF** — forms must be `method="post"` (server-rendered, no JS fetch), consistent with existing forms; the CSRF middleware handles token injection.

## File Structure

| File | Change |
|------|--------|
| `r3-intake/internal/server/admin.go` | Add `SiteID` and `Description` fields to `EventRow`; populate them in `loadAllEvents()`. |
| `r3-intake/internal/assets/public/index.html` | Add Edit + Delete UI to the Events tab table (`panel-events`). |

No new files. No handler changes.

## Implementation Notes

### 1. `internal/server/admin.go` — extend `EventRow` and `loadAllEvents()`

**`EventRow` struct** (currently lines ~60-67) — add two fields:

```go
type EventRow struct {
	ID          string
	Name        string
	SiteID      string // raw site record ID, for pre-selecting the edit <select>
	SiteName    string
	StartDate   string
	EndDate     string
	Description string // raw description, for pre-filling the edit <textarea>
	Enrolled    int
	Status      string
}
```

**`loadAllEvents()`** (currently lines ~864-895) — populate the new fields. The raw site ID is `r.GetString("site")`; the description is `r.GetString("description")`:

```go
out = append(out, EventRow{
	ID:          r.Id,
	Name:        r.GetString("name"),
	SiteID:      r.GetString("site"),
	SiteName:    site,
	StartDate:   r.GetString("start_date"),
	EndDate:     r.GetString("end_date"),
	Description: r.GetString("description"),
	Enrolled:    s.loadEnrolledCount(r.Id),
	Status:      r.GetString("status"),
})
```

> Note: `siteMap` is keyed by site ID, so `SiteID` is simply the raw `site` field value — no extra lookup needed. The existing `site == ""` fallback to `"—"` for `SiteName` stays as-is.

### 2. `internal/assets/public/index.html` — Events tab UI (`panel-events`)

**Edit + Delete buttons** — add to the existing actions cell (currently lines ~768-781), alongside Manage/Matrix/Report. Toggle the hidden edit row, mirroring the Users-tab `onclick` pattern:

```html
<td class="admin-actions">
  <a href="/admin/events/{{.ID}}/manage" class="btn btn-tiny">Manage</a>
  <a href="/attendance?event={{.ID}}" class="btn btn-tiny">Matrix</a>
  {{if eq .Status "completed"}}<a href="/admin/events/{{.ID}}/report" class="btn btn-tiny">Report</a>{{end}}
  <button type="button" class="btn btn-tiny" onclick="document.getElementById('edit-event-{{.ID}}').style.display=document.getElementById('edit-event-{{.ID}}').style.display==='none'?'table-row':'none'">Edit</button>
  <button type="button" class="btn btn-tiny btn-danger" onclick="if(confirm('Delete this event?')){document.getElementById('delete-event-{{.ID}}').submit()}">Delete</button>
</td>
```

**Hidden edit row** — insert immediately after the event `<tr>`, before the `{{end}}` of the range. Uses `colspan="6"` (6 columns in the events table). The site `<select>` iterates `.Sites` and marks the option whose `.ID` equals `$.SiteID` as selected; the description `<textarea>` is pre-filled with `{{.Description}}`:

```html
<tr id="edit-event-{{.ID}}" style="display:none">
  <td colspan="6">
    <form method="post" action="/admin/events/{{.ID}}/update" class="form-grid-2">
      <input type="text" name="name" value="{{.Name}}" placeholder="Event name" required class="field-input grid-full">
      <select name="site" class="field-input grid-full" required>
        <option value="">Select a location</option>
        {{range $.Sites}}{{if .Active}}<option value="{{.ID}}" {{if eq .ID $.SiteID}}selected{{end}}>{{.Name}}</option>{{end}}{{end}}
      </select>
      <input type="date" name="start_date" value="{{.StartDate}}" required class="field-input">
      <input type="date" name="end_date" value="{{.EndDate}}" required class="field-input">
      <textarea name="description" placeholder="Description (optional)" class="grid-full textarea textarea-sm" rows="5" maxlength="500">{{.Description}}</textarea>
      <button type="submit" class="btn btn-primary grid-full">Save Event</button>
    </form>
  </td>
</tr>
```

**Hidden delete form** — one per row, used by the Delete button's `confirm()`:

```html
<form id="delete-event-{{.ID}}" method="post" action="/admin/events/{{.ID}}/delete" style="display:none"></form>
```

**Template-scope notes:**
- Inside `{{range .Events}}`, the range variable is `.` (the `EventRow`). Use `$.Sites` for the sites list (root `AdminView.Sites`).
- In the nested `{{range $.Sites}}`, the inner range shadows `.` with the `Site` value, so `.ID` there is the site's ID. Use `$.SiteID` for the row's site ID (root scope). Correct form: `{{if eq .ID $.SiteID}}selected{{end}}`. This mirrors the existing create form which uses `{{if eq .ID $.EventSite}}selected{{end}}`.

**Delete button placement** — the `confirm()` + submit approach keeps it a plain `method="post"` form (CSRF-safe), no JS fetch needed. The hidden form must be inside the `{{range .Events}}` block so `{{.ID}}` resolves per row.

**Empty-state colspan** — the existing `{{if not .Events}}<tr><td colspan="6" ...>No events yet.</td></tr>{{end}}` already uses `colspan="6"`; no change needed.

## Verification Criteria

1. **Build & static checks** (from `r3-intake/`):
   ```
   go build ./...
   go vet ./...
   go test ./...
   ```
   All must pass cleanly.

2. **Struct/loader correctness** — `EventRow` has `SiteID` and `Description`; `loadAllEvents()` populates both from `r.GetString("site")` and `r.GetString("description")`.

3. **Template correctness** — `index.html` parses without template errors (verified by `go build`/`go test` since the template is embedded and rendered in tests). The site `<select>` uses `$.SiteID` (root scope) for the selected option; the description `<textarea>` is pre-filled with `{{.Description}}`.

4. **UI behavior (manual)** — on the admin Events tab:
   - Each row shows **Edit** and **Delete** buttons alongside Manage/Matrix/Report.
   - Clicking **Edit** toggles the hidden `edit-event-{id}` row; the form pre-selects the event's site and pre-fills name/dates/description.
   - Submitting the edit form posts to `POST /admin/events/{id}/update` and (on success) 303-redirects to `/admin?tab=events`.
   - Clicking **Delete** shows a `confirm()` dialog; confirming submits the hidden form to `POST /admin/events/{id}/delete`; the event disappears from the list (soft-delete, `deleted=true` excluded by `loadAllEvents()`).

5. **No scope creep** — no backend handler changes; only `admin.go` (struct + loader) and `index.html` (Events tab) are modified.
