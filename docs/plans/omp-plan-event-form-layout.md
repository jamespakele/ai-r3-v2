# Working Plan: Restructure event form layout

## Objective

Restructure the "Create Event" form on the admin screen so it uses screen real estate better. The form currently uses a 4-column grid (`form-grid-4`) that wastes horizontal space — each date field occupies a full column, the description is cramped, and the Create Event button sits on the same row as the description.

Target layout (2-column grid):

- **Row 1:** Event name (full width, spans both columns)
- **Row 2:** Location select (full width, spans both columns)
- **Row 3:** Start date + End date side by side (one column each)
- **Row 4:** Description (full width, spans both columns, larger — more rows)
- **Row 5:** Create Event button (full width, below the description)

This satisfies all three requirements:

1. **Start and End dates side by side** — paired on one row, one column each.
2. **Create Event button below the description box** — moved to its own row beneath the textarea.
3. **Description box bigger** — spans the full 2-column width, matching the width of the event name textbox, with more rows.

## Constraints

- **Language:** Go (server) + Go templates (server-rendered HTML) + vanilla CSS. No JS framework changes.
- **Framework:** Embedded PocketBase backend; HTMX + Alpine.js already present but **not** involved in this change.
- **Dependencies:** None new. Pure template + CSS change.
- **Scope boundary:** This is a child story card in an isolated worktree. Sibling card `t_30cf5ee4` owns the tabbed-layout conversion of the admin screen (Sites/Users/Events tabs). **Do NOT touch the accordion/tab structure** — only restructure the event form itself.
- **Backend contract:** Keep the form's `method="post"`, `action="/admin/events"`, and all field `name` attributes identical (`name`, `site`, `start_date`, `end_date`, `description`). **Do not change any Go code** — the Go handler must be unaffected.
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, 14px card radii, 8px input radii.

## File Structure

| File | Action | Notes |
|------|--------|-------|
| `r3-intake/internal/assets/public/index.html` | **Modify** | Change the event `<form>` markup (currently ~line 658) from `form-grid-4` to a 2-column grid; reorder fields; enlarge textarea. |
| `r3-intake/internal/assets/public/app.css` | **Modify** | Replace the `.form-grid-4` rules (lines ~242–249) with a 2-column grid class; keep responsive single-column fallback. |

No new files. No Go files touched.

## Implementation Notes

### Markup changes (`index.html`)

Replace the current `<form method="post" action="/admin/events" class="form-grid-4">` block with:

```html
<form method="post" action="/admin/events" class="form-grid-2">
  <input type="text" name="name" placeholder="Event name" required class="field-input grid-full" value="{{.EventName}}">
  <select name="site" class="field-input grid-full" required>
    <option value="">Select a location</option>
    {{range .Sites}}{{if .Active}}<option value="{{.ID}}" {{if eq .ID $.EventSite}}selected{{end}}>{{.Name}}</option>{{end}}{{end}}
  </select>
  <input type="date" name="start_date" placeholder="Start date" required class="field-input" value="{{.EventStart}}">
  <input type="date" name="end_date" placeholder="End date" required class="field-input" value="{{.EventEnd}}">
  <textarea name="description" placeholder="Description (optional)" class="grid-full textarea textarea-sm" rows="5" maxlength="500">{{.EventDescription}}</textarea>
  <button type="submit" class="btn btn-primary grid-full">Create Event</button>
</form>
```

Key decisions:

- **Class rename `form-grid-4` → `form-grid-2`** to reflect the new 2-column layout. `form-grid-4` is used only by this form (verified via grep), so renaming is safe and self-documenting.
- **Event name and Location each get `grid-full`** so they span both columns on their own rows. This gives the name field the full width and keeps the location select readable.
- **Start/End dates get no span class** — they naturally occupy one column each, sitting side by side on the same row.
- **Description keeps `grid-full`** (spans both columns = same width as the name textbox) and `rows` bumped from `3` to `5` for a larger box. `textarea-sm` (14px) retained for consistency.
- **Button keeps `grid-full`** so it sits on its own row below the description, full width.
- **Field order** is preserved (name, site, start_date, end_date, description) so the DOM/Go handler contract is unchanged.

### CSS changes (`app.css`)

Replace the `.form-grid-4` block (lines ~242–249) with:

```css
.form-grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin: 12px 0 24px; }
.form-grid-2 .field-input,
.form-grid-2 .textarea,
.form-grid-2 select { width: 100%; }
.form-error { color: #b3261e; font: 500 13px 'Public Sans', sans-serif; margin: 8px 0; }

@media (max-width: 620px) {
  .form-grid-2 { grid-template-columns: 1fr; }
}
```

Notes:

- `grid-template-columns: 1fr 1fr` gives two equal columns so the two date inputs pair up side by side.
- `.grid-full { grid-column: 1 / -1; }` (already defined at line 140) is reused to span full width — no new span utility needed.
- The `@media (max-width: 620px)` fallback collapses to a single column so dates stack on small screens, matching the existing responsive pattern used by `.grid-2`.
- `.form-error` rule is preserved (it was part of the same block).

### Edge cases

- **Responsive/small screens:** Below 620px the grid collapses to one column; dates stack vertically. No layout breakage.
- **HTMX/Alpine:** No HTMX attributes or Alpine bindings exist on this form; the change is purely presentational. No JS behavior to preserve.
- **Server rendering:** All `{{.Event*}}` template variables and `{{range .Sites}}` remain untouched — only class names and element order within the form change.
- **Sibling card conflict:** The tab/accordion wrapper around this form is owned by `t_30cf5ee4`. This card only edits the `<form>` element and its CSS class — no overlap.
- **Button placement:** Moving the button below the description is purely visual; the submit action and handler are unchanged.

## Verification Criteria

1. **Build passes:** `go build ./...` (or the project's build command) succeeds — no Go changes, so this is a sanity check only.
2. **Form renders:** Load the admin screen and confirm the event form shows:
   - Event name full-width on its own row.
   - Location select full-width on its own row.
   - Start date and End date side by side on the same row.
   - Description spanning the full width (same width as the name field) with 5 rows.
   - Create Event button on its own row below the description.
3. **Field contract intact:** Inspect the rendered HTML — `method="post"`, `action="/admin/events"`, and `name` attributes (`name`, `site`, `start_date`, `end_date`, `description`) are unchanged.
4. **Create Event still works:** Submit the form and confirm an event is created (Go handler unaffected).
5. **Responsive check:** Narrow the viewport below 620px — all fields stack into a single column without overflow.
6. **No regressions:** Grep confirms no remaining references to `form-grid-4` in the codebase.
