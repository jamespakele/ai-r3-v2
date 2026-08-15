# Working Plan: Replace Site/Location dropdown with Event dropdown on intake form

## Objective
On the R3 Intake form (section 01), replace the field group currently labeled
"R3 Site Location" — which renders a dropdown of **Sites** via the
`site-fragment` template — with a dropdown of **Events** (the `events`
collection), relabeled appropriately. The intake's home reference becomes an
**Event** instead of a **Site**.

The Go backend renames are **already done** (sibling card): `FormState.SiteSel`
→ `EventSel`, `intake.site` → `intake.event`, `REQUIRED_FIELDS` `"site"` →
`"event"`, `blankState` defaults `EventSel` to the first active event,
`stateFromRecord` reads `rec.GetString("event")`, `applySection`/`applyToState`
read `FormValue("event")`, and `validateState` checks `"event"`. The
`loadEvents()` helper and `Event` struct already exist in `attendance.go`.

This card's job is the **template region** in
`r3-intake/internal/assets/public/index.html`, plus the small Go change needed
to give `FormState` an `Events []Event` list so the template can iterate it.

## Constraints
- **Template-only card.** The only file this card should modify is
  `r3-intake/internal/assets/public/index.html`. The one exception is the
  required `FormState.Events []Event` field + population in `handlers.go`
  (explicitly left in scope for this card by the parent).
- Do **not** touch `attendance.go`, `server.go`, or any other Go logic beyond
  the `FormState` struct and its two constructors (`blankState`,
  `stateFromRecord`).
- The form field `name` **must be `"event"`** — matches
  `applySection`/`applyToState` reading `FormValue("event")` and
  `validateState` checking `"event"`.
- Error map key must be `Errors.event` (was `Errors.site`).
- Templates are a single embedded HTML file with multiple `{{define}}` blocks.
  New blocks go at the end, before the closing `{{end}}` of the last block.
- HTMX: return raw HTML fragments; use `s.tpl.ExecuteTemplate(w, "partial-name", data)`.
- All timestamps HST. Design system: Public Sans + Lora, accent `#b5502e`,
  card `#fffdfa`, page bg `#f7f1e6`, 14px card radii, 8px input radii.
- Verification: `cd r3-intake && go build ./... && go vet ./... && go test ./...`
  must pass.

## File Structure
- `r3-intake/internal/assets/public/index.html` — **primary edit** (template region).
  - `site-fragment` template (~line 101) → becomes `event-fragment`.
  - Section-01 field group (~line 128) → relabel + swap fragment + error key.
- `r3-intake/internal/server/handlers.go` — **small Go edit** (in scope).
  - `FormState` struct: add `Events []Event`.
  - `blankState`: populate `Events` via `s.loadEvents()`.
  - `stateFromRecord`: populate `Events` via `s.loadEvents()`.
- `r3-intake/internal/server/attendance.go` — **read-only** (reference only).
  - `Event` struct `{ID, Name, SiteID, StartDate, EndDate, Status}`.
  - `loadEvents()` returns active, non-deleted events sorted by `start_date,name`.
- `r3-intake/internal/server/server.go` — **read-only** (reference only).
  - Route `/sites` → `handleSites` (site-fragment htmx refresh).

## Implementation Notes

### 1. Add `Events []Event` to `FormState` (handlers.go)
The template needs a list to iterate. `FormState` currently has `EventSel string`
but no `Events` field. Add it next to `Sites`:

```go
type FormState struct {
	ID        string
	HasRecord bool
	Sites     []Site
	Events    []Event   // NEW: active events for the intake dropdown
	EventSel  string
	...
}
```

### 2. Populate `Events` in `blankState` (handlers.go)
`blankState` already calls `s.loadEvents()` to default `EventSel`. Capture the
result into the struct so the template can iterate it:

```go
func (s *Server) blankState(user *sessionUser) *FormState {
	sites := must(s.loadSites(false))
	events := must(s.loadEvents())   // NEW: load once, reuse for default + list
	st := &FormState{
		Sites:         sites,
		Events:        events,        // NEW
		...
	}
	// Default the intake's home event to the first active event.
	if len(events) > 0 {
		st.EventSel = events[0].ID
	}
	...
}
```

> Note: `blankState` currently uses `if events, err := s.loadEvents(); err == nil && len(events) > 0`. Refactor to load once into a local `events` slice and assign both `st.Events` and the default `EventSel`. Using `must()` matches the existing `sites := must(s.loadSites(false))` pattern; if a load error is acceptable to swallow, keep the `err == nil` guard but still assign `st.Events` from the successful result.

### 3. Populate `Events` in `stateFromRecord` (handlers.go)
`stateFromRecord` builds a `FormState` from a saved record. Add the events list
alongside the existing `Sites` load:

```go
st := &FormState{
	ID:        rec.Id,
	HasRecord: true,
	Sites:     must(s.loadSites(false)),
	Events:    must(s.loadEvents()),   // NEW
	EventSel:  rec.GetString("event"),
	...
}
```

### 4. Rename `site-fragment` → `event-fragment` (index.html)
Replace the template block (~line 101). Iterate `.Events`, select by `.EventSel`,
use `name="event"`:

```html
{{define "event-fragment"}}
<select name="event" class="field-input">
  <option value="">Select an event</option>
  {{range .Events}}<option value="{{.ID}}" {{if eq .ID $.EventSel}}selected{{end}}>{{.Name}}</option>{{end}}
</select>
{{end}}
```

### 5. Update the section-01 field group (index.html, ~line 128)
Relabel the group, swap the fragment, and update the error key:

```html
<div class="field-group {{if .Errors.event}}has-error{{end}}">
  <label class="field-label">R3 Event <span class="field-required">*</span></label>
  {{template "event-fragment" .}}
  {{if .Errors.event}}<div class="field-error">Please select an event.</div>{{end}}
</div>
```

### 6. htmx refresh path — recommendation: leave static (no new handler)
The section-01 form already `hx-post="/section/01"` on `change` (with
`hx-trigger="change delay:300ms, submit"`), so the dropdown value is persisted
server-side on every change. The `/sites` route + `handleSites` handler exist
only for an **optional** htmx refresh of the site list; there is no requirement
to refresh the event list dynamically.

**Recommendation:** do **not** add an `event-fragment` handler or `/events`
route. The dropdown is populated server-side on every page render from
`FormState.Events`, and the form already self-persists on change. Adding a
handler would be dead code with no consumer. If a future card needs dynamic
event refresh, it can add `handleEvents` mirroring `handleSites` (build
`&FormState{Events: must(s.loadEvents())}` and
`ExecuteTemplate(w, "event-fragment", st)`), but that is out of scope here.

> Note: the old `site-fragment` template and `handleSites`/`/sites` route are
> left in place (template-only card; removing them is a cleanup card). The
> `site-fragment` block becomes unused but harmless. Do not delete it in this
> card to keep the diff minimal and avoid touching `server.go`.

## Verification Criteria
1. `cd r3-intake && go build ./...` passes.
2. `cd r3-intake && go vet ./...` passes.
3. `cd r3-intake && go test ./...` passes.
4. Template renders the event dropdown with:
   - `name="event"` (matches `FormValue("event")` in applySection/applyToState).
   - Options iterating `.Events` (the `events` collection).
   - The saved/default selection marked `selected` via `eq .ID $.EventSel`.
   - Label updated (no longer "R3 Site Location").
   - Error key `Errors.event` (matches `validateState`'s `check("event", ...)`).
5. `FormState` has `Events []Event` populated in **both** `blankState` and
   `stateFromRecord` via `s.loadEvents()`.
6. Manual smoke test (if runnable): open a new intake → section 01 shows an
   "R3 Event" dropdown listing active events, defaulting to the first; changing
   it persists via the existing `hx-post="/section/01"`; a saved record reloads
   with the stored event selected.
