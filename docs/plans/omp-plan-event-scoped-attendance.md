# Working Plan: Event-Scoped Attendance Schema and Data Migration

> **For Hermes:** Use subagent-driven-development to implement this plan task-by-task. omp executes; this is the blueprint.

**Goal:** Make `attendance.event` a required (non-null) relation, backfill existing null-event records to a synthetic per-site "Legacy / Unassigned" event, and enforce event-scoping across every attendance write/read path so uniqueness is always keyed on `(event, intake, date)`.

**Architecture:** A new Go migration (`010_attendance_event_required.go`) backfills null-event attendance rows into per-site synthetic legacy events, then flips the `event` field to required. A shared `requireEventID` gate is added to all attendance write handlers. The matrix and per-person calendar gain event-scoped behavior (matrix disables toggling without an event; per-person calendar gains an event selector). All uniqueness filters become full-key `(event, intake, date)`.

**Tech Stack:** Go 1.25, PocketBase v0.39.9 (Go API only), server-rendered Go templates, HTMX + Alpine.js, vanilla CSS.

---

## Objective

Implement the event-scoped attendance schema change and data migration in the R3 Intake Go app:

1. **Schema:** `attendance.event` becomes REQUIRED (non-null) via a new Go migration `010_attendance_event_required.go`, registered in `002_encryption.go`.
2. **Backfill:** Null-event attendance records are assigned to a synthetic per-site "Legacy / Unassigned" event before the field is flipped to required.
3. **Enforcement:** A shared `requireEventID(eventID)` gate (400 `"an event must be selected before recording attendance"`) guards `handleToggle`, `handleWalkin`, and `handlePersonAttendanceDaySave`.
4. **Uniqueness:** Every write path and the day-detail read path key on the full `(event, intake, date)` tuple.
5. **Matrix UX:** Full roster still shows, but toggling is disabled when no event is selected (banner + disabled cells + hidden walk-in panel + dropdown label change).
6. **Per-person calendar:** Gains an event selector; calendar and day-detail are scoped to the selected event; day-save requires `event_id`.

## Constraints

- **Language/Framework:** Go 1.25 module `r3-intake`; PocketBase v0.39.9 embedded; server-rendered Go templates in a single embedded `index.html`; HTMX + Alpine.js; vanilla CSS.
- **PocketBase v0.39 Go API ONLY** — no `app.dao()`. Use:
  - `s.pb.FindCollectionByNameOrId(name)`
  - `s.pb.FindRecordsByFilter(col.Id, filter, sort, limit, offset)`
  - `s.pb.FindRecordById(col.Id, id)`
  - `s.pb.Save(rec)`, `s.pb.Delete(rec)`
  - `core.NewRecord(col)`, `rec.Set("field", val)`, `rec.GetString("field")`, `rec.Id`
- **Filter escaping:** every user-supplied value interpolated into a PB filter string MUST go through `mcpmod.EscapeFilter(s)` from `r3-intake/internal/mcp` (import as `mcpmod "r3-intake/internal/mcp"`).
- **Time:** `var hst = time.FixedZone("HST", -10*60*60)`; dates `time.Now().In(hst).Format("2006-01-02")`; timestamps `"2006-01-02 15:04:05"`.
- **No native unique constraints in PB** — enforce idempotency in Go by querying for an existing full-key record before creating.
- **Migration registration:** Go migrations live in `r3-intake/pocketbase/migrations/` and are registered in `r3-intake/pocketbase/migrations/002_encryption.go` via `migrations.Register(up, down, "NNN_name.go")`. The mirrored copy at `r3-intake/internal/migrations/pocketbase/migrations/` is OLDER (up to 008) and is NOT canonical — **do not edit it**. The `internal/migrations` package only embeds JS files for the test harness; Go migrations are registered directly via `pbmigrations.Register(pb)`.
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, 14px card radii, 8px input radii. Reuse `.btn`, `.btn-ghost`, `.btn-tiny`, `.btn-danger`, `.status-badge`, `.form-grid-4`, `.form-error`, `.walkin-result*`, `.matrix-dot`, `.dot-disabled`, `.field-input`, `.field-label`, `.field-group`, `.empty-state`, `.muted`.
- **Verification gates:** `go build ./...`, `go vet ./...`, `go test ./...` must all pass.

## File Structure

**Create:**
- `r3-intake/pocketbase/migrations/010_attendance_event_required.go` — the backfill + required-flip migration.

**Modify:**
- `r3-intake/pocketbase/migrations/002_encryption.go` — register the new migration.
- `r3-intake/internal/server/attendance.go` — `requireEventID` helper; `handleToggle`, `handleWalkin`, `loadMatrixRows`, `handleMatrix` (view flag); `handleExportCSV`/`loadExportRows` (event required for export).
- `r3-intake/internal/server/person_attendance.go` — `handlePersonAttendanceDaySave`, `handlePersonAttendanceDayDelete`, `buildPersonDayDetailView`, `buildPersonAttendanceView`, `handlePersonAttendance`, view structs (`PersonAttendanceView`, `PersonDayDetailView`).
- `r3-intake/internal/assets/public/index.html` — `matrix-content`, `matrix-cell`, `walkin-panel`, `person-attendance-calendar`, `person-attendance-day` templates.
- `r3-intake/internal/server/attendance_test.go` — matrix render test updates.
- `r3-intake/internal/server/person_attendance_test.go` — person-attendance render test updates.
- `r3-intake/internal/server/attendance_toggle_integration_test.go` — toggle/walkin event-required tests.
- `r3-intake/internal/server/person_attendance_integration_test.go` — day-save/delete event-scoped tests.
- `r3-intake/internal/server/attendance_export_integration_test.go` — export event-required test.

**Do NOT touch:** `r3-intake/internal/migrations/pocketbase/migrations/` (stale mirror).

## Implementation Notes

### 1. Migration `010_attendance_event_required.go`

Follow the exact pattern of `009_attendance_site_optional.go` (same package `migrations`, `func upAttendanceEventRequired(app core.App) error` / `func downAttendanceEventRequired(app core.App) error`).

**`upAttendanceEventRequired(app core.App) error`:**
1. `attCol, err := app.FindCollectionByNameOrId("attendance")`; return err on failure.
2. **Idempotency guard:** `f := attCol.Fields.GetByName("event")`; if `rf, ok := f.(*core.RelationField); ok && rf.Required` → already required, return `nil` (no-op).
3. **Query null-event records:** `recs, err := app.FindRecordsByFilter(attCol.Id, "event=''", "date", 100000, 0)`; return err on failure. If `len(recs) == 0`, skip straight to the required-flip (step 6).
4. **Group by site:** build `bySite := map[string][]*core.Record{}` keyed on `rec.GetString("site")` (may be `""` for no-location records — group them under the empty-site key).
5. **For each site group** (including the empty-site group):
   - Load `eventsCol, _ := app.FindCollectionByNameOrId("events")` and `sitesCol, _ := app.FindCollectionByNameOrId("sites")` once (outside the loop).
   - Compute `minDate`/`maxDate` across the group's records from `rec.GetString("date")` (string compare works for `YYYY-MM-DD`). If a group has no parseable dates, fall back to `""` for both.
   - Create one synthetic legacy event: `legacy := core.NewRecord(eventsCol)`; set:
     - `site` = the group's site id (or `""` if the group key is empty — `events.site` is required, so if the site is empty, resolve the site from the first record's `intake` via `intakeCol` lookup, or skip backfill for that group and leave those records to fail the required flip — see Edge Cases).
     - `name` = `"Legacy / Unassigned"`
     - `start_date` = minDate, `end_date` = maxDate
     - `status` = `"completed"`
     - `description` = `"Auto-created by migration 010 to preserve pre-event-scoped attendance records."`
     - `created_by` = `""` (leave unset)
   - `app.Save(legacy)`; return err on failure.
   - For each record in the group: `rec.Set("event", legacy.Id)`; `app.Save(rec)`; return err on failure.
6. **Flip required:** re-read `f := attCol.Fields.GetByName("event")`; if `rf, ok := f.(*core.RelationField); ok && !rf.Required { rf.Required = true; app.Save(attCol) }`. Return err or nil.

**`downAttendanceEventRequired(app core.App) error` (best-effort):**
1. `attCol, err := app.FindCollectionByNameOrId("attendance")`; return err on failure.
2. `f := attCol.Fields.GetByName("event")`; if `rf, ok := f.(*core.RelationField); ok && rf.Required { rf.Required = false; app.Save(attCol) }`.
3. **Do NOT delete the legacy events** — `attendance.event` has `cascadeDelete: true`, so deleting a legacy event would cascade-delete the attendance records it now owns. Document this in a comment: down is best-effort and intentionally leaves legacy events in place.

**Register in `002_encryption.go`:** add `migrations.Register(upAttendanceEventRequired, downAttendanceEventRequired, "010_attendance_event_required.go")` after the `009` registration line.

**Edge cases:**
- **Empty-site group:** `events.site` is required. If a null-event attendance record has an empty `site`, resolve the site from its `intake` record (`intakeCol.FindRecordById` → `GetString("site")`). If still empty, the record cannot be assigned to a valid legacy event; leave it unassigned and let the required-flip fail loudly (return the save error) rather than silently dropping data. In practice the 009 migration made `attendance.site` optional, so this is a real possibility — handle it explicitly.
- **Idempotency:** the `rf.Required` guard makes re-running a no-op after the first successful run.
- **Large backfill:** `limit=100000` covers realistic data; the per-record save loop is acceptable for a one-time migration.

### 2. Shared `requireEventID` gate (attendance.go)

Add a small helper near the top of `attendance.go`:

```go
// requireEventID returns a 400 if eventID is empty. Attendance is now
// event-scoped; every write path must select an event first.
func requireEventID(w http.ResponseWriter, eventID string) bool {
    if strings.TrimSpace(eventID) == "" {
        http.Error(w, "an event must be selected before recording attendance", http.StatusBadRequest)
        return false
    }
    return true
}
```

Call it at the top of each write handler, right after parsing `eventID` from the form/query, before any DB work:
- `handleToggle` (attendance.go:460) — after reading `eventID := strings.TrimSpace(r.FormValue("event_id"))`.
- `handleWalkin` (attendance.go:675) — after reading `eventID := strings.TrimSpace(r.FormValue("event_id"))`.
- `handlePersonAttendanceDaySave` (person_attendance.go:250) — after reading `eventID := strings.TrimSpace(r.FormValue("event_id"))`.

### 3. `handleToggle` (attendance.go:460) — full-key uniqueness + always set event

- Add `requireEventID(w, eventID)` gate (see above).
- **Find existing:** change the filter to always include the event key:
  ```go
  filter := fmt.Sprintf("intake='%s' && event='%s' && date='%s'",
      mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(date))
  ```
  (Remove the `if eventID != ""` conditional — event is now always present.)
- **Create path:** always `rec.Set("event", eventID)` (remove the `if eventID != ""` guard).
- **Update path:** also set `existing.Set("event", eventID)` for consistency (harmless, keeps the record aligned).
- The `MatrixCell` returned still carries `EventID: eventID` (now always non-empty).

### 4. `handleWalkin` (attendance.go:675) — full-key uniqueness + always set event

- Add `requireEventID(w, eventID)` gate. Read `eventID := strings.TrimSpace(r.FormValue("event_id"))` before the gate.
- **Find existing:** change filter to full key:
  ```go
  filter := fmt.Sprintf("intake='%s' && event='%s' && date='%s'",
      mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(today))
  ```
- **Create path:** always `rec.Set("event", eventID)`.
- **Update path:** also `existing.Set("event", eventID)`.
- The 303 redirect already appends `event=` when non-empty; since event is now required, it will always be appended.

### 5. `loadMatrixRows` (attendance.go:233) + `handleMatrix` (attendance.go:77) — EventRequired view flag

- Add `EventRequired bool` to `MatrixViewData` (attendance.go:17). Set it in `handleMatrix`:
  ```go
  EventRequired: eventID == "",
  ```
- In `loadMatrixRows`, when `eventID == ""` (no event selected), the roster should still show the FULL site/role-scoped roster (current `else` branch), but every cell must be disabled. Set `Disabled: cellSiteID == "" || eventID == ""` in the `MatrixCell` construction (attendance.go:346). This keeps the full roster visible while preventing toggling.
- The `attMap` lookup stays as-is (no event filter when `eventID == ""`), so cells still display existing statuses; only toggling is disabled.

### 6. `handleExportCSV` (attendance.go:787) / `loadExportRows` (attendance.go:843) — event required

- In `handleExportCSV`, after `eventID := strings.TrimSpace(r.URL.Query().Get("event"))`, add `requireEventID(w, eventID)` (export is admin-only; a 400 with the standard message is acceptable). This prevents exporting a mix of event-scoped and legacy records without an event context.
- `loadExportRows` already filters `&& event='%s'` when `eventID != ""`; since event is now required, the `if eventID != ""` guard can stay (it will always be true after the gate) — no functional change needed beyond the gate.

### 7. Per-person calendar — event selector + event-scoped queries (person_attendance.go)

**View struct changes:**
- `PersonAttendanceView` (person_attendance.go:20): add `EventID string` and `Events []Event` (reuse the `Event` type from attendance.go, same package). Add `EventRequired bool` (true when `EventID == ""`).
- `PersonDayDetailView` (person_attendance.go:66): add `EventID string` and `Events []Event`.

**`handlePersonAttendance` (person_attendance.go:112):** read `eventID := strings.TrimSpace(r.URL.Query().Get("event"))`; pass it into `buildPersonAttendanceView`.

**`buildPersonAttendanceView` (person_attendance.go:131):** add an `eventID string` parameter. Load events via `s.loadEvents("")` (all active events) into the view. When `eventID != ""`, scope the attendance query:
```go
filter := fmt.Sprintf("intake='%s' && event='%s' && date>='%s' && date<='%s'",
    mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(eventID),
    mcpmod.EscapeFilter(firstStr), mcpmod.EscapeFilter(lastStr))
```
When `eventID == ""`, keep the current unscoped query (calendar still shows all records) but set `EventRequired: true` so the template can prompt for an event. Set `EventID: eventID` on the view.

**`renderPersonAttendanceCalendar` (person_attendance.go:187):** pass the event through — it calls `buildPersonAttendanceView(u, intake, month)`; add the event param.

**`handlePersonAttendanceDay` (person_attendance.go:194):** read `eventID` from the query/form and thread it into `handlePersonAttendanceDayGet` and `handlePersonAttendanceDaySave`.

**`handlePersonAttendanceDayGet` (person_attendance.go:211):** read `eventID := strings.TrimSpace(r.URL.Query().Get("event"))`; pass to `buildPersonDayDetailView`.

**`buildPersonDayDetailView` (person_attendance.go:223):** add `eventID string` param. Load events into the view. Scope the existing-record lookup to the full key:
```go
filter := fmt.Sprintf("intake='%s' && event='%s' && date='%s'",
    mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(date))
```
Set `view.EventID = eventID` and `view.Events = events`.

**`handlePersonAttendanceDaySave` (person_attendance.go:250):**
- Add `requireEventID(w, eventID)` gate (read `eventID := strings.TrimSpace(r.FormValue("event_id"))` first).
- Scope the existing-record lookup to the full key (same filter as above).
- **Create path:** always `rec.Set("event", eventID)`.
- **Update path:** also `existing.Set("event", eventID)`.

**`handlePersonAttendanceDayDelete` (person_attendance.go:318):**
- Read `eventID := strings.TrimSpace(r.FormValue("event_id"))`.
- Scope the delete lookup to the full key:
  ```go
  filter := fmt.Sprintf("intake='%s' && event='%s' && date='%s'",
      mcpmod.EscapeFilter(intake.Id), mcpmod.EscapeFilter(eventID), mcpmod.EscapeFilter(date))
  ```
- (No `requireEventID` gate needed for delete — deleting a non-existent key is a no-op success, and the calendar only offers delete from an event-scoped day view. But scoping the filter is required so it only deletes the record for the selected event.)

### 8. Templates (index.html)

**`matrix-content` (index.html:870):**
- Change the dropdown's empty option label from `All dates — no event filter` to `Select an event…` (keep `value=""`).
- Add an `EventRequired` banner when `.EventRequired` is true, e.g. `<p class="empty-state">Select an event to record attendance.</p>` (or a `.form-error`-style banner) near the top of the matrix.
- Gate the `walkin-panel` on `{{if not .EventRequired}}` so the "Add walk-in" panel is hidden when no event is selected.

**`matrix-cell` (index.html:967):**
- The `event_id` hidden input is currently `{{if .EventID}}...{{end}}`. Since event is now always set when a cell is toggleable, keep the conditional (it renders when `EventID != ""`). The `Disabled` flag already renders the disabled dot; with `EventRequired`, `Disabled` is true for all cells, so no template change is strictly needed here — but ensure the disabled dot's `aria-label` reads appropriately (e.g. `Attendance requires a location` → keep, or generalize to `Attendance requires an event and location`). Keep it simple: leave the existing disabled-dot markup; the banner communicates the event requirement.

**`person-attendance-calendar` (index.html:1199):**
- Add an event selector `<select name="event">` in the calendar nav area, with `value=""` labeled `Select an event…` and options from `.Events` (selected when `eq $.EventID .ID`). Wire it to re-render the calendar via HTMX (`hx-get="/intake/{{.IntakeID}}/attendance?month={{.Month}}&event=..."` `hx-target="#person-attendance-calendar"` `hx-swap="outerHTML"`), or a plain GET form.
- When `.EventRequired` is true, show a prompt (e.g. `Select an event to record attendance.`) and disable the day-cell click-to-open behavior (or leave cells clickable but the day-save will 400 — better to gate the `hx-get` on the day cells with `{{if not $.EventRequired}}`).

**`person-attendance-day` (index.html:1242):**
- Add a hidden `event_id` input to both the save form and the delete form: `<input type="hidden" name="event_id" value="{{.EventID}}">`.
- Add an event selector `<select name="event">` in the day-detail form (options from `.Events`, selected on `.EventID`) so the user can change the event when saving. The save handler reads `event_id` from the form.

**CSS (app.css):** add a `.matrix-event-banner` (or reuse `.empty-state`) style and bump the `?v=` cache-buster on the stylesheet link in `index.html` (both the `matrix` page link ~line 819 and the `person-attendance` page link ~line 1170) whenever CSS changes.

### 9. Tests

**`attendance_test.go` — `TestMatrixContentRender` (line 65):** update the fixture to include `EventRequired: true` (or `false`) and assert the banner text and the `Select an event…` label render. Add a case with `EventRequired: true` asserting cells are disabled and the walk-in panel is hidden.

**`person_attendance_test.go` — `TestPersonAttendanceTemplateRenders` (line 209):** update the `PersonAttendanceView` fixture with `EventID`/`Events`/`EventRequired` and assert the event selector renders.

**`attendance_toggle_integration_test.go`:** add `TestToggleRequiresEvent` — POST `/attendance/toggle` with `event_id=""` (and valid intake/date/site), assert 400 and the message `an event must be selected before recording attendance`, and assert no attendance record was created. Add `TestToggleEventScoped` — seed an event, POST with `event_id`, assert the record is created with `event` set and that a second POST for the same `(event, intake, date)` updates rather than duplicates.

**`person_attendance_integration_test.go`:** add `TestPersonAttendanceDaySaveRequiresEvent` — POST day-save with `event_id=""`, assert 400. Update `TestPersonAttendanceDaySaveCreate`/`Update`/`Delete` to pass an `event_id` and assert the record is created/updated/deleted with the event key.

**`attendance_export_integration_test.go`:** add `TestExportCSVRequiresEvent` — GET `/attendance/export` with no `event`, assert 400.

**Migration test (optional but recommended):** add a test in `attendance_export_integration_test.go` (or a new `migration_test.go`) that seeds a null-event attendance record, runs `upAttendanceEventRequired`, and asserts the record now has a non-empty `event` pointing at a `"Legacy / Unassigned"` event, and that `attendance.event` is required. Use the `newTestServer` harness (which calls `pbmigrations.Register(pb)` + `pb.RunAllMigrations()`), so the migration runs automatically on bootstrap — seed the null-event record AFTER bootstrap but BEFORE the migration would run is not possible with the current harness; instead, test the migration function directly by calling `upAttendanceEventRequired(pb)` on a server where the field is still optional (seed a null-event record, call the up func, assert). Note the harness runs all migrations on bootstrap, so the field will already be required — to test the backfill, construct a fresh PB without running `010` (or call `downAttendanceEventRequired` first to make the field optional, seed, then call `upAttendanceEventRequired`).

## Verification Criteria

1. **Build/vet/test gates pass:**
   ```bash
   cd r3-intake && go build ./... && go vet ./... && go test ./...
   ```
   All three must exit 0.

2. **Migration correctness:**
   - `010_attendance_event_required.go` is registered in `002_encryption.go` and runs on bootstrap (the `newTestServer` harness exercises it via `pbmigrations.Register(pb)` + `pb.RunAllMigrations()`).
   - Null-event attendance records are backfilled to a per-site `"Legacy / Unassigned"` event with `status="completed"`, correct `start_date`/`end_date` (min/max across the site's null-event records), and the specified description.
   - `attendance.event` is required after the migration (assert `rf.Required == true`).
   - Re-running the migration is a no-op (idempotency guard).

3. **Write-path enforcement:**
   - `handleToggle`, `handleWalkin`, `handlePersonAttendanceDaySave` all return 400 `"an event must be selected before recording attendance"` when `event_id` is empty, and create no record.
   - With an event selected, each write path creates/updates a record keyed on `(event, intake, date)` with `event` always set.

4. **Uniqueness:** toggling the same `(event, intake, date)` twice updates the existing record (no duplicate); a different event for the same `(intake, date)` creates a separate record.

5. **Matrix UX:** with no event selected, the full roster renders, all cells are disabled, the `Select an event to record attendance.` banner shows, the walk-in panel is hidden, and the dropdown label reads `Select an event…`. With an event selected, toggling works and cells carry the `event_id`.

6. **Per-person calendar:** the event selector renders; selecting an event scopes the calendar and day-detail to that event; day-save requires `event_id`; day-delete only removes the record for the selected event.

7. **Export:** `/attendance/export` without an event returns 400; with an event it filters correctly.

8. **No regressions:** existing matrix, toggle, walk-in, person-attendance, and export tests still pass (updated fixtures compile and assert correctly).
