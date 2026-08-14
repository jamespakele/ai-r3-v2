# Working Plan: Fix loadEnrolledRoster to return enrolled participants and stats

## Objective

Make `loadEnrolledRoster` (in `r3-intake/internal/server/admin.go`) return the actual enrolled participants for an event, with correct per-participant attendance stats, so the Event Manage page (`/admin/events/{id}/manage`) and the HTMX roster fragment (`respondRoster`) show enrolled participants instead of the empty state "No participants enrolled yet."

## Constraints

- **Language/runtime:** Go, server-rendered Go templates + HTMX. PocketBase **v0.39.9** (embedded, `github.com/pocketbase/pocketbase v0.39.9`).
- **PocketBase API:** Use `app.findCollectionByNameOrId` / `s.pb.FindCollectionByNameOrId` and `s.pb.FindRecordsByFilter`. **Do NOT use `app.dao()`** (v0.39 removed the DAO layer).
- **Timezone:** All dates are `YYYY-MM-DD` strings in the `hst` timezone (`time.Now().In(hst)`).
- **Scope:** Fix **only** `loadEnrolledRoster`. Do **not** touch the enroll-search handler (`handleEnrollSearch`, sibling card `t_b478182c`). Do **not** write tests (sibling card `t_d6f46781`).
- **Filter escaping:** Use `mcpmod.EscapeFilter` (escapes `\` and `"`) for any user/record-derived values interpolated into PocketBase filters.

## File Structure

- **Modify:** `r3-intake/internal/server/admin.go` — the `loadEnrolledRoster` function (filter at line ~531). No new files.

## Implementation Notes

### The actual bug

`loadEnrolledRoster` builds its query with:

```go
filter := fmt.Sprintf("event='%s' && deleted=false", mcpmod.EscapeFilter(eventID))
```

In PocketBase v0.39, a `BoolField` declared with `Required: false` (as migration `008_event_enrollment_deleted.go` does for the `deleted` field on `event_enrollment`) **defaults to `NULL`, not `false`**, for any record where the field was never explicitly written. The filter `deleted=false` matches only rows whose stored value is literally `false`; rows whose `deleted` value is `NULL` are **excluded**. Result: the query returns zero (or too few) records, so the roster renders empty even when enrollments exist.

This is exactly the situation for the parent bug (`t_3c03b154`): enrollment records created **before** migration 008 ran (or created through any path that did not explicitly `Set("deleted", false)`) carry `deleted = NULL`. Only records created by the current `handleEventEnroll` (which explicitly does `rec.Set("deleted", false)` at line 671) survive the filter — so the roster appears empty for pre-existing enrollments.

### The fix

Change the filter in `loadEnrolledRoster` to treat `NULL` as "not deleted", matching the codebase's own established pattern in `notes.go` (`loadNoteRows`, line 363):

```go
filter := fmt.Sprintf("event='%s' && (deleted = false || deleted = null)", mcpmod.EscapeFilter(eventID))
```

This is the minimal, correct change. It:
- Returns enrollments whose `deleted` is `false` (explicitly active).
- Returns enrollments whose `deleted` is `NULL` (legacy / never-set records — treated as active).
- Still excludes soft-deleted enrollments (`deleted = true`), preserving the unenroll behavior (attendance history kept, row hidden).

### Related same-bug occurrences (note, do not change unless in scope)

The identical `deleted=false` filter appears in:
- `loadEnrolledCount` (line ~899) — drives the `EventEnrolled` count on the events list and the manage page header. It has the same NULL-exclusion bug and will under-count legacy enrollments.
- `handleEventEnroll` idempotency check (line ~661) and `handleEventUnenroll` (line ~698) — these filter on `(event, intake)` pairs; a legacy NULL record would not be matched by the enroll idempotency check, allowing a duplicate enrollment row to be created.

This card is scoped to `loadEnrolledRoster` only, so the primary change is the single filter above. Flag the `loadEnrolledCount` occurrence to the parent as a follow-up (it is the same root cause and affects the displayed count), but do not expand scope here.

### No other changes needed

- **Stats computation** (`loadEnrollmentStats`, `enrollmentRate`, `daysInRange`): verified correct — `totalDays` is capped to today for in-progress events, `rate` guards division by zero, `lastPresent` tracks the max present/walk_in date. No bug.
- **Intake lookup / sort:** `FindRecordById("intake", iid)` with `continue` on error (cascade-deleted intake skip) and `sort.SliceStable` by name are correct.
- **Template:** `event-roster` renders `{{range .Enrolled}}` and the empty state only when `.Enrolled` is empty — no template change needed.

## Verification Criteria

1. `cd r3-intake && go build ./...` — compiles clean (baseline already passes).
2. `go vet ./...` — no new vet warnings.
3. `go test ./...` — existing tests still pass (no new tests written per scope).
4. Manual sanity (optional, if a running instance is available): with a legacy enrollment record whose `deleted` field is `NULL`, the manage page roster and the HTMX `respondRoster` fragment now list the participant with correct Days Attended / Rate / Last Present; unenroll still removes the row while keeping attendance history.
