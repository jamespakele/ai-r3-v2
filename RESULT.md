# RESULT: Replace location dropdown with disabled textbox on attendance matrix

## What was built
Replaced the admin-only location `<select name="site">` filter on the attendance
matrix screen with a **disabled (read-only) textbox** that displays the location
of the currently selected event.

- `r3-intake/internal/server/attendance.go`
  - Added `EventLocation string` field to `MatrixViewData`.
  - `handleMatrix` resolves the selected event location by iterating the
    already-loaded events list for `ev.ID == eventID`, then
    `s.nameFor("sites", ev.SiteID)`. No extra DB round-trip.
- `r3-intake/internal/assets/public/index.html`
  - In `matrix-content`, replaced the admin `<select name="site">` with a
    disabled textbox bound to `{{.EventLocation}}`.
  - `name="site"` removed (no longer submits a `site` query param).
  - Non-admin `Site: {{.SiteName}}` span unchanged; walk-in `site_id` hidden
    inputs untouched.

## Scope boundary
This card is the UI change only. The sibling card (t_f3396ece) removes the
location-filter logic from the Go handler. `resolveSite`, `parseMatrixFilters`,
and `site` query-param parsing were left untouched here.

## Verification
- `go build ./...` - pass
- `go vet ./...` - pass
- `go test ./...` - pass (ok r3-intake/internal/server 15.051s, ok
  r3-intake/pocketbase/migrations 0.197s)
- `TestMatrixContentRender` / `TestMatrixContentRenderEventRequired` render the
  changed `matrix-content` block with admin views, exercising the new
  `{{.EventLocation}}` template reference - parse + execute clean.

## Artifacts
- Working plan: `docs/plans/omp-plan-replace-location-dropdown-disabled-textbox.md`
