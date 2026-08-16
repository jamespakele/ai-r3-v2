# RESULT — Collapse Create Event form behind toggle button

## What was built
On the Admin > Events tab, the Create Event form (name, location, start/end
dates, description) is now hidden by default and only appears when the
"Create Event" button is clicked. Clicking again (now labeled "Cancel") hides
it. The form renders below the button.

## Files changed
- `r3-intake/internal/assets/public/index.html` (11 insertions, 1 deletion)

## Changes
1. Added a `type="button"` toggle button (`id="create-event-toggle"`, class
` btn btn-primary btn-auto`) above the form, wired to
`toggleCreateForm('create-event-form', this)`. Its label is
`{{if .EventError}}Cancel{{else}}Create Event{{end}}`.
2. Gave the form `id="create-event-form"` and a conditional
`style="display:block"` (on validation error) / `style="display:none"`
(default). POST target, class, and all field bindings unchanged.
3. Added `window.toggleCreateForm` next to `toggleEditRow`, toggling
`style.display` between `'none'`/`'block"` and swapping the button label.

## Logical consequences handled
- Submit button stays `type="submit"` inside the form, label "Create Event",
  unchanged — only reachable once the form is shown.
- On validation error (`.EventError` set), the form auto-shows and the toggle
  button reads "Cancel", so the error is actionable with pre-filled values.
- `.btn-auto` reused on the toggle button (already defined in app.css L75).
- CSS cache-buster `?v=` NOT bumped — that is the sibling card's scope.

## Verification
- `go build ...` — PASS
- `go vet ...` — PASS
- `go test ...` — PASS (internal/server 17.1s, migrations 0.225s)
- `TestAdminEventsRender` covers both toggle states (default hidden + error
  path shown) and asserts `action="/admin/events"` and `Create Event`.

## Note
Live-browser smoke test not run: this worktree has no `cmd/` package / `func
main`, so the binary cannot be built here (pre-existing repo state). The
template-render test covers both toggle states' server-side output; the
client-side click behavior is the standard `style.display` swap already proven
by the identical `toggleEditRow` pattern.
