# Working Plan: Add `deleted` field to `events` collection via migration

## Objective
Create a new PocketBase Go migration `014_events_deleted.go` that adds a soft-delete `deleted` bool field to the `events` collection, and register it in the migrations registry (`migrations.go`). This is a child story card of the Event CRUD epic (update + soft-delete); the sibling handler card will consume the `deleted` field. **No handlers or UI are implemented here** — only the migration + registry registration.

## Constraints
- **Follow the exact established pattern** in `008_event_enrollment_deleted.go` (up/down funcs, idempotency guards, `core.BoolField`).
- Target collection is **`events`** (created in `007_events_attendance.js`; currently has NO `deleted` field).
- Migration must be **idempotent**: add only if `col.Fields.GetByName("deleted") == nil`; remove only if `!= nil`.
- Field is `core.BoolField{Name: "deleted", Required: false}`.
- File name must be **`014_events_deleted.go`** (next sequential number after `013_attendance_unique_indexes.go`).
- Up/down funcs named **`upEventsDeleted`** / **`downEventsDeleted`**.
- Registry line: `migrations.Register(upEventsDeleted, downEventsDeleted, "014_events_deleted.go")`.
- Only the single new file + one registry line. Do NOT touch handlers, templates, or the `events` collection's other fields.
- No mirrored migrations dir exists in this worktree (integration test reads JS migrations from `pocketbase/migrations/` and registers Go migrations via `pbmigrations.Register(pb)`), so no duplicate file is needed.

## File Structure
- `r3-intake/pocketbase/migrations/014_events_deleted.go` — **new** migration file (up/down funcs).
- `r3-intake/pocketbase/migrations/migrations.go` — **modified**: add one `migrations.Register(...)` line for `014_events_deleted.go`.

## Implementation Notes
1. **Create `014_events_deleted.go`** in `r3-intake/pocketbase/migrations/` with `package migrations` and import `"github.com/pocketbase/pocketbase/core"`.
2. **`upEventsDeleted(app core.App) error`**:
   - `col, err := app.FindCollectionByNameOrId("events")`; return `err` on failure.
   - If `col.Fields.GetByName("deleted") == nil`: `col.Fields.Add(&core.BoolField{Name: "deleted", Required: false})`, then `app.Save(col)` (return `err` on failure).
   - Return `nil`.
3. **`downEventsDeleted(app core.App) error`**:
   - `col, err := app.FindCollectionByNameOrId("events")`; return `err` on failure.
   - If `col.Fields.GetByName("deleted") != nil`: `col.Fields.RemoveByName("deleted")`, then `app.Save(col)` (return `err` on failure).
   - Return `nil`.
4. **Register in `migrations.go`**: add `migrations.Register(upEventsDeleted, downEventsDeleted, "014_events_deleted.go")` after the `013_attendance_unique_indexes.go` line (filename passed last per the v0.39 signature, matching the existing comment).
5. **Do not** add any handler, query filter, or UI change — the sibling card owns consumption of the field.

## Verification Criteria
- `cd r3-intake && go build ./...` passes (migration compiles).
- `cd r3-intake && go vet ./...` passes.
- `cd r3-intake && go test ./...` passes (existing tests, including the in-process PB integration harness, still green).
- `grep -n "014_events_deleted" r3-intake/pocketbase/migrations/migrations.go` shows the registration line.
- `grep -n "deleted" r3-intake/pocketbase/migrations/014_events_deleted.go` confirms both up/down idempotency guards are present.
- Migration is idempotent: re-running up/down is a no-op (guards on `GetByName("deleted")`).
