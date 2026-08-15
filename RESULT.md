# RESULT — Remove location filter logic from attendance matrix Go handler

## What was built
Removed the redundant location (site) *filter* from the attendance matrix Go
handler. Post epic-18, location is derived from the event (`events.site`); the
matrix is scoped by the selected event. No Go handler/filter logic reads a
separate `site`/`site_id` location-filter query param on the matrix or export
path anymore.

## Changes (Go handler side only — no template, no migration)
- `r3-intake/internal/server/attendance.go`
  - `parseMatrixFilters` no longer reads the `site` query param or calls
    `resolveSite`; signature is now `(from, to, eventID, dates)`.
  - `handleMatrix` drops `loadSites(false)` and the `SiteID`/`SiteName`/`Sites`
    view fields; calls `loadEvents()` with no arg.
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
- `go test ./...` — `ok r3-intake/internal/server 15.027s`,
  `ok r3-intake/pocketbase/migrations 0.179s`
- No `Query().Get("site")` on the matrix/export path; only walk-in `site_id`
  reads remain (out of scope). No `site='...'` clause in any attendance query
  (location derived from the event set only).

## Artifacts
- Working plan: `docs/plans/omp-plan-remove-location-filter-matrix.md`
