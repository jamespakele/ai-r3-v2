# Working Plan: Tabbed layout for admin screen

## Objective
Convert the admin settings screen from three stacked accordion sections (Sites, Users, Events) into a tabbed interface with three tabs: **Sites**, **Users**, **Events**. Only one tab's content is visible at a time. All existing functionality (site add/toggle/default, user add/edit, event add/manage/matrix/report) must be preserved unchanged. The event form markup is **out of scope** — a sibling card restructures it; this card only wraps it in a tab.

## Constraints
- **Language:** Go (server-rendered `html/template`), vanilla JS for tab switching, vanilla CSS.
- **Framework:** No JS framework. The admin block is a standalone full HTML document that does **not** load Alpine.js or htmx (those are only in the main intake template). The existing accordion uses inline `onclick` JS, so tab switching must use plain vanilla JS — do not add Alpine to the admin block.
- **Dependencies:** None new. Reuse existing `app.css` and the existing `?v=` cache-buster (bump `v=4` → `v=5`).
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, 14px card radii, 8px input radii.
- **Scope boundary:** Do **not** touch the event form structure (state/end dates, create button placement, description box size). Preserve the event form markup exactly as-is.

## File Structure
| File | Action | Notes |
|------|--------|-------|
| `r3-intake/internal/assets/public/index.html` | **Modify** | Replace the three `.accordion` blocks in `{{define "admin"}}` (lines ~512–676) with a tabbed layout; add a small inline `<script>` for tab switching; bump stylesheet link to `?v=5`. |
| `r3-intake/internal/assets/public/app.css` | **Modify** | Add tab styles (`.tabs`, `.tab-list`, `.tab`, `.tab[aria-selected="true"]`, `.tab-panel`). Keep existing `.accordion*` rules (used elsewhere / harmless) or remove them if confirmed unused. |
| `r3-intake/internal/server/admin.go` | **No change** | `AdminView` already carries all needed fields (`IsAdmin`, `Sites`, `Users`, `Events`, `EventError`, `EventName`, …). No handler changes required. |

## Implementation Notes

### Markup structure
Replace the three `.accordion` divs with a single tab container:

```html
<div class="tabs" data-tabs>
  <div class="tab-list" role="tablist" aria-label="Admin sections">
    <button type="button" class="tab" role="tab" id="tab-sites" aria-controls="panel-sites"
            aria-selected="true" data-tab-target="panel-sites">Sites</button>
    {{if .IsAdmin}}
    <button type="button" class="tab" role="tab" id="tab-users" aria-controls="panel-users"
            aria-selected="false" data-tab-target="panel-users">Users</button>
    <button type="button" class="tab" role="tab" id="tab-events" aria-controls="panel-events"
            aria-selected="false" data-tab-target="panel-events">Events</button>
    {{end}}
  </div>

  <div class="tab-panel" role="tabpanel" id="panel-sites" aria-labelledby="tab-sites">
    <!-- existing Sites table + add-site form, unchanged -->
  </div>

  {{if .IsAdmin}}
  <div class="tab-panel" role="tabpanel" id="panel-users" aria-labelledby="tab-users" hidden>
    <!-- existing Users table + add-user form, unchanged -->
  </div>
  <div class="tab-panel" role="tabpanel" id="panel-events" aria-labelledby="tab-events" hidden>
    <!-- existing Events table + event form, unchanged -->
  </div>
  {{end}}
</div>
```

- **Gating preserved:** The Users and Events tabs **and** their panels stay wrapped in `{{if .IsAdmin}}`. The Sites tab/panel is always rendered. A non-admin sees only the Sites tab.
- **Event form untouched:** Copy the existing event table + `{{if .EventError}}` block + `<form method="post" action="/admin/events" class="form-grid-4">…</form>` verbatim into the Events panel. No restructuring.

### Tab switching (vanilla JS)
Add a small inline `<script>` before `</body>` in the admin block (the block has no external script today). Use event delegation on the tab list:

```html
<script>
(function () {
  var tabs = document.querySelector('[data-tabs]');
  if (!tabs) return;
  var buttons = tabs.querySelectorAll('.tab');
  var panels = tabs.querySelectorAll('.tab-panel');
  function activate(id) {
    buttons.forEach(function (b) {
      var on = b.getAttribute('aria-controls') === id;
      b.setAttribute('aria-selected', on ? 'true' : 'false');
      b.tabIndex = on ? 0 : -1;
    });
    panels.forEach(function (p) {
      p.hidden = p.id !== id;
    });
  }
  tabs.addEventListener('click', function (e) {
    var b = e.target.closest('.tab');
    if (b) activate(b.getAttribute('aria-controls'));
  });
  // Default active tab: Events when a failed event POST left an error or form values.
  var evtPanel = document.getElementById('panel-events');
  var evtTab = document.getElementById('tab-events');
  if (evtPanel && evtTab &&
      ({{if .EventError}}true{{else}}false{{end}} || {{if .EventName}}true{{else}}false{{end}})) {
    activate('panel-events');
  }
})();
</script>
```

- **Why plain JS over Alpine:** The admin block does not load Alpine (only the main intake template does). Adding Alpine just for tabs would be heavier and inconsistent with the existing inline-`onclick` accordion pattern. A ~20-line vanilla snippet is the simplest approach consistent with the codebase.
- **Default active tab / error edge case:** After a failed event create, the page reloads with `EventError` set and `EventName`/`EventStart`/`EventEnd`/`EventDescription` populated. The snippet defaults the active tab to **Events** when `EventError` is truthy **or** `EventName` is non-empty, so the user sees the error and their entered values. Otherwise Sites is active by default (first tab, `aria-selected="true"` in markup).
- **Keyboard/ARIA:** `role=tablist`/`role=tab`/`role=tabpanel`, `aria-selected`, `aria-controls`, `aria-labelledby`, and `tabIndex` management give reasonable accessibility. Full arrow-key roving is optional; the click handler + `tabIndex` is sufficient for this card.

### CSS
Add to `app.css` (design-system consistent):

```css
.tabs { margin: 24px 0; }
.tab-list { display: flex; gap: 4px; border-bottom: 1px solid #e4d9c8; margin-bottom: 0; }
.tab {
  padding: 12px 20px; border: none; background: transparent; cursor: pointer;
  font: 600 15px 'Public Sans', sans-serif; color: #6b5f52;
  border-bottom: 3px solid transparent; border-radius: 8px 8px 0 0;
}
.tab:hover { color: #2b2320; background: #f7f1e6; }
.tab[aria-selected="true"] { color: #b5502e; border-bottom-color: #b5502e; }
.tab-panel {
  border: 1px solid #e4d9c8; border-top: none; border-radius: 0 0 14px 14px;
  background: #fffdfa; padding: 24px;
}
.tab-panel[hidden] { display: none; }
```

- Active tab uses accent `#b5502e` underline; panel uses card `#fffdfa` with 14px bottom radii to match the design system.
- **Cache-buster:** bump the admin stylesheet link from `?v=4` to `?v=5` so the new CSS is fetched.
- The old `.accordion*` rules can be left in place (harmless) or removed if a grep confirms no other template uses them; prefer leaving them to minimize risk.

## Verification Criteria
1. **Build/vet/test:** `cd r3-intake && go build ./... && go vet ./... && go test ./...` — all pass. The template-parsing tests must still pass, confirming the admin block parses after the markup change.
2. **Template parses:** The embedded `index.html` is parsed by tests; a successful `go test ./...` proves the `{{define "admin"}}` block is still valid Go template syntax (balanced `{{if}}`/`{{end}}`, correct field references).
3. **Manual (browser) checks:**
   - Log in as admin → admin screen shows three tabs (Sites, Users, Events); Sites is active by default; only Sites content visible.
   - Click Users → Users content shows; Sites hidden; `aria-selected` moves correctly.
   - Click Events → Events content shows; event table + form render.
   - Add a site, toggle/deactivate a site, set default → all still work from the Sites tab.
   - Add/edit a user → still works from the Users tab.
   - Create an event with a **blank/invalid field** → page reloads, **Events tab is active**, error message and previously entered form values are visible.
   - Create a valid event → Events tab shows the new row; Manage/Matrix/Report links work.
   - Log in as a **non-admin** → only the Sites tab is present; Users/Events tabs and panels are absent.
4. **CSS:** active tab underline is `#b5502e`; panel/card colors match the design system; `?v=5` is served (hard-refresh to bypass cache).
