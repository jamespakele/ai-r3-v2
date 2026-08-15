# RESULT: Replace location filter box on attendance matrix

## What was built
Removed the location (site) *filter* from the attendance matrix and replaced
the admin location dropdown with a **disabled (read-only) textbox** that
displays the location of the currently selected event. Post epic-18, location
is derived from the event (`events.site`); the matrix is scoped by the selected
event. No Go handler/filter logic reads a separate `site`/`site_id`
location-filter query param on the matrix or export path anymore.

## Changes

### UI (child story t_f07eca4c)
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

### Go handler (child story t_f3396ece)
- `r3-intake/internal/server/attendance.go`
  - `parseMatrixFilters` no longer reads the `site` query param or calls
    `resolveSite`; signature is now `(from, to, eventID, dates)`.
  - `handleMatrix` drops `loadSites(false)` and the `SiteID`/`Sites` view
    fields; calls `loadEvents()` with no arg. `SiteName` is still resolved for
    the non-admin `Site:` span.
  - `loadMatrixRows` derives the roster scope from the SELECTED EVENT's site
    (when an event is selected) or falls back to role-based scope (case_manager
    `assigned_to`; admin all intakes). `cellSiteID`/NoLocation grouping derives
    from the event's site.
  - `loadEvents()` and `resolveEventIDs(eventID)` drop the `siteID` parameter;
    the site-scoped event-set branch is removed.
  - `handleExportCSV`/`loadExportRows` drop the `site` read and `siteID` param.
- `r3-intake/internal/server/person_attendance.go` — two `loadEvents("")` →
  `loadEvents()` call sites.
- Tests updated to the new signatures/scoping semantics (roster, export, stats,
  toggle, admin events).

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (`ok r3-intake/internal/server`,
  `ok r3-intake/pocketbase/migrations`)
- `TestMatrixContentRender` / `TestMatrixContentRenderEventRequired` render the
  changed `matrix-content` block with admin views, exercising the new
  `{{.EventLocation}}` template reference — parse + execute clean.
- No `Query().Get("site")` on the matrix/export path; only walk-in `site_id`
  reads remain (out of scope). No `site='...'` clause in any attendance query
  (location derived from the event set only).

## Artifacts
- Working plan: `docs/plans/omp-plan-replace-location-dropdown-disabled-textbox.md`
- Working plan: `docs/plans/omp-plan-remove-location-filter-matrix.md`
