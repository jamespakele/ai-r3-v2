# RESULT — Add Edit and Delete actions to admin Events UI

## What was built
Added Edit and Delete actions to the admin Events tab (UI layer only).

- **Edit**: each event row now has an Edit button that toggles a hidden inline
  edit row (`edit-event-{id}`), mirroring the existing Users-tab pattern. The
  form pre-fills name, site `<select>` (selected via `$ev.SiteID`), start/end
  dates, and description, and posts to `POST /admin/events/{id}/update`.
- **Delete**: each row has a Delete button with a `confirm()` dialog that
  submits a hidden `delete-event-{id}` form to `POST /admin/events/{id}/delete`
  (soft-delete).

## Files changed
- `r3-intake/internal/server/admin.go` — added `SiteID` and `Description` to
  `EventRow`; populated both in `loadAllEvents()` from `r.GetString("site")` /
  `r.GetString("description")`.
- `r3-intake/internal/assets/public/index.html` — Events tab (`panel-events`):
  Edit/Delete buttons, hidden edit row, hidden delete form. Outer loop changed
  to `{{range $ev := .Events}}` so `$ev.SiteID` is reachable inside the nested
  `{{range $.Sites}}`.

## Backend dependency
The `/update` and `/delete` handlers live on the sibling branch `t_dc1890be`
(not in this worktree). This card only targets those routes; no handler code
was written here.

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (`ok r3-intake/internal/server`)
- `TestAdminEventsRender` exercises the `{{range $ev := .Events}}` block
  including the nested site `<select>` scoping.
