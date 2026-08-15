# RESULT — Replace Site/Location dropdown with Event dropdown on intake form

## What was built
Replaced the section-01 "R3 Site Location" dropdown (which listed SITES via the
`site-fragment` template) with an EVENT dropdown on the R3 Intake form.

- **Template** (`r3-intake/internal/assets/public/index.html`):
  - New `event-fragment` block: `<select name="event">` iterating `.Events`,
    selected by `.EventSel`, with a "Select an event" empty option.
  - Section-01 field group relabeled "R3 Event", error key `Errors.event`
    ("Please select an event.").
  - `site-fragment` left intact (still used by `handleSites` in server.go).
- **Go** (`r3-intake/internal/server/handlers.go`):
  - Added `Events []Event` to `FormState`.
  - `blankState` loads events once via `must(s.loadEvents())`, assigns
    `st.Events`, defaults `EventSel` to the first active event.
  - `stateFromRecord` populates `Events: must(s.loadEvents())`.

## Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (internal/server 15.6s, pocketbase/migrations 0.18s)
- Template render check (real funcs map): `event-fragment` outputs
  `<select name="event">` with options from `.Events` and `selected` on the
  saved value.

## Notes
- No new `/events` htmx handler added — the section-01 form already
  self-persists via `hx-post="/section/01"` on change; a dynamic refresh handler
  would be dead code.
- The parent card's Go renames (EventSel, intake.event, loadEvents) were copied
  into this worktree from the parent's worktree (uncommitted there) so the
  template could be built against the correct view model.
