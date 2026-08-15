# Working Plan: Event-Derived Location Filtering

## Objective

Update the Go attendance read/write paths so that a location is **always derived
from the event** (via `events.site`), never from the redundant `attendance.site`
field. This is the filtering child card of the "Event / Location / Attendance
Relationships" epic. The sibling schema-migration card removes `attendance.site`
from the collection entirely; this card makes the Go code correct regardless of
whether that migration has run — it simply stops reading and writing the field.

Concretely:
- **Filtering by location** applies to the **events** query (`events.site = X`),
  producing a set of event IDs; the **attendance** query filters by `event IN
  (that set)` and by `date`. The attendance query never contains a `site=`
  clause.
- **Filtering by event** restricts attendance to that event's rows.
- **Export `SiteName`** is resolved from the row's event's site, not from
  `attendance.site`.
- **All writes** stop setting `site` on attendance rows.

## Constraints

- **Language:** Go.
- **Framework:** PocketBase v0.39 JS/Go API. Use `app.findCollectionByNameOrId`
  (Go: `s.pb.FindCollectionByNameOrId`), `s.pb.FindRecordsByFilter`,
  `s.pb.FindRecordById`. **No `app.dao()`** — the v0.39 API is used throughout.
- **Timezone:** All timestamps are HST (`time.Now().In(hst)`); date strings are
  `2006-01-02`. Do not introduce timezone conversions.
- **Design system:** Follow the existing `attendance.go` / `person_attendance.go`
  conventions — `mcpmod.EscapeFilter` for filter values, `s.nameFor` for name
  resolution, `core.NewRecord` + `rec.Set` for writes, `s.pb.Save` for persists.
- **Epic rule:** No participant-to-location relationship. `intake.site` is the
  participant's **home site** (roster scoping) and must remain untouched.

## File Structure

Files to **modify** (no new files):

- `r3-intake/internal/server/attendance.go`
  - `loadMatrixRows` (~270–308) — event-set resolution + attendance filter.
  - `loadExportRows` (~908–930) — event-set resolution + attendance filter +
    `SiteName` resolution.
  - `handleToggle` (~574) — remove `rec.Set("site", siteID)`.
  - `handleWalkin` (~774, 786, 797) — remove `Set("site", siteID)`.
- `r3-intake/internal/server/person_attendance.go`
  - day-detail insert (~326) — remove `rec.Set("site", intake.GetString("site"))`.

No files created. No schema/migration changes in this card (that is the sibling
card).

## Implementation Notes

### Shared event-set resolution

Both `loadMatrixRows` and `loadExportRows` need the same "resolve the set of
event IDs to include" step. Implement a small helper (e.g. `resolveEventIDs(s
*Server, eventID, siteID string) ([]string, error)`) to avoid duplication:

1. If `eventID != ""` → return `[]string{eventID}`.
2. Else if `siteID != ""` → call `loadEvents(siteID)` (which already builds
   `status='active' && deleted=false && site='<siteID>'`) and collect the
   `Event.ID` values. If the result is empty, return an empty slice.
3. Else (admin, all locations) → return `nil` (no event restriction).

### `loadMatrixRows` (attendance.go ~270)

- **Roster (`intakeFilter`) is UNCHANGED.** It still scopes by `intake.site`
  (home site) for case managers and site-filtered admins. The roster is the set
  of people to display, independent of the event.
- **Attendance filter:** always start with
  `date>='<from>' && date<='<to>'`. Then:
  - If the resolved event set is non-empty, append
    ` && event IN ('<id1>','<id2>',...)` (each id via `mcpmod.EscapeFilter`).
  - If the set is empty (a site with no active events), the attendance query
    returns no rows — correct, because there are no events at that location.
  - **Remove the `(site='' || site='%s')` clause entirely.**
- The `cellSiteID` grouping (line ~331, `NoLocation` grouping) reads
  `intake.site` and **MUST STAY** — it reflects the person's roster scoping, not
  the event location.

### `loadExportRows` (attendance.go ~908)

- Same event-set approach: resolve the set from `eventID` or from
  `events.site = siteID`, then filter attendance by `date` and `event IN (...)`.
- **Remove the `site='%s'` clause entirely.**

### Export `SiteName` column (attendance.go ~930)

Replace `SiteName: s.nameFor("sites", rec.GetString("site"))` with event-derived
resolution:

```go
eventRec, err := s.pb.FindRecordById(eventsCol.Id, rec.GetString("event"))
siteName := ""
if err == nil {
    siteName = s.nameFor("sites", eventRec.GetString("site"))
}
```

`eventsCol` comes from `s.eventsCollection()` (already available). If the event
lookup fails (e.g. hard-deleted event), `siteName` stays `""` — matches the
current "no location" behavior.

### Write paths — remove `Set("site", ...)` on attendance rows

- `attendance.go:574` (`handleToggle` insert) — drop `rec.Set("site", siteID)`.
- `attendance.go:774` (`handleWalkin` update) — drop `existing.Set("site", siteID)`.
- `attendance.go:786` (`handleWalkin` insert) — drop `rec.Set("site", siteID)`.
- `attendance.go:797` (`handleWalkin` update-after-race) — drop `recs[0].Set("site", siteID)`.
- `person_attendance.go:326` (day-detail insert) — drop
  `rec.Set("site", intake.GetString("site"))`.

The `siteID` variable in `handleToggle`/`handleWalkin` is **still used** for the
`Disabled` flag and the redirect query string — **keep it**. It is simply no
longer written to the attendance row.

### `intake.site` reads that MUST STAY (do not touch)

These are the participant's **home site** for roster scoping, not the event
location:

- `attendance.go:242` — `resolveSite` case-manager site derivation.
- `attendance.go:331` — `loadMatrixRows` `cellSiteID` (NoLocation grouping).
- `attendance.go:520,525` — `handleToggle` site derivation from intake.
- `person_attendance.go:183` — person calendar `SiteName` display.
- `attendance.go:741` — `handleWalkin` sets `site` on the **intake** record
  (the walk-in's home site) — MUST STAY.

### Edge cases

- **Legacy data:** pre-010 null-event attendance was backfilled into synthetic
  "Legacy / Unassigned" events (010). Those events have a `site`, so their
  attendance resolves a location via the event. Any divergent stored
  `attendance.site` is simply ignored (the event's site is authoritative).
- **No-location participants:** a participant with `intake.site=""` can still
  attend an event; their attendance's location is the event's site. The matrix
  `NoLocation` grouping (based on `intake.site`) is unchanged and correct.
- **Walk-ins:** a walk-in is walked into an event; the event was at a location.
  `handleWalkin` already requires an `event_id`. After the change, the walk-in's
  attendance location is the event's site. The walk-in's `intake.site` (home
  site) is still set for roster scoping and stays.
- **Event with no site:** `events.site` is `required`, so this cannot occur for
  new events. If a legacy event somehow has an empty site, the event-derived
  location is empty and the attendance row simply has no location — no special
  handling required.
- **Site with no events:** filtering by that site yields an empty event set, so
  the attendance query returns no rows. The roster still shows the site's
  participants (unchanged), but no attendance cells are filled — correct.
- **Soft-deleted events:** `loadEvents` filters `deleted=false`, so a
  soft-deleted event is excluded from the event set and its attendance is not
  surfaced. Attendance rows referencing a hard-deleted event (cascadeDelete)
  are gone.
- **Migration not yet run:** the Go code must not reference `attendance.site`
  at all. It simply stops reading/writing the field, so it is correct whether or
  not the sibling migration has removed the column.

## Verification Criteria

1. `go build ./... && go vet ./... && go test ./...` all pass.
2. **Filter by location:** with `site=X`, the matrix and export surface only
   attendance whose **event's** site is X. A participant whose home site is Y but
   who attended an event at X appears under X (event-derived), not Y.
3. **Filter by event:** with `event=E`, only E's attendance is surfaced,
   regardless of any site filter.
4. **No-location participant:** a participant with `intake.site=""` who attended
   an event at X still shows that attendance under X.
5. **Walk-in:** recording a walk-in for event E at site X creates an attendance
   row with `event=E` and no `site`; the export `Site` column shows X (from E).
6. **Site with no events:** filtering by that site shows the roster but no
   attendance cells.
7. **No person-to-location link:** verify the attendance query never contains a
   `site=` clause; location is always via the event join.
8. **Export `SiteName`:** the column reflects the event's site, not any stored
   attendance site.
9. **Regression:** existing matrix, toggle, walk-in, export, and
   person-attendance integration tests pass unchanged (or are updated only where
   they asserted on `attendance.site`).
