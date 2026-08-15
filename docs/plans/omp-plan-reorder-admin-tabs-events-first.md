# Working Plan: Reorder admin tabs to put Events first

## Objective
Reorder the Admin screen's tab list so **Events** is the first tab (before Sites), and make **Events** the default active tab so the Admin screen opens on the Events tab. This is a pure UI reordering in the single embedded Go template file — no Go handler, JS, or CSS changes.

## Constraints
- **Single file only:** `r3-intake/internal/assets/public/index.html`. No other files touched.
- **Preserve the `{{if .IsAdmin}}` guard:** Users and Events tabs/panels stay wrapped in `{{if .IsAdmin}}` exactly as today. The Sites tab/panel stays **outside** the admin guard.
- **Default active tab must become Events:** `aria-selected="true"` moves from Sites to Events; the Events panel must no longer be `hidden`, and the Sites panel must become `hidden`. Inactive tabs get `tabindex="-1"`; the active tab has no `tabindex` (or `tabindex="0"`).
- **No JS changes:** The existing tab-activation JS (lines ~836–872) is order-agnostic — it derives state from `aria-controls`/`aria-selected`/`hidden` and from `panel-events`/`tab-events` IDs, which are unchanged. The `EventError`/`EventName` auto-activate and the `?tab=` query-param restore continue to work untouched.
- **IDs and `aria-controls`/`aria-labelledby`/`data-tab-target` relationships must be preserved** so the JS and accessibility wiring keep working.

## File Structure
All edits are in `r3-intake/internal/assets/public/index.html`.

**Current tab buttons (lines 679–682):**
```
<button ... id="tab-sites" aria-selected="true" data-tab-target="panel-sites">Sites</button>
{{if .IsAdmin}}
<button ... id="tab-users" aria-selected="false" ... tabindex="-1">Users</button>
<button ... id="tab-events" aria-selected="false" ... tabindex="-1">Events</button>
{{end}}
```

**Current panels (lines 686, 733, 780):**
- `panel-sites` (line 686) — no `hidden` (default)
- `{{if .IsAdmin}}` wraps `panel-users` (733, `hidden`) and `panel-events` (780, `hidden`), closed by `{{end}}` at line 830.

## Implementation Notes

### 1. Reorder the tab buttons (lines 679–682)
Move Events first, then Users, then Sites. Events becomes the active tab; Sites becomes inactive. Keep the `{{if .IsAdmin}}` guard wrapping Events and Users, with Sites outside it:
```
{{if .IsAdmin}}
<button type="button" class="tab" role="tab" id="tab-events" aria-controls="panel-events" aria-selected="true" data-tab-target="panel-events">Events</button>
<button type="button" class="tab" role="tab" id="tab-users" aria-controls="panel-users" aria-selected="false" data-tab-target="panel-users" tabindex="-1">Users</button>
{{end}}
<button type="button" class="tab" role="tab" id="tab-sites" aria-controls="panel-sites" aria-selected="false" data-tab-target="panel-sites" tabindex="-1">Sites</button>
```
- Events: `aria-selected="true"`, **no** `tabindex` (active).
- Users: unchanged except position (still `aria-selected="false"`, `tabindex="-1"`).
- Sites: now `aria-selected="false"` and gains `tabindex="-1"` (was active).

### 2. Reorder the tab panels (lines 686–830)
Move the Events panel first, then Users, then Sites, matching the tab order. Keep the `{{if .IsAdmin}}` guard wrapping Events and Users panels, with Sites outside it:
```
{{if .IsAdmin}}
<div class="tab-panel" role="tabpanel" id="panel-events" aria-labelledby="tab-events">
  ... (Events table + create form, unchanged content)
</div>
<div class="tab-panel" role="tabpanel" id="panel-users" aria-labelledby="tab-users" hidden>
  ... (Users table + add form, unchanged content)
</div>
{{end}}
<div class="tab-panel" role="tabpanel" id="panel-sites" aria-labelledby="tab-sites" hidden>
  ... (Sites table + add form, unchanged content)
</div>
```
- Events panel: **remove** `hidden` (now the default active panel).
- Users panel: unchanged (still `hidden`).
- Sites panel: **add** `hidden` (no longer default).

### 3. Verify no JS changes needed
The `activate(id)` function (lines ~840–850) sets `aria-selected` and `tabIndex` from `aria-controls` and toggles `hidden` on panels by `id` — all IDs are preserved, so it works regardless of DOM order. The `EventError`/`EventName` auto-activate block and the `?tab=` restore block reference `panel-events`/`tab-events` by ID and are unaffected.

## Verification Criteria
1. **Template renders** — `go build ./...` (or `go vet ./...`) succeeds; no template syntax errors from the reordering.
2. **Tab order** — In the rendered Admin page, the tab list shows **Events, Users, Sites** (for admins). For non-admins, only **Sites** shows (Events/Users correctly hidden by the guard).
3. **Default active tab** — On a fresh load of `/admin` with no `?tab=` param and no `EventError`/`EventName`, the **Events** tab is active: `aria-selected="true"`, no `tabindex`, and `panel-events` is visible (not `hidden`). Sites and Users tabs are `aria-selected="false"` with `tabindex="-1"` and their panels `hidden`.
4. **Tab switching** — Clicking Sites, Users, and Events each activates the correct panel and updates `aria-selected`/`tabindex`/`hidden` correctly.
5. **Event error/name auto-activate** — When `EventError` or `EventName` is set, the Events tab still auto-activates.
6. **`?tab=` restore** — `/admin?tab=sites` and `/admin?tab=events` still restore the correct active tab.
7. **Admin guard intact** — Users and Events tabs/panels remain inside `{{if .IsAdmin}}`; Sites remains outside. Non-admin users see only the Sites tab.
