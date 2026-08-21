# Working Plan: Require event selection before recording attendance

## Objective

Enforce that an event **must be selected before any attendance record can be
written**, and that every attendance write is scoped to (and stored against)
that event. This card implements the write-path enforcement gate (AC #3) and
event-scoped write association (AC #4) in **this** worktree, which is forked
from master and does **not** contain the parent card's (t_f9a235ad) schema
migration or matrix-scoping work.

The parent card already implemented, in its own worktree (branch
`wt/t_f9a235ad`, commit `7681b12`): a Go migration
`010_attendance_event_required.go` that backfills null-event attendance to a
synthetic per-site "Legacy / Unassigned" event and flips `attendance.event` to
required; a shared `requireEventID` gate (400 "an event must be selected before
recording attendance") on `handleToggle`/`handleWalkin`/
`handlePersonAttendanceDaySave`; uniqueness keyed on `(event, intake, date)`;
matrix disabling of toggles without an event; and a per-person calendar event
selector. The parent will merge all child worktrees at the end, so this card
mirrors that design for the write paths that exist in this worktree.

Core question answered by this card: "Attended what exactly?" → "Attended X
event." Attendance is meaningless without a selected event.

## Constraints

- **PocketBase v0.39 Go API only.** No `app.dao()`. Use `s.pb.FindCollectionByNameOrId`,
  `s.pb.FindRecordsByFilter`, `s.pb.FindRecordById`, `s.pb.Save`, `s.pb.Delete`,
  `core.NewRecord`, `rec.Set`, `rec.GetString`, `rec.Id`.
- **Filter escaping is mandatory.** Every user-supplied value interpolated into
  a PB filter string MUST go through `mcpmod.EscapeFilter(s)` from
  `r3-intake/internal/mcp` (import as `mcpmod "r3-intake/internal/mcp"`).
- **No native unique constraints in PB.** Enforce idempotency in Go by querying
  for an existing full-key record `(event, intake, date)` before creating.
- **All timestamps HST** (`var hst = time.FixedZone("HST", -10*60*60)` in
  admin.go). Dates `time.Now().In(hst).Format("2006-01-02")`.
- **Do NOT add a migration in this worktree.** The schema change (AC #5) and
  data backfill live in the parent's `010_attendance_event_required.go`. This
  card only touches Go handlers + templates. Adding a competing migration here
  would conflict at merge time.
- **Do NOT rework matrix roster scoping (AC #2).** `loadMatrixRows` already
  scopes the attendance filter by `eventID` when non-empty, and the matrix
  template already renders the event selector and passes `event_id` through
  `matrix-cell`/`walkin-panel`/`walkin-results`. The parent owns the "disable
  toggle without an event" UI state. This card's job is the **server-side
  write gate** so a crafted POST cannot bypass the UI.
- **Verification gates:** `cd r3-intake && go build ./... && go vet ./... && go test ./...`
  must all pass.
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page
  bg `#f7f1e6`, 14px card radii, 8px input radii. Reuse `.field-input`,
  `.field-label`, `.field-group`, `.btn`, `.btn-primary`, `.form-error`.

## File Structure

Files touched in **this** worktree (all under `r3-intake/`):

- `internal/server/attendance.go`
  - `handleToggle` (~line 460): add `requireEventID` gate; make the idempotency
    filter always include `event`; always set `event` on create/update.
  - `handleWalkin` (~line 675): read `event_id`; add `requireEventID` gate;
    include `event` in the idempotency filter; set `event` on create/update.
  - Add a small shared helper `requireEventID(w, eventID) bool` (or inline the
    check) returning 400 "an event must be selected before recording attendance"
    when `eventID == ""`.
- `internal/server/person_attendance.go`
  - `handlePersonAttendanceDaySave` (~line 250): read `event_id`; add
    `requireEventID` gate; include `event` in the idempotency filter; set
    `event` on create/update.
  - `buildPersonDayDetailView` (~line 223): load the event list and expose an
    `Events []Event` + `SelectedEventID` on `PersonDayDetailView` so the
    day-detail form can render an event selector.
  - `PersonDayDetailView` struct (~line 66): add `Events []Event` and
    `SelectedEventID string` fields.
- `internal/assets/public/index.html`
  - `person-attendance-day` template (~line 1242): add an event `<select
    name="event_id">` (required) to both the create and edit forms, pre-selecting
    `SelectedEventID` when editing an existing record.
- `internal/server/attendance_test.go` / `person_attendance_test.go` (new tests)
  - Add white-box unit tests for the gate helper and for the idempotency-filter
    construction (assert the filter includes `event='...'`).
  - Add route-level tests via the `newTestServer`/`adminCookie` in-process PB
    harness asserting: POST `/attendance/toggle` with no `event_id` → 400;
    with `event_id` → record created with `event` set; POST `/attendance/walkin`
    with no `event_id` → 400; POST `/intake/{id}/attendance/day` with no
    `event_id` → 400.

No new files are required. No migration file is added (see Constraints).

## Implementation Notes

### 1. Shared gate helper (attendance.go)

Add a small helper near the top of attendance.go (or in a shared spot):

```go
// requireEventID rejects an attendance write that does not name an event.
// Attendance is meaningless without knowing which event the person attended.
func requireEventID(w http.ResponseWriter, eventID string) bool {
    if strings.TrimSpace(eventID) == "" {
        http.Error(w, "an event must be selected before recording attendance", http.StatusBadRequest)
        return false
    }
    return true
}
```

Call it as the **first** validation in each write handler, before any PB
lookup or mutation. This is the server-side enforcement of AC #3 — it cannot be
bypassed by a crafted POST even if the UI is bypassed.

### 2. handleToggle (attendance.go ~460)

- `eventID` is already read from the form (`strings.TrimSpace(r.FormValue("event_id"))`).
- After the existing `intakeID == "" || date == ""` check, add:
  `if !requireEventID(w, eventID) { return }`.
- The idempotency filter currently only appends ` && event='...'` when
  `eventID != ""`. Since the gate guarantees non-empty, make the filter
  **always** include the event term:
  ```go
  filter := fmt.Sprintf("intake='%s' && date='%s' && event='%s'",
      mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(date), mcpmod.EscapeFilter(eventID))
  ```
  This keys uniqueness on `(event, intake, date)` (AC #4) and prevents a
  toggle in one event from clobbering a record in another.
- In the create branch, always set `rec.Set("event", eventID)` (drop the
  `if eventID != ""` guard). In the update branch, also set
  `existing.Set("event", eventID)` so a record is never left with a stale/null
  event.
- The `MatrixCell` returned still carries `EventID: eventID` (already set).

### 3. handleWalkin (attendance.go ~675)

- `handleWalkin` currently does **not** read `event_id` at all. Add at the top,
  alongside the site resolution:
  ```go
  eventID := strings.TrimSpace(r.FormValue("event_id"))
  if !requireEventID(w, eventID) { return }
  ```
- The idempotency filter is currently `intake='%s' && date='%s'`. Change it to
  include the event:
  ```go
  filter := fmt.Sprintf("intake='%s' && date='%s' && event='%s'",
      mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(today), mcpmod.EscapeFilter(eventID))
  ```
- In both the update and create branches, set `rec.Set("event", eventID)`.
- The 303 redirect already forwards `event_id` → `event=` query param; keep it.

### 4. handlePersonAttendanceDaySave (person_attendance.go ~250)

- Add `eventID := strings.TrimSpace(r.FormValue("event_id"))` and gate it:
  `if !requireEventID(w, eventID) { return }` (place before the date/status
  validation so a missing event is the first error surfaced).
- The idempotency filter is currently `intake='%s' && date='%s'`. Change to
  include the event:
  ```go
  filter := fmt.Sprintf("intake='%s' && date='%s' && event='%s'",
      mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(date), mcpmod.EscapeFilter(eventID))
  ```
- In both the update and create branches, set `rec.Set("event", eventID)`.
- `handlePersonAttendanceDayDelete` (~line 318) deletes by `(intake, date)`.
  Leave it as-is for now (deleting a day removes the record regardless of
  event); note in a comment that if per-event delete is desired later it should
  also key on `event`. The parent's design keys uniqueness on `(event, intake,
  date)`, so a delete that omits the event could match multiple rows — keep the
  `limit 1` and document this. (Optional hardening: also read `event_id` and
  include it in the delete filter for precision.)

### 5. PersonDayDetailView + day-detail template (AC #4 UI)

- Add to `PersonDayDetailView`:
  ```go
  Events          []Event
  SelectedEventID string
  ```
- In `buildPersonDayDetailView`, load the events collection (mirror how the
  matrix loads `s.Events` — via `s.pb.FindRecordsByFilter` on the `events`
  collection, sorted by name) and populate `view.Events`. When an existing
  record is loaded, set `view.SelectedEventID = rec.GetString("event")`.
- In the `person-attendance-day` template, add an event selector to **both**
  the create and edit forms, before the Status field:
  ```html
  <div class="field-group">
    <label class="field-label">Event</label>
    <select name="event_id" class="field-input" required>
      <option value="">Select an event…</option>
      {{range .Events}}<option value="{{.ID}}" {{if eq $.SelectedEventID .ID}}selected{{end}}>{{.Name}}</option>{{end}}
    </select>
  </div>
  ```
  The `required` attribute gives a client-side guard; the Go gate is the
  authoritative server-side guard.

### 6. Tests

- **Unit (no DB):** test `requireEventID` returns false + writes 400 for empty
  input, true for non-empty. If the filter construction is extracted into a
  helper, unit-test that it always includes `event='...'`.
- **Route-level (in-process PB via `newTestServer`/`adminCookie`):**
  - POST `/attendance/toggle` with `intake_id`, `date`, `site_id` but **no**
    `event_id` → expect 400 and **no** attendance record created.
  - POST `/attendance/toggle` with a valid `event_id` → expect 200 and a record
    whose `event` equals the posted id.
  - POST `/attendance/walkin` with no `event_id` → 400, no record.
  - POST `/intake/{id}/attendance/day` with no `event_id` → 400, no record.
  - POST `/intake/{id}/attendance/day` with `event_id` → record created with
    `event` set.
  - Idempotency: two toggles for the same `(event, intake, date)` produce a
    single record (query count == 1).

## Verification Criteria

1. `cd r3-intake && go build ./...` passes.
2. `cd r3-intake && go vet ./...` passes.
3. `cd r3-intake && go test ./...` passes (all existing + new tests green).
4. **AC #3 (enforcement):** A POST to any of `handleToggle`, `handleWalkin`, or
   `handlePersonAttendanceDaySave` with an empty/missing `event_id` returns
   HTTP 400 with body "an event must be selected before recording attendance"
   and writes **no** attendance record. Verified by route-level tests.
5. **AC #4 (association):** Every attendance record created/updated through the
   three write paths has its `event` field set to the posted `event_id`, and the
   idempotency lookup keys on `(event, intake, date)` so records for different
   events never collide. Verified by asserting `rec.GetString("event")` and by
   the single-record idempotency test.
6. **AC #1 / AC #2 (context, owned by parent):** The matrix participant list
   renders identically regardless of selected event and the attendance filter
   is scoped to the event — already present in `loadMatrixRows`/templates in
   this worktree; the parent's merge completes the "disable toggle without an
   event" UI state. This card does not regress these.
7. **AC #5 (migration, owned by parent):** No migration is added in this
   worktree; the parent's `010_attendance_event_required.go` handles backfill +
   required flip. Merge coherence is preserved by not duplicating it.
8. **No regression:** The per-person calendar still renders and saves when an
   event is selected; the matrix toggle and walk-in still work when an event is
   selected; the day-detail form shows the new event selector.
