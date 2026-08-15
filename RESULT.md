# RESULT: Update filtering logic to use event-derived location

## What was built
Updated the Go attendance read/write paths so location is always derived from
the event (`events.site`), never from the redundant `attendance.site` field.

## Changes
- `r3-intake/internal/server/attendance.go`
  - Added `resolveEventIDs(eventID, siteID)` helper: specific event wins; else
    active events at site (empty slice when none); nil = no restriction (admin).
  - `loadMatrixRows`: attendance filter is now date range + `event='id1' || event='id2' …`
    (OR-chain; PocketBase has no IN operator). Empty event set skips the query.
    Removed the `(site='' || site='%s')` clause. Roster (intakeFilter) unchanged.
  - `loadExportRows`: same event-set resolution; empty set returns empty rows;
    removed `site='%s'` clause.
  - Export `SiteName` resolved from the row's event's site via
    `FindRecordById(eventsCol.Id, event)`; empty on lookup failure.
  - Removed `Set("site", siteID)` from `handleToggle` insert and all three
    `handleWalkin` write paths. `siteID` kept for `Disabled` flag + redirect.
- `r3-intake/internal/server/person_attendance.go`
  - Removed `rec.Set("site", intake.GetString("site"))` from day-detail insert.

## Home-site reads left untouched (roster scoping, NOT event location)
`intake.site` in roster filter, toggle site derivation, `cellSiteID`/NoLocation
grouping, walk-in intake creation (line 770), person calendar SiteName.

## Tests updated (asserted on the now-removed field)
- `TestToggleLocated`, `TestPersonAttendanceDaySaveCreate`: assert site empty.
- `TestExportCSVSiteFilter`: site2-only now expects 3 records (att-5's divergent
  stored Kona site resolves to ev2's Waianae); added "event wins over site"
  subtest; removed invalid site-only subtest (export requires an event).

## Verification
- `go build ./...` PASS
- `go vet ./...` PASS
- `go test ./...` PASS (internal/server 15.3s ok, migrations ok)
- Attendance queries contain no `site=` clause; remaining `site='%s'` filters
  are on `intake` (roster) and `events` (event-set resolution) only.
