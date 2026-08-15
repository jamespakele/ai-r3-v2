# Working Plan: Remove location filter logic from attendance matrix Go handler

## Objective
Remove the redundant location (site) *filter* from the attendance matrix Go
handler. Post epic-18, location is derived from the event (`events.site`); the
matrix is scoped by the selected event. No Go handler/filter logic may read a
separate `site`/`site_id` location-filter query param on the matrix or export
path. Location editing stays exclusively in the admin event management screen
(out of scope for this card).

## Constraints
- PocketBase v0.39 Go API only: `s.pb.FindCollectionByNameOrId`,
  `s.pb.FindRecordsByFilter`, `s.pb.FindRecordById`, `s.pb.Save`, `s.pb.Delete`.
  No `app.dao()`.
- Filter escaping via `mcpmod.EscapeFilter(s)` (import
  `mcpmod "r3-intake/internal/mcp"`).
- All timestamps HST: `time.Now().In(hst)`. Date strings `2006-01-02`.
- Verification gates: `cd r3-intake` then `go build ./...`, `go vet ./...`,
  `go test ./...` must all pass.
- Do NOT touch the template file `r3-intake/internal/assets/public/index.html`
  (sibling card t_f07eca4c owns the UI: replace the location dropdown with a
  disabled textbox populated from the selected event's location). Do NOT touch
  `person_attendance.go` unless strictly necessary.
- Do NOT plan schema/migration changes (epic-18 already removed
  `attendance.site` via migration 015).
- KEEP `intake.site` reads (person's HOME site) for roster scoping and
  NoLocation grouping — these are NOT event location and must remain.

## File Structure
- `r3-intake/internal/server/attendance.go` — the only source file changed
  (Go handler side).
- Tests (same `package server`, updated/added):
  - `r3-intake/internal/server/attendance_roster_integration_test.go`
  - `r3-intake/internal/server/attendance_export_integration_test.go`
  - `r3-intake/internal/server/attendance_stats_integration_test.go`
- No template, no migration, no `person_attendance.go` changes.

## Implementation Notes

### Key decision: what scopes the roster and the event set
The location filter is removed entirely. The matrix is scoped by the selected
event. When an event is selected, its site (`events.site`) is the location and
scopes the roster (participants at that location). When no event is selected,
the roster falls back to the role-based scope (case_manager: `assigned_to`;
admin: all intakes). This is the intended end state per the epic and matches
the existing `loadMatrixRows` roster switch.

Concretely:
- `parseMatrixFilters` STOPS reading the `site` query param. It returns
  `(from, to, eventID, dates)` only. The `siteID`/`siteName` values are no
  longer derived from a filter param.
- `loadMatrixRows` derives the roster site from the SELECTED EVENT's site, not
  from a filter param. When `eventID != ""`, look up the event record and use
  its `site` as the roster scope. When `eventID == ""`, fall back to the
  role-based scope (case_manager: `assigned_to`; admin: all intakes).
- `loadEvents` and `resolveEventIDs` DROP the `siteID` parameter. The event set
  is either the single selected event (`{eventID}`) or, when no event is
  selected, all active non-deleted events (admin view). No site-scoped event
  query remains on the matrix/export path.

### Detailed changes in attendance.go

1. `parseMatrixFilters` (~L143): remove the `site` query-param read and the
   `resolveSite` call. New signature:
   `(from, to, eventID string, dates []string)`. Keep from/to parsing,
   validation, range swap, 30-day cap, and the `event`/`event_id` fallback
   read unchanged.

2. `handleMatrix` (~L71): call the new `parseMatrixFilters` signature. Remove
   `loadSites(false)` (no longer needed — the location box is the sibling's
   disabled textbox, not a dropdown). Call `loadEvents()` with no arg. Build
   `MatrixViewData` WITHOUT `SiteID`/`SiteName`/`Sites` (or leave them zeroed
   if the struct fields are kept for template compatibility — see note below).
   Keep `EventID`, `EventRequired`, `HasNoLocation`, `Rows`, `Dates`, `Summary`.

3. `handleStats` (~L181): update to the new `parseMatrixFilters` signature and
   `loadMatrixRows` signature. It only needs `Summary`, so pass the eventID and
   dates through.

4. `loadMatrixRows` (~L271): change signature to
   `(u *sessionUser, dates []string, eventID, to string)`. Roster scoping:
   - If `eventID != ""`: look up the event record
     (`s.pb.FindRecordById(eventsCol.Id, eventID)`); if found and its `site`
     is non-empty, `intakeFilter = "site='<site>'"` (escaped). If the event
     has no site, fall through to the role-based default.
   - Else (no event): case_manager -> `assigned_to='<u.ID>'`; admin -> `1=1`.
   - `cellSiteID` for NoLocation grouping: when `eventID != ""`, derive from
     the event's site; when `eventID == ""`, fall back to `rec.GetString("site")`
     (intake home site) as today. `row.NoLocation = cellSiteID == ""` stays.
   - `resolveEventIDs(eventID)` (no siteID arg) for the attendance map.

5. `loadEvents` (~L424): drop the `siteID` parameter. Filter is always
   `status='active' && deleted=false`. Callers: `handleMatrix` (no arg),
   `resolveEventIDs` (no arg), and `person_attendance.go` (already calls
   `loadEvents("")` — update those two call sites to `loadEvents()`).

6. `resolveEventIDs` (~L455): drop the `siteID` parameter. If `eventID != ""`
   return `{eventID}`; else return `nil` (no restriction — all active events).
   The site-scoped event-set branch is removed.

7. `handleExportCSV` (~L875): remove the `site` query-param read and the
   `resolveSite` call. Call `loadExportRows(eventID, from, to)`.

8. `loadExportRows` (~L934): drop the `siteID` parameter. Signature
   `(eventID, from, to string)`. Use `resolveEventIDs(eventID)` for the event
   set. The `eventIDs != nil && len(eventIDs) == 0` early-return is now
   unreachable (eventID non-empty -> `{eventID}`; empty -> nil) — remove it or
   keep it harmlessly. Export `SiteName` already derives from the event's site
   (unchanged).

### View-model / template compatibility
The sibling card replaces the location dropdown with a disabled textbox. To
avoid a merge conflict and keep the template compiling in the interim, KEEP the
`SiteID`, `SiteName`, and `Sites` fields on `MatrixViewData` (leave them zeroed
/ empty in `handleMatrix`). The sibling owns removing their template usage. Do
NOT delete the struct fields in this card.

### Out of scope (do NOT touch)
- `resolveSite` (~L218) itself: still used by `handleWalkinSearch` (~L704) and
  `handleWalkin` (~L743) for the walk-in path, which is a separate concern.
  Only the matrix/export callers stop using it.
- `handleToggle` (~L462): still reads `site_id` from the toggle form and
  derives the intake's home site for the attendance record. This is the
  write-path site derivation (intake home site), NOT a location filter. Leave
  it. The `site=` redirect query it appends is harmless; the sibling's UI
  change will stop sending a location filter.
- `handleWalkinSearch` / `handleWalkin`: unchanged.
- Template file, migrations, `person_attendance.go`.

## Verification Criteria
- `cd r3-intake` then `go build ./...`, `go vet ./...`, `go test ./...` all
  pass.
- `grep -n 'Query().Get("site")' r3-intake/internal/server/attendance.go`
  returns nothing on the matrix/export path (the only remaining `site` reads
  are the walk-in handlers' `site_id` form/query reads, which are out of
  scope).
- `parseMatrixFilters` no longer calls `resolveSite`; `loadMatrixRows`,
  `loadEvents`, `resolveEventIDs`, and `loadExportRows` no longer take a
  `siteID` parameter.
- No `site='...'` clause appears in any attendance query in `attendance.go`
  (location is derived from the event set only).

- Roster scoping: with an event selected, the roster is scoped to that event's
  site; with no event selected, it falls back to role-based scope
  (case_manager: assigned_to; admin: all intakes).
- Existing integration tests updated to the new signatures still pass:
  - attendance_roster_integration_test.go: loadMatrixRows calls updated (drop
    the explicit fx.site arg; the event's site now scopes the roster).
  - attendance_export_integration_test.go: loadExportRows calls updated (drop
    the siteID arg); ?site= query cases removed or re-expressed via the event.
  - attendance_stats_integration_test.go: ?site= query cases removed.
  - admin_events_update_delete_test.go: loadEvents calls updated to no-arg.
  - attendance_toggle_integration_test.go: loadExportRows call updated.
- No template, migration, or person_attendance.go changes in this card.
