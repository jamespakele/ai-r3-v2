# Working Plan: Event-Scoped Attendance Schema and Migration

## Objective

Make attendance records **event-scoped** in the R3 Intake app. Today a single
attendance matrix tracks participants independent of events, and attendance can
be recorded with **no event** (the `attendance.event` relation is nullable and
several handlers create records without setting it). After this change:

- The participant roster renders identically regardless of the selected event.
- Selecting an event scopes the attendance records shown and edited to that event.
- Attendance **cannot be recorded until an event is selected** (server-enforced).
- Every attendance record is associated with the event it was recorded for
  (`attendance.event` becomes required / non-null).
- Existing null-event records are migrated so no data is lost.

This is a **design-only** card. It produces the schema + migration + handler
contract for a sibling implementation card. No source code is written here.

## Constraints

- **Go is the policy layer.** The browser never talks to PocketBase directly; all
  collection rules are `null` (locked). All enforcement happens in Go handlers.
- **PocketBase v0.39 Go API only** (no `app.dao()`):
  `s.pb.FindCollectionByNameOrId`, `s.pb.FindRecordsByFilter(col.Id, filter, sort,
  limit, offset)`, `s.pb.FindRecordById`, `s.pb.Save`, `s.pb.Delete`,
  `core.NewRecord(col)`, `rec.Set`, `rec.GetString`, `rec.Id`.
- **Filter escaping:** every user-supplied value interpolated into a PB filter
  string must go through `mcpmod.EscapeFilter(s)` from `r3-intake/internal/mcp`
  (import `mcpmod "r3-intake/internal/mcp"`).
- **Time:** `var hst = time.FixedZone("HST", -10*60*60)`; dates
  `time.Now().In(hst).Format("2006-01-02")`.
- **Templates:** single embedded `index.html` with `{{define}}` blocks; new blocks
  go at the END of the file. HTMX partial swaps return raw HTML fragments.
  No-JS fallback = POST -> 303 redirect.
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg
  `#f7f1e6`, 14px card radii, 8px input radii. Reuse `.btn`, `.btn-ghost`,
  `.btn-tiny`, `.btn-danger`, `.status-badge`, `.field-input`, `.form-error`,
  `.empty-state`, `.matrix-dot`, `.dot-disabled`, `.dot-present`, `.dot-empty`.
- **Verification gates:** `go build ./...`, `go vet ./...`, `go test ./...` must
  pass.
- **Migration registration:** Go migrations live in
  `r3-intake/pocketbase/migrations/` and are registered in
  `r3-intake/pocketbase/migrations/002_encryption.go` via
  `migrations.Register(upX, downX, "NNN_name.go")`. The next Go migration is
  `010_attendance_event_required.go`. The mirrored copy at
  `r3-intake/internal/migrations/pocketbase/migrations/` is older (up to 008) and
  is **not** the canonical location — do not edit it for this change.

## Current State

### Schema (`007_events_attendance.js`)

`attendance` collection:
- `event` relation -> events, maxSelect 1, **required: false (nullable)**,
  cascadeDelete: true
- `intake` relation -> intake, required: true, cascadeDelete: true
- `site` relation -> sites, required: true (made optional by `009`),
  cascadeDelete: false
- `recorded_by` relation -> users, optional
- `date` text, required, max 20 (YYYY-MM-DD)
- `status` select: present/absent/excused/walk_in, required
- `check_in_time` text optional, `note` text optional, created/updated autodate

`events` collection: `site` (relation, required), `name`, `start_date`,
`end_date`, `description`, `status` (active/completed/cancelled), `created_by`,
`created`, `updated`.

`event_enrollment` junction: `event` (relation, cascadeDelete true), `intake`
(relation, cascadeDelete true), `enrolled_date`, `deleted` (soft-delete flag from
`008`), `created`.

### Behavior (`attendance.go`, `person_attendance.go`)

- **Matrix GET** (`handleMatrix`): optional `event` query param. When
  `eventID == ""`, rows = ALL intakes (site-scoped), attendance map keyed by
  `(intake, date)` only. When `eventID != ""`, rows = enrolled participants +
  walk-ins for that event, attendance filtered by `event='<id>'`.
- **`handleToggle`**: finds existing record by filter
  `intake='<id>' && date='<date>'` (+ `&& event='<id>'` if eventID set). If no
  event selected, it **creates a record WITHOUT setting event** (event stays
  null). This is the core problem.
- **`handleWalkin`**: creates attendance with no event set (`event_id` only passed
  through if present).
- **`handlePersonAttendanceDaySave`** (per-person calendar, Epic 4): creates /
  updates attendance by `(intake, date)` with **no event at all** — event is never
  set.
- **`handleExportCSV` / `loadExportRows`**: filters by date range + optional site
  + optional event.
- **Matrix template** (`matrix-content`): event dropdown with option value=""
  labeled "All dates — no event filter". Cells carry a hidden `event_id` only when
  `EventID` is set. Cells are disabled only when the participant has no location
  (`Disabled: cellSiteID == ""`).
- **Per-person calendar** (`person-attendance-calendar` / `person-attendance-day`):
  loads all records for `(intake, month)` regardless of event; the day-detail form
  has no event selector; `buildPersonDayDetailView` only *displays* the resolved
  `EventName` of an existing record.

### Root cause

`attendance.event` is nullable and three write paths (`handleToggle`,
`handleWalkin`, `handlePersonAttendanceDaySave`) can create records with
`event = null`. The matrix's "All dates — no event filter" mode is the only place
that surfaces these null-event records.

## Target Schema

`attendance.event` becomes **required (non-null)**, maxSelect 1, relation ->
events, cascadeDelete: true. All other fields unchanged.

Every attendance record is therefore uniquely identified by
`(event, intake, date)` — the FR2 uniqueness key. PocketBase has no native unique
constraint, so this is enforced in Go (see Implementation Notes).

## Data Migration Plan

New Go migration `r3-intake/pocketbase/migrations/010_attendance_event_required.go`,
registered in `002_encryption.go` as
`migrations.Register(upAttendanceEventRequired, downAttendanceEventRequired, "010_attendance_event_required.go")`.

### Backfill strategy (decision)

**Create a synthetic "Legacy / Unassigned" event per site and assign every
null-event attendance record to it.** This is the safest data-preserving option:
it keeps every existing record, gives each a real event association (satisfying
"attendance records are associated with the event they were recorded for"), and
avoids guessing which real event a historical record "belonged" to (records were
created without event context, so any real-event assignment would be fabricated).

### `upAttendanceEventRequired(app core.App) error`

1. Load the `attendance` collection. If `event` field is already required, no-op
   (idempotent).
2. Query all attendance records with `event=''` (null event):
   `s.pb.FindRecordsByFilter(attCol.Id, "event=''", "date", 100000, 0)`.
   (PB treats a null relation as empty string in filters.)
3. Group the null-event records by their `site` value. For each distinct site
   that has null-event records:
   - Load the `events` collection and the `sites` collection.
   - Create one synthetic event:
     - `name`: `"Legacy / Unassigned"`
     - `site`: the site id
     - `start_date`: min `date` across that site's null-event records
     - `end_date`: max `date` across that site's null-event records
     - `status`: `"completed"`
     - `description`: `"Auto-created by migration 010 to preserve pre-event-scoped attendance records."`
     - `created_by`: empty (no user)
     - Save via `s.pb.Save`.
   - For each null-event record in that site group, `rec.Set("event", legacyEvent.Id)`
     and `s.pb.Save(rec)`.
4. After all null-event records are assigned, flip the field:
   `f := col.Fields.GetByName("event")`; if `rf, ok := f.(*core.RelationField); ok && !rf.Required { rf.Required = true; s.pb.Save(col) }`.
5. Return nil. If any step errors, return the error (migration fails loudly and
   rolls back the whole migration — PB applies migrations atomically per file).

> Note: a site with null-event records but **no** `events`/`sites` row (should not
> happen given cascadeDelete:false on site) — if the site lookup fails, fall back
> to a single site-less legacy event only if the `events.site` field allows it;
> otherwise return the error. In practice every attendance record has a site, so
> this is defensive only.

### `downAttendanceEventRequired(app core.App) error`

1. Load `attendance`; if `event` is required, set `rf.Required = false` and save.
2. Best-effort: delete the synthetic `"Legacy / Unassigned"` events created by
   the up migration (query `events` by `name='Legacy / Unassigned'`). Because
   `attendance.event` has `cascadeDelete: true`, deleting these events would
   cascade-delete the attendance records assigned to them — so **only** delete the
   legacy events if the operator explicitly wants to drop that data. Default:
   leave them in place (they are harmless, status=completed) and document that
   down is best-effort and does not re-null event fields.

### Ordering / idempotency

- The migration runs after `009` (site optional) and after `008` (enrollment
  soft-delete). It does not depend on either beyond the collections existing.
- Guard every step so re-running is safe (check `rf.Required` before flipping;
  skip creating a legacy event if one already exists for that site).

## Implementation Notes

### 1. Enforce event selection before recording (all write paths)

Add a shared validation helper in `attendance.go`:

```go
func requireEventID(eventID string) error {
    if strings.TrimSpace(eventID) == "" {
        return errors.New("an event must be selected before recording attendance")
    }
    return nil
}
```

- **`handleToggle`**: after parsing `event_id`, if `requireEventID(eventID)` fails,
  respond `http.Error(w, "an event must be selected before recording attendance", http.StatusBadRequest)`.
  This is the primary gate — the matrix will no longer render toggles when no
  event is selected, but the server must still reject a forged/empty POST.
- **`handleWalkin`**: same check on `event_id` before resolving site / creating the
  record. Reject with 400.
- **`handlePersonAttendanceDaySave`**: same check on `event_id` before the
  upsert. On failure, render the day fragment with an error via
  `renderDayError(w, intake, date, "an event must be selected before recording attendance")`
  (400), consistent with existing validation-error handling.

### 2. Unique constraint (FR2) — always key on (event, intake, date)

Every write path must query for an existing record using the full key:

```go
filter := fmt.Sprintf("event='%s' && intake='%s' && date='%s'",
    mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(date))
```

- **`handleToggle`**: replace the current conditional filter (which only appends
  `&& event='...'` when eventID is set) with the unconditional full-key filter.
  Since event is now required, the filter is always complete.
- **`handleWalkin`**: same — always include `event='<id>'` in the idempotent
  upsert filter.
- **`handlePersonAttendanceDaySave`**: same — always include `event='<id>'`.
- **`handlePersonAttendanceDayDelete`**: update its filter to
  `event='<id>' && intake='<id>' && date='<date>'` so it deletes the record for the
  currently selected event, not an arbitrary one.
- **`buildPersonDayDetailView`**: update its lookup filter to include
  `event='<id>'` so the day-detail modal shows the record for the selected event.

### 3. Matrix behavior when no event is selected (decision)

**Show the full participant roster (identical to today's "All dates" mode) but
disable all toggling, with a clear prompt to select an event.**

- `loadMatrixRows` keeps its existing `eventID == ""` branch (rows = all intakes,
  site-scoped) so the roster renders identically regardless of selection.
- Add a view flag to the matrix view model, e.g. `EventRequired bool` (true when
  `EventID == ""`), and set `MatrixCell.Disabled = true` for every cell when no
  event is selected (in addition to the existing no-location disable).
- **Template change** (`matrix-content`): when `EventRequired` is true, render a
  banner above the table, e.g.
  `<div class="form-error">Select an event to record attendance.</div>`, and
  render each cell as a disabled dot (reuse `.matrix-dot .dot-disabled`) instead
  of a submit button. The `matrix-cell` template already branches on `.Disabled` —
  extend the condition so `Disabled` is true when no event is selected.
- **Walk-in panel**: when `EventRequired` is true, hide the "Add walk-in" button /
  panel (or render it disabled with the same prompt). The server still rejects
  walk-in POSTs without `event_id` as a backstop.
- **Event dropdown label**: change the `value=""` option label from
  "All dates — no event filter" to something that signals the new requirement,
  e.g. "Select an event…". This makes the "must select an event" rule visible.

### 4. Per-person calendar (Epic 4) event scoping (decision)

**Add an event selector to the per-person attendance page and scope the calendar
and day-detail to the selected event.** Keep it minimal and consistent with the
matrix.

- Add `Events []EventOption` and `EventID string` to `PersonAttendanceView` and
  `PersonDayDetailView` (mirror the matrix's event dropdown pattern).
- `handlePersonAttendance` reads an `event` query param (default: the most recent
  `active` event for the intake's site, falling back to the site's
  "Legacy / Unassigned" event if none; if neither exists, default to empty and
  require selection).
- `buildPersonAttendanceView` filters the month's records by
  `intake='<id>' && event='<id>' && date>='<first>' && date<='<last>'`.
- The `person-attendance-calendar` template gets an event dropdown (same
  `hx-get`/`hx-target` swap pattern as the matrix) so changing the event re-scopes
  the calendar.
- The `person-attendance-day` form includes a hidden `event_id` (from the selected
  event) and, when no event is selected, renders a disabled state with the
  "Select an event to record attendance" prompt. `handlePersonAttendanceDaySave`
  requires `event_id` (see note 1).

### 5. Export CSV

`loadExportRows` already filters by optional `event`. No schema change needed.
Optionally, when `eventID == ""`, the export now returns only records that have an
event (all of them, post-migration). No code change required beyond the migration
guaranteeing no null-event records remain.

### 6. Files to touch (implementation card)

- `r3-intake/pocketbase/migrations/010_attendance_event_required.go` (NEW)
- `r3-intake/pocketbase/migrations/002_encryption.go` (register the migration)
- `r3-intake/internal/server/attendance.go` (requireEventID helper; handleToggle,
  handleWalkin, loadMatrixRows, handleExportCSV/loadExportRows; matrix view flag)
- `r3-intake/internal/server/person_attendance.go` (handlePersonAttendanceDaySave,
  handlePersonAttendanceDayDelete, buildPersonDayDetailView, buildPersonAttendanceView,
  view structs)
- `r3-intake/internal/assets/public/index.html` (matrix-content, matrix-cell,
  walkin-panel, person-attendance-calendar, person-attendance-day)
- `r3-intake/internal/server/attendance_test.go`,
  `r3-intake/internal/server/person_attendance_test.go` (tests)

## Verification Criteria

- `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- **Migration:** after boot, `attendance.event` is required (non-null). Every
  pre-existing null-event attendance record is assigned to a synthetic
  "Legacy / Unassigned" event for its site; no attendance record has `event=''`.
- **Roster invariance:** the matrix participant list renders identically whether
  an event is selected or not.
- **Event scoping:** selecting an event shows only that event's attendance
  records; switching events changes the records shown but not the roster.
- **Gate enforcement:** with no event selected, matrix cells are disabled and the
  walk-in panel is hidden; a direct POST to `/attendance/toggle`,
  `/attendance/walkin`, or `/intake/{id}/attendance/day` without `event_id`
  returns 400 with the message "an event must be selected before recording
  attendance".
- **Association:** every newly created attendance record has a non-empty `event`.
- **Uniqueness:** toggling the same `(event, intake, date)` twice updates the
  existing record rather than creating a duplicate (idempotent upsert keyed on the
  full triple).
- **Per-person calendar:** the calendar and day-detail are scoped to the selected
  event; saving a day requires an event and records it against that event.
- **Export:** CSV export includes the event column and returns only event-scoped
  records.
