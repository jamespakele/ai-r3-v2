# Working Plan: Event / Location / Attendance Relationships

## Objective

Design the schema and relationship rules that remove the redundant
participant-to-location link from the R3 Intake attendance app, so that a
location is always derived from the event, never stored on the person or on an
attendance row. This card produces a **design document only** — no code. Two
child cards implement it:

1. **Schema migration** — enforce event-location and attendance relationships
   (remove `attendance.site`, keep `events.site` required, keep `intake.site`
   as the participant's home site).
2. **Filtering logic** — update the Go attendance filters to derive location
   from the event instead of reading `attendance.site`.

The design must be precise enough that those implementers can act on it
directly.

### What the design must achieve

- **One event, one location.** `events.site` is the single source of truth for
  where an event happens. It is already `required` and stays that way.
- **Attendance is per event, never per location.** An attendance row belongs to
  an event; its location is the event's site, resolved by join. No location is
  stored on the attendance row.
- **No participant-to-location relationship.** `intake.site` (the person's home
  site) remains for roster scoping and is NOT the event location. There is no
  other link between a person and a location.
- **Filtering by location surfaces only events at that location** (and their
  attendance). Filtering by event surfaces that event and its attendance. No
  filter combination implies a person is directly tied to a location.
- **One attendance record = person_id + event_id + date** (already enforced by
  the unique index `idx_attendance_event_intake_date`).

## Current Schema Analysis

### Collections

| Collection | Fields (relevant) | Notes |
|---|---|---|
| `sites` | `name`, `address`, `active`, `sort_order`, `is_default`, `deleted` | The location table. |
| `events` | `site` (relation, **required**), `name`, `start_date`, `end_date`, `description`, `status`, `created_by`, `deleted` | `site` is the event's location. |
| `event_enrollment` | `event`, `intake`, `enrolled_date`, `deleted` | Junction; unique `(event, intake)` where `deleted=false`. |
| `attendance` | `event` (relation, **required** since 010), `intake` (relation, required), `site` (relation, **optional** since 009), `recorded_by`, `date`, `status`, `check_in_time`, `note` | **`site` is the redundant field.** |
| `intake` | `site` (person's **home** site), `name`, `assigned_to`, `status`, ... | `intake.site` is the participant's assigned site — must stay. |

### Root cause of the inconsistent filtering

The `attendance` collection carries a direct `site` relation field that is
stored independently of the event. Because it is written from several different
sources (the intake's home site in `handleToggle`/day-detail, the resolved site
in `handleWalkin`), it can diverge from the event's site. The Go filter in
`attendance.go` `loadMatrixRows` reads it directly:

```go
attFilter := fmt.Sprintf("date>='%s' && date<='%s'", ...)
if siteID != "" {
    attFilter += fmt.Sprintf(" && (site='' || site='%s')", mcpmod.EscapeFilter(siteID))
}
```

This filters attendance by `attendance.site`, which conflicts with the
event-derived location. The same pattern appears in `loadExportRows`
(`attendance.go` ~878) and the export `SiteName` column reads
`rec.GetString("site")` (~930). This redundant participant-to-location
relationship is exactly what the epic wants removed.

### Where `attendance.site` is touched today

**Writes (to be removed):**
- `attendance.go:574` — `handleToggle` insert: `rec.Set("site", siteID)`.
- `attendance.go:774` — `handleWalkin` update: `existing.Set("site", siteID)`.
- `attendance.go:786` — `handleWalkin` insert: `rec.Set("site", siteID)`.
- `attendance.go:797` — `handleWalkin` update-after-race: `recs[0].Set("site", siteID)`.
- `person_attendance.go:326` — day-detail insert: `rec.Set("site", intake.GetString("site"))`.

**Reads (to be replaced with event-derived location):**
- `attendance.go:303` — `loadMatrixRows` filter `(site='' || site='%s')`.
- `attendance.go:878` — `loadExportRows` filter `site='%s'`.
- `attendance.go:930` — `loadExportRows` `SiteName: s.nameFor("sites", rec.GetString("site"))`.

**Reads of `intake.site` (the person's home site — MUST STAY):**
- `attendance.go:242` — `resolveSite` case-manager site derivation.
- `attendance.go:331` — `loadMatrixRows` `cellSiteID` (NoLocation grouping).
- `attendance.go:520,525` — `handleToggle` site derivation from intake.
- `person_attendance.go:183` — person calendar `SiteName` display.

**`handleWalkin` line 741** sets `site` on the **intake** record (the walk-in's
home site), not on attendance — this MUST STAY.

## Target Data Model

### Relationship rules (epic-enforced)

1. **Events restricted to a location.** One event, one location; the location
   is inherent to the event. `events.site` is `required` and is the only place
   an event's location lives.
2. **Attendance tracked per event, not per location.** An attendance row
   references an event; its location is the event's site, resolved by join.
   Location is never connected to the person — the event is.
3. **Walk-ins walked into an EVENT.** That event was at a location; the location
   is derived from the event, never stored on the person.
4. **NO table relationship between people/participants and locations.** The
   only relationships are people-to-events and locations-to-events. The location
   table connects to the event table, never to the participant table.
5. **One attendance record = person_id + event_id + date.**

### Field changes per collection

**`sites` (locations)** — no change. It is the location table.

**`events`** — no change. `site` stays `required` (one event, one location).

**`event_enrollment`** — no change. Junction between people and events.

**`attendance`** — **remove the `site` relation field entirely.** The location
is derived from the event via join. Remaining fields: `event` (required),
`intake` (required), `recorded_by`, `date`, `status`, `check_in_time`, `note`,
`created`, `updated`. The unique index
`idx_attendance_event_intake_date` on `(event, intake, date)` is unchanged.

**`intake`** — no change. `site` is the participant's **home** site, used for
roster scoping and case-manager site derivation. It is NOT the event location
and must remain.

### Recommendation: remove `attendance.site` entirely

Remove the field rather than keeping it read-only/derived. Rationale:

- The epic explicitly forbids any participant-to-location relationship. A
  stored `attendance.site` is exactly that redundant link, even if the Go code
  stops writing it — it can still diverge from the event's site and re-introduce
  the inconsistency (e.g. legacy rows, direct DB edits, future code paths).
- Keeping a derived/read-only field adds schema surface with no benefit: the
  value is always `events.site` for the row's event, so it is pure duplication.
- Removing it forces every read/write path to go through the event, which is the
  single source of truth. This is the cleanest option and matches the epic's
  "no participant-to-location relationship" rule.

The cost is updating the Go code that reads/writes it (enumerated above). That
is the filtering child card's job and is bounded.

## Migration Design

New Go migration `015_attendance_remove_site.go` (register in `migrations.go`).
It must run **after** 010 (event required) and 013 (unique indexes), which it
does by numbering.

### What the migration must do

1. **Idempotency guard.** If `attendance.site` is already absent, no-op.
2. **Backfill safety check.** Before removing the field, verify every attendance
   row's `site` matches its event's `site`. Because `attendance.event` is
   required (010) and `events.site` is required, the event-derived location is
   always defined. If any row's `attendance.site` differs from its event's
   `site`, log a warning (do not fail) — the event's site is authoritative and
   the divergent stored value is discarded. This is a data-integrity note, not a
   blocker: the epic's rule is that the event is the source of truth.
3. **Remove the field.** `col.Fields.RemoveByName("site")` then `app.Save(col)`.
4. **Down migration.** Re-add `attendance.site` as an optional relation to
   `sites` (matching 009's state) and backfill it from each row's event's site
   so the down path is lossless. Best-effort: if an event lookup fails, leave
   the field empty (it is optional).

### Index changes

- `idx_attendance_event_intake_date` on `(event, intake, date)` — **unchanged**.
  It does not reference `site`, so removing the field does not affect it.
- No new indexes required. Filtering by location now goes through the event
  join, so the existing event index (implicit on the relation) suffices.

### Backfill summary

- No data backfill is needed for correctness: the event-derived location is
  always available via `events.site`. The migration only discards the redundant
  stored `attendance.site` value.
- The down migration backfills `attendance.site` from `events.site` for
  reversibility.

## Filtering Logic Design

The Go filtering must derive location from the event, never from
`attendance.site`. The core change: **filter attendance by event, and filter
events by site** — two separate queries joined in Go.

### Principle

- **Location filter** applies to the **events** query (`events.site = X`),
  producing the set of event IDs at that location.
- **Attendance filter** applies to the **attendance** query by `event` (in that
  set) and by `date`. It never references a site field.

### `loadMatrixRows` (attendance.go ~270)

Current buggy filter:

```go
attFilter := fmt.Sprintf("date>='%s' && date<='%s'", ...)
if siteID != "" {
    attFilter += fmt.Sprintf(" && (site='' || site='%s')", mcpmod.EscapeFilter(siteID))
}
if eventID != "" {
    attFilter += fmt.Sprintf(" && event='%s'", mcpmod.EscapeFilter(eventID))
}
```

New logic:

1. Resolve the set of event IDs to include:
   - If `eventID != ""`: the set is `{eventID}`.
   - Else if `siteID != ""`: query `events` with
     `status='active' && deleted=false && site='<siteID>'` and collect the
     event IDs. (Reuse `loadEvents(siteID)` which already builds exactly this
     filter.)
   - Else (admin, all locations): no event restriction.
2. Build the attendance filter:
   - Always: `date>='<from>' && date<='<to>'`.
   - If the event set is non-empty, add
     `&& event IN ('<id1>','<id2>',...)`. If the set is empty (a site with no
     events), the attendance query returns no rows — correct, because there are
     no events at that location.
   - **Remove the `(site='' || site='%s')` clause entirely.**

The participant roster (`intakeFilter`) is unchanged — it still scopes by
`intake.site` (home site) for case managers and site-filtered admins. This is
correct: the roster is the set of people to display, independent of the event.

### `loadExportRows` (attendance.go ~878)

Current:

```go
if siteID != "" {
    filter += fmt.Sprintf(" && site='%s'", mcpmod.EscapeFilter(siteID))
}
if eventID != "" {
    filter += fmt.Sprintf(" && event='%s'", mcpmod.EscapeFilter(eventID))
}
```

New: same event-set approach. Resolve the event set from `eventID` or from
`events.site = siteID`, then filter attendance by `event IN (...)` and `date`.
Remove the `site='%s'` clause.

### Export `SiteName` column (attendance.go ~930)

Current: `SiteName: s.nameFor("sites", rec.GetString("site"))`.

New: resolve the event's site and name it:

```go
eventRec, err := s.pb.FindRecordById(eventsCol.Id, rec.GetString("event"))
siteName := ""
if err == nil {
    siteName = s.nameFor("sites", eventRec.GetString("site"))
}
```

### Write paths (remove `Set("site", ...)`)

- `attendance.go:574` (`handleToggle` insert) — drop `rec.Set("site", siteID)`.
- `attendance.go:774,786,797` (`handleWalkin`) — drop `Set("site", siteID)`.
- `person_attendance.go:326` (day-detail insert) — drop
  `rec.Set("site", intake.GetString("site"))`.

The `siteID` variable in `handleToggle`/`handleWalkin` is still used for the
`Disabled` flag and the redirect query string — keep it. It is no longer written
to the attendance row.

### `intake.site` reads that MUST stay

- `attendance.go:242` — `resolveSite` case-manager site derivation.
- `attendance.go:331` — `loadMatrixRows` `cellSiteID` (NoLocation grouping).
- `attendance.go:520,525` — `handleToggle` site derivation from intake.
- `person_attendance.go:183` — person calendar `SiteName` display.
- `attendance.go:741` — `handleWalkin` sets `site` on the **intake** record.

These are the participant's home site for roster scoping, not the event
location. Do not touch them.

## Edge Cases

- **Legacy data.** Pre-010 null-event attendance was backfilled into synthetic
  "Legacy / Unassigned" events (010). Those events have a `site`, so their
  attendance resolves a location via the event. The migration discards any
  divergent stored `attendance.site`; the event's site is authoritative.
- **No-location participants.** A participant with no home site
  (`intake.site = ""`) can still attend an event. Their attendance row's
  location is the event's site, not their (empty) home site. The matrix
  `NoLocation` grouping (based on `intake.site`) is unchanged and correct — it
  reflects the person's roster scoping, not the event location.
- **Walk-ins.** A walk-in walked into an event; the event was at a location.
  `handleWalkin` already requires an `event_id` and resolves the site for the
  redirect. After the change, the walk-in's attendance location is the event's
  site. The walk-in's `intake.site` (home site) is set for roster scoping and
  stays.
- **Event with no site.** `events.site` is `required`, so this cannot occur for
  new events. For robustness, if a legacy event somehow has an empty site, the
  event-derived location is empty and the attendance row simply has no location
  (matches the current "no location" behavior). No special handling required.
- **Site with no events.** Filtering by that site yields an empty event set, so
  the attendance query returns no rows. The roster still shows the site's
  participants (unchanged), but no attendance cells are filled — correct.
- **Event deleted / soft-deleted.** `loadEvents` filters `deleted=false`, so a
  soft-deleted event is excluded from the event set and its attendance is not
  surfaced. Attendance rows referencing a hard-deleted event (cascadeDelete)
  are gone.

## Verification Criteria

### Schema migration

1. After migration 015, `attendance` has **no** `site` field; `events.site` is
   still `required`; `intake.site` is unchanged.
2. `idx_attendance_event_intake_date` on `(event, intake, date)` still exists.
3. Down migration re-adds `attendance.site` (optional) and backfills it from
   each row's event's site.
4. `go build ./... && go vet ./... && go test ./...` pass.

### Filtering logic

5. **Filter by location:** with `site=X`, the matrix and export surface only
   attendance whose event's site is X. A participant whose home site is Y but
   who attended an event at X appears under X (event-derived), not Y.
6. **Filter by event:** with `event=E`, only E's attendance is surfaced,
   regardless of any site filter.
7. **No-location participant:** a participant with `intake.site=""` who attended
   an event at X still shows that attendance under X.
8. **Walk-in:** recording a walk-in for event E at site X creates an attendance
   row with `event=E` and no `site`; the export `Site` column shows X (from E).
9. **Site with no events:** filtering by that site shows the roster but no
   attendance cells.
10. **No filter combination implies a person-to-location link:** verify the
    attendance query never contains a `site=` clause; location is always via the
    event join.

### Regression

11. Existing matrix, toggle, walk-in, export, and person-attendance integration
    tests pass unchanged (or are updated only where they asserted on
    `attendance.site`).
