# Working Plan: Replace Location Dropdown with Disabled Textbox on Attendance Matrix

## Objective

On the attendance matrix screen (`GET /attendance`, template block `matrix-content`), replace the admin-only site/location `<select>` filter with a **read-only, disabled textbox** that displays the location of the currently selected event.

The location box is no longer a filter — it is a display-only field. When an event is selected, the box shows that event's location. It must not be editable; editing an event's location happens on the admin event management screen. Non-admin users keep the existing muted `Site: {{.SiteName}}` span (their site is fixed by their session, not event-derived).

This card (`t_f07eca4c`) is the **UI change only**. A sibling card (`t_f3396ece`) separately removes the location-filter logic from the Go handler. This card's Go change is minimal: expose the selected event's location name to the template. Do **not** duplicate the sibling's filter-logic removal.

## Constraints

- **Language/runtime:** Go, server-rendered Go templates (`html/template`), embedded in a single HTML file.
- **Backend:** PocketBase v0.39 JS/Go API — no `app.dao()`. Use `s.pb.FindRecordById` / `s.nameFor` helpers.
- **Timezone:** All timestamps in HST (`hst` location).
- **Design system:** Public Sans + Lora fonts; accent `#b5502e`; card background `#fffdfa`; page background `#f7f1e6`; 14px card radii; 8px input radii. Reuse the existing `.field-input` class for the new textbox so it inherits the correct styling.
- **Scope boundary:** Do not remove the `site` query-param parsing or `resolveSite` logic — that is the sibling card's job. Only add the new display field and swap the template element.

## File Structure

| File | Action | Change |
|------|--------|--------|
| `r3-intake/internal/server/attendance.go` | Modify | Add `EventLocation string` field to `MatrixViewData`; populate it in `handleMatrix`. |
| `r3-intake/internal/assets/public/index.html` | Modify | In `matrix-content` block (~lines 1158–1182), replace the admin `<select name="site">` with a disabled `<input type="text">` bound to `.EventLocation`. |

No new files. No changes to walk-in forms (they use hidden `name="site_id"` bound to `.SiteID`, which is unaffected).

## Implementation Notes

### Key design decision: how the template gets the selected event's location

Add a new field to `MatrixViewData`:

```go
EventLocation string // display-only location of the selected event ("" when no event selected)
```

Populate it in `handleMatrix` after `loadEvents(siteID)` returns. Two viable resolution strategies:

1. **Match the selected event in the loaded events list (preferred).** `loadEvents` already returns `Event` records carrying `SiteID`. Iterate `events` to find the one whose `ID == eventID`, then resolve its `SiteID` to a name via `s.nameFor("sites", ev.SiteID)`. This avoids an extra DB round-trip and reuses the already-loaded list.
2. **Direct lookup.** If `eventID != ""`, call `s.nameFor("sites", <event's SiteID>)` — but this requires fetching the event record first, so strategy 1 is simpler and cheaper.

Edge cases for population:
- **No event selected** (`eventID == ""`): leave `EventLocation` empty. The template renders an empty disabled textbox (or a placeholder like "Select an event…"). The existing `{{if .EventRequired}}` empty-state message already covers this state.
- **Event found but its `SiteID` is empty** (event with no location): `nameFor` returns `""` for empty ids, so `EventLocation` is `""`. The disabled box shows empty — acceptable, and consistent with the "No Location" grouping already handled by `HasNoLocation`.
- **Event's site cascade-deleted:** `nameFor` returns `""` on failed lookup; box shows empty rather than erroring.

### Template change

Replace the admin-only `<select name="site">` block with a disabled textbox:

```html
{{if .IsAdmin}}
<input type="text" class="field-input" value="{{.EventLocation}}" disabled
       placeholder="Select an event to show its location">
{{else}}
<span class="muted">Site: {{.SiteName}}</span>
{{end}}
```

- `disabled` makes it read-only — the user cannot edit it. Editing happens on the admin event management screen.
- Because the form uses `hx-trigger="change, submit"` and re-renders `matrix-content` on event change, the box auto-updates to the newly selected event's location without extra JS.
- The `name="site"` attribute is **removed** from this element so it no longer submits a `site` query param. (The sibling card removes the server-side handling of that param; removing the attribute here is the UI-side counterpart and does not conflict — walk-in forms use `site_id`, not `site`.)
- Keep the `{{else}}` non-admin branch unchanged.

### Why this is safe

- Walk-in forms use hidden `name="site_id" value="{{.SiteID}}"` (lines ~1190, 1201, 1271, 1284) — these bind to the handler-resolved `SiteID`, not the dropdown, so removing the dropdown does not break them.
- `resolveSite` and the `site` query-param parsing remain untouched in this card (sibling's scope).

## Verification Criteria

1. **Build/vet/test:** `cd r3-intake && go build ./... && go vet ./... && go test ./...` — all pass.
2. **Admin, event selected:** `GET /attendance?event=<id>` renders a disabled textbox whose value equals the selected event's location name (verify against the `events.site` relation in PocketBase).
3. **Admin, no event selected:** `GET /attendance` renders a disabled, empty textbox (placeholder shown); the `EventRequired` empty-state message still appears.
4. **Admin, event with no location:** disabled textbox renders empty; no error.
5. **Non-admin:** still sees the muted `Site: {{.SiteName}}` span; no textbox.
6. **Read-only enforcement:** the textbox is `disabled` — cannot be typed into; no `name="site"` attribute is submitted on Apply.
7. **HTMX refresh:** changing the event `<select>` re-renders `matrix-content` and the textbox updates to the new event's location without a full page reload.
8. **Walk-in forms unaffected:** the walk-in panel's hidden `site_id` still carries the correct resolved `SiteID` after the change.
