# Working Plan: Schema migration — remove attendance.site

## Objective

Create a new Go migration `015_attendance_remove_site.go` that removes the now-redundant `attendance.site` relation field from the R3 Intake attendance collection. Because `attendance.event` is required (migration 010) and `events.site` is required, the event's site is the single authoritative source of location for every attendance row — the stored `attendance.site` value is redundant and must be dropped. The migration must be idempotent, perform a non-blocking data-integrity backfill check, and provide a lossless down path that re-adds the field and backfills it from each row's event's site.

**Scope boundary:** This is a schema-only card. The Go filtering logic that currently *reads* `attendance.site` is a separate sibling card and is explicitly **out of scope** here. This migration only alters the schema; it does not touch any query/filter code.

## Constraints

- **Language:** Go, matching the existing migration files in `r3-intake/pocketbase/migrations/`.
- **PocketBase version:** v0.39. No `app.dao()`. Use the `core.App` surface: `app.FindCollectionByNameOrId`, `app.Save`, `app.FindRecordsByFilter(colId, filter, sort, limit, offset)`, `app.FindRecordById(colId, id)`. Fields via `col.Fields.GetByName("name")` and `col.Fields.RemoveByName("name")`. Records via `rec.GetString("field")` and `rec.Set("field", value)`.
- **Migration numbering:** New file is `015_attendance_remove_site.go`. Numbering guarantees it runs after `010_attendance_event_required.go` (event required) and `013_attendance_unique_indexes.go` (unique indexes). It must be registered in `migrations.go` with the filename passed **last** per the v0.39 `migrations.Register(upFn, downFn, "NNN_name.go")` signature.
- **No code changes** to any non-migration Go files (filters, handlers, etc.) — those belong to the sibling card.

## File Structure

| File | Action | Notes |
|------|--------|-------|
| `r3-intake/pocketbase/migrations/015_attendance_remove_site.go` | **Create** | New migration with `upAttendanceRemoveSite` / `downAttendanceRemoveSite` functions. |
| `r3-intake/pocketbase/migrations/migrations.go` | **Modify** | Add `migrations.Register(upAttendanceRemoveSite, downAttendanceRemoveSite, "015_attendance_remove_site.go")` to the registration list. |

No other files are touched. The unique index `idx_attendance_event_intake_date` on `(event, intake, date)` does **not** reference `site`, so it is unchanged and requires no index work.

## Implementation Notes

### Up migration — `upAttendanceRemoveSite(app core.App) error`

1. **Load collection.**
   ```go
   attCol, err := app.FindCollectionByNameOrId("attendance")
   if err != nil {
       return err
   }
   ```

2. **Idempotency guard.** If the `site` field is already absent, no-op.
   ```go
   if attCol.Fields.GetByName("site") == nil {
       return nil
   }
   ```

3. **Backfill safety check (non-blocking).** Before removing the field, verify every attendance row's `site` matches its event's `site`. Because `attendance.event` is required (010) and `events.site` is required, the event-derived location is always defined. If any row's `attendance.site` differs from its event's `site`, log a warning (do **not** fail) — the event's site is authoritative and the divergent stored value is discarded. This is a data-integrity note, not a blocker.
   - Load all attendance records: `app.FindRecordsByFilter(attCol.Id, "", "", 100000, 0)`.
   - Load the `events` collection once: `eventsCol, err := app.FindCollectionByNameOrId("events")`.
   - For each attendance record, resolve its event: `ev, err := app.FindRecordById(eventsCol.Id, rec.GetString("event"))`. If the event lookup fails, log a warning and continue (best-effort; the field is being removed anyway).
   - Compare `rec.GetString("site")` against `ev.GetString("site")`. If they differ (or the stored site is non-empty while the event's is empty), log a warning via `app.Logger().Warn(...)` describing the record id, stored site, and event site. Do not abort.
   - Note: `app.Logger()` is the v0.39 logging surface; use it rather than `fmt.Println` to match PocketBase conventions.

4. **Remove the field.**
   ```go
   attCol.Fields.RemoveByName("site")
   if err := app.Save(attCol); err != nil {
       return err
   }
   ```

5. Return `nil`.

### Down migration — `downAttendanceRemoveSite(app core.App) error`

1. **Load collection.**
   ```go
   attCol, err := app.FindCollectionByNameOrId("attendance")
   if err != nil {
       return err
   }
   ```

2. **Idempotency guard.** If the `site` field is already present, no-op.
   ```go
   if attCol.Fields.GetByName("site") != nil {
       return nil
   }
   ```

3. **Re-add the field as an optional relation to `sites`** (matching 009's state — optional, not required):
   ```go
   attCol.Fields.Add(&core.RelationField{
       Name:         "site",
       CollectionId: sitesCol.Id, // resolve sites collection first
       Required:     false,
       MaxSelect:    1,
   })
   if err := app.Save(attCol); err != nil {
       return err
   }
   ```
   - Resolve the sites collection first: `sitesCol, err := app.FindCollectionByNameOrId("sites")`. Match the field shape used in 009 (optional relation, single-select). Verify the exact `RelationField` fields against 009's original definition before writing.

4. **Backfill from each row's event's site (lossless).** Best-effort: if an event lookup fails, leave the field empty (it is optional).
   - Load all attendance records: `app.FindRecordsByFilter(attCol.Id, "", "", 100000, 0)`.
   - Load the `events` collection once.
   - For each record: `ev, err := app.FindRecordById(eventsCol.Id, rec.GetString("event"))`. If the event resolves, `rec.Set("site", ev.GetString("site"))` and `app.Save(rec)`. If the event lookup fails or the event's site is empty, leave `site` unset (optional field) and continue.

5. Return `nil`.

### Edge cases

- **Empty attendance table:** both up and down paths handle zero records gracefully (loops no-op).
- **Event lookup failure during backfill:** logged and skipped; field left empty (optional) — never a hard failure.
- **Divergent stored site vs. event site:** warning only, never a blocker, per the design doc.
- **Down migration ordering:** re-adding the field and backfilling must happen in the correct order — add the field to the schema first, then backfill records, so `rec.Set("site", ...)` has a valid field to write to.
- **Index integrity:** `idx_attendance_event_intake_date` does not reference `site`, so removing the field does not invalidate it. No index add/remove is required in either direction.

## Verification Criteria

1. **Compiles:** `go build ./...` (or `go vet ./...`) succeeds from `r3-intake/` with the new migration file and the updated `migrations.go`.
2. **Registration correct:** `migrations.go` contains `migrations.Register(upAttendanceRemoveSite, downAttendanceRemoveSite, "015_attendance_remove_site.go")` with the filename last.
3. **Idempotency (up):** Running the migration twice is a no-op on the second run — the `site` field is absent, so the guard returns early with no error.
4. **Field removed:** After the up migration, `attendance` collection has no `site` field (`col.Fields.GetByName("site")` returns `nil`), and `idx_attendance_event_intake_date` on `(event, intake, date)` still exists unchanged.
5. **Backfill check behavior:** With a seeded row whose `attendance.site` differs from its event's `site`, the migration completes successfully and emits a warning log (does not fail).
6. **Down migration lossless:** After the down migration, `attendance.site` is re-added as an optional relation to `sites`, and every attendance row's `site` equals its event's `site` (backfilled). Rows whose event lookup failed have an empty `site`.
7. **Down idempotency:** Running the down migration twice is a no-op on the second run.
8. **Round-trip:** `up → down → up` leaves the schema in the final (site-removed) state with no errors.
9. **No out-of-scope changes:** `git status` shows only the new migration file and the `migrations.go` edit — no filter/handler/query code modified.
