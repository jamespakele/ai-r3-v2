# Working Plan: Join Attendance Records into the Records Screen Event Filter

## Objective

Fix the Records screen (`handleList` in `r3-intake/internal/server/admin.go`) so that filtering by an event surfaces the people who **attended** that event (per the `attendance` collection), not just those whose `intake.event` (home event) happens to match. The filter must become a **union**: an intake matches the event filter if its home event equals the selected event **OR** it has an attendance record for that event (`attendance.intake == intake.id`).

The verification target: filtering Records by event `lu9ohnxysl9pccf` (R3 - Sprng 2027) must return the 4 people (Cancel Screenshot Gamma, James Pakele, John, John Doe) who have attendance records for that event, even though their `intake.event` is `50wlnz4ttrsk0ql` (R3 - Fall 2026).

## Constraints

- **Language:** Go (server-rendered templates, HTMX + Alpine.js, vanilla CSS).
- **Framework:** PocketBase v0.39 embedded, **JS-style camelCase API** — `app.findCollectionByNameOrId`, `app.save`, `app.delete`; **no `app.dao()`**. Queries use `s.pb.FindRecordsByFilter(col.Id, filter, sort, limit, offset)`. Filter strings are built in Go and escaped with `mcpmod.EscapeFilter` (escapes backslash and double-quote).
- **Timezone:** All timestamps in HST (UTC-10, no DST) via the `hst` fixed zone. Not directly relevant to this change (no time math), but must not be disturbed.
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, 14px card radii, 8px input radii. No visual changes required by this task.
- **Data model:** `intake.event` is a single optional relation to `events` (the intake's **home event**, migration 016). `attendance` has `event` (relation, required after migration 010), `intake` (relation, required), `date`, `status` (present/absent/excused/walk_in), with a unique index on `(event, intake, date)` (migration 013). The join is `attendance.intake == intake.id`.
- **Browser never touches PocketBase** (all PB rules null); Go is the sole policy layer.

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `r3-intake/internal/server/admin.go` | **Modify** | Replace the event-filter block in `handleList` (~lines 100-110) with the attendance-join union logic. |
| `r3-intake/internal/server/records_list_integration_test.go` | **Create** | New integration test proving the event filter joins attendance (mirrors `attendance_export_integration_test.go` patterns). |
| `r3-intake/internal/assets/public/index.html` | **No change** | Template already renders `view.EventFilter` correctly; no markup change needed. |
| `r3-intake/internal/server/attendance.go` | **No change** | Reuses existing `attendanceCollection()` helper; no edits. |
| `r3-intake/pocketbase/migrations/*` | **No change** | No schema change required. |

## Implementation Notes

### Semantics (decision)

The event filter becomes a **UNION**, not strictly attendance-based:

```
(event='<eventID>' || id in ('<intakeID1>','<intakeID2>',...))
```

Rationale:
- **Satisfies the verification:** people who attended the event (per attendance) appear even when their home event differs.
- **No regression for zero-attendance events:** an event with no attendance records still shows its home-event intakes via the `event='<id>'` branch. A strictly attendance-based filter would make a freshly-created event with no dots render an empty Records screen — a confusing regression.
- **Preserves the "home event" semantics** of the Records screen (it is the intake-management screen, not the attendance screen).

**Status handling:** include **all** attendance statuses (present/absent/excused/walk_in) when collecting intake IDs. The verification wording is "have attendance records for that event" — any record links the person to the event. (If product later wants only present/walk_in to count as "attended", add `&& (status='present' || status='walk_in')` to the attendance query — note this as a follow-up, not part of this change.)

### Implementation approach

The current code appends a single string to `parts`, which are joined with ` && `. To preserve composition with the existing status (`?status=`) and search (`?q=`) filters, keep building one string per filter and append the union as a single parenthesized part:

1. Query the `attendance` collection for the event: `event='<eventID>'` (escaped), sort by `intake`, limit 5000 (consistent with existing attendance queries).
2. Collect distinct non-empty `intake` IDs into a slice (dedupe with a `map[string]bool`).
3. If any IDs found, append `(event='<eventID>' || id in ('<id1>','<id2>',...))` to `parts`.
4. If no IDs found (or the attendance query errors), fall back to appending `event='<eventID>'` (home-event match only) — graceful degradation, no crash.
5. Always set `view.EventFilter = eventFilter` (unchanged) so the select's `selected` state and the Clear link still work.

The union part composes correctly with the other parts because `parts` are joined with ` && ` and the union is parenthesized, e.g. `(event='X' || id in ('a','b')) && status='claimed'`.

### Edge cases

- **Dangling attendance.intake:** impossible in practice — `attendance.intake` has `cascadeDelete: true`, so deleting an intake cascades its attendance rows. Still guard with `iid != ""` and dedupe.
- **Soft-deleted intakes:** the `intake` collection has **no** `deleted` field (only sites/users/events do), so no extra filter needed.
- **Large `id in (...)` list:** bounded by the number of distinct participants for a single event (limit 5000, same as existing attendance queries). Acceptable.
- **Attendance query error:** fall back to home-event-only filter rather than failing the whole request.
- **`id in ('...')` syntax:** valid PocketBase v0.39 filter syntax; IDs are record IDs (safe, no escaping needed beyond the existing `EscapeFilter` on the event ID).

## Logical Consequences

Trace of every downstream site that reads/writes/displays `intake.event` or the Records event filter:

| # | Site | Current behavior | Decision | Rationale |
|---|------|------------------|----------|-----------|
| 1 | `?event=` query param (`handleList`) | Filter input | **CHANGE** | Meaning changes from "intake.event == X" to "intake.event == X OR has attendance for X". |
| 2 | `view.EventFilter` (`AdminView`) | Set to eventID | **KEEP** | Still needed for the select's `selected` state and the Clear link. |
| 3 | Template event `<select>` `selected` state (`eq $.EventFilter .ID`, index.html:589) | Highlights chosen event | **KEEP** | No change; `EventFilter` still holds the eventID. |
| 4 | "Clear" link (`if or .Query .StatusFilter .EventFilter`, index.html:591) | Shows when any filter active | **KEEP** | No change; `EventFilter` still set. |
| 5 | Empty-state message (`if or .Query .StatusFilter .EventFilter`, index.html:617) | "No records match your search." | **KEEP** | No change; still driven by the same flags. |
| 6 | Result count (`view.Total = len(view.Rows)`) | Count of returned rows | **KEEP** | Automatically reflects the new union result set. |
| 7 | **Event column** (`row.EventName = s.nameFor("events", rec.GetString("event"))`, admin.go:135) | Shows intake's **home event** | **KEEP** | The Records screen is the intake-management screen; the column should keep showing the intake's home event for context. When filtering by an attended-but-different event, the column correctly shows the person's home event (e.g. R3 - Fall 2026) while the filter surfaced them via attendance. Changing it to the filtered event would be misleading and inconsistent with the intake detail page and form. |
| 8 | Event dropdown data (`view.Events = must(s.loadAllEvents())`) | All non-deleted events | **KEEP** | Must keep listing completed/cancelled events so users can filter by any event that has attendance. |
| 9 | `intake.event` **writes** — `handlers.go:537` (`applySection`), `blankState` default (handlers.go:233), `stateFromRecord` (handlers.go:252) | Home event set/edited on intake form | **KEEP** | Home-event concept is unchanged; this task only changes the Records *filter*, not the data model. |
| 10 | `intake.event` **reads** — `person_attendance.go:183` (intake attendance page header), `person_attendance.go:262`, matrix `cellSiteID` fallback (attendance.go:413) | Home event displayed/used | **KEEP** | Unrelated to the Records filter; home event remains authoritative for these. |
| 11 | `attendance` collection | Schema | **KEEP** | No schema change; only a new read path. |
| 12 | `event_enrollment` junction | Used only for `loadEnrolledCount` (Enrolled count on events admin) and roster tests | **KEEP** | Not used for the Records filter; do not introduce it here. |

**End-to-end data flow (after change):** `GET /?event=<id>` → `handleList` reads `eventFilter` → queries `attendance` for `event='<id>'`, collects distinct `intake` IDs → builds union part → joins with status/search parts → `FindRecordsByFilter` on `intake` → builds `IntakeRow`s (Event column still shows home event) → `view.Total` = row count → renders `list-content` (HTMX) or `list` (full page). The select, Clear link, empty-state, and count all behave identically; only the row set changes.

## Verification Criteria

1. **New integration test** `TestListEventFilterJoinsAttendance` (new file `records_list_integration_test.go`, following `newTestServer`/`seedExportData`/`adminCookie`/`doExport` patterns from `attendance_export_integration_test.go`):
   - Seed an intake **A** whose home event is `ev1` but with an attendance record for `ev2` (analogous to the verification scenario: home `50wlnz4ttrsk0ql`, attended `lu9ohnxysl9pccf`), and an intake **B** whose home event is `ev2`.
   - `GET /?event=ev2` → assert **both A and B** appear in the rendered rows (A via attendance join, B via home event).
   - `GET /?event=ev1` → assert A appears (home event) plus anyone with attendance for `ev1`.
   - Assert the union composes with status: `GET /?event=ev2&status=claimed` returns only claimed intakes among the union.
   - Assert `view.Total` equals the number of returned rows.
2. **Manual/verification scenario:** with the real data, `GET /?event=lu9ohnxysl9pccf` must return Cancel Screenshot Gamma, James Pakele, John, and John Doe (their attendance records for that event), even though their `intake.event` is `50wlnz4ttrsk0ql`.
3. **No regression:** `GET /?event=<event-with-no-attendance>` still returns intakes whose home event matches (fallback branch).
4. **Existing tests still pass:** `go test ./...` in `r3-intake/` — especially `attendance_export_integration_test.go`, `attendance_roster_integration_test.go`, and `person_attendance_test.go`, which must be unaffected (no changes to attendance/export/person-attendance code paths).
5. **Template unchanged:** the event `<select>`, Clear link, empty-state, and result count render identically; only the row set differs.
