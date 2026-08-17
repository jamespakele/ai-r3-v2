# Working Plan: Fix event filter logic in handleExportCSV so TestExportCSVEventFilter passes

## Objective

Fix `handleExportCSV` in `r3-intake/internal/server/attendance.go` so that when an event filter is supplied but no explicit valid `from`/`to` dates are given, the export date range auto-scopes to the selected event's `start_date` -> `end_date` (full span, no 30-day cap). This makes `TestExportCSVEventFilter` (specifically the `ev1_only` subtest) pass.

## Problem Statement

`TestExportCSVEventFilter` (in `r3-intake/internal/server/attendance_export_integration_test.go`) has two subtests. The **`ev1_only`** subtest fails:

- It calls `doExport(srv, admin, "?event="+fx.ev1)` with **no explicit `from`/`to`** query params.
- `handleExportCSV` falls back to the default 14-day window `[today-13, today]` = `[2026-08-04, 2026-08-17]` (when run on 2026-08-17).
- The seeded ev1 attendance records are dated **2026-08-01** and **2026-08-02** — both **outside** the default window.
- `loadExportRows` builds `date>='from' && date<='to'` from those defaults, so **all ev1 rows are excluded by the date filter before the event filter is even considered** -> the subtest sees zero rows and fails.
- `ev2_only` happens to pass only because ev2's records (08-02, 08-05, 08-10) fall inside the default window on 2026-08-17.

## Root Cause

`handleExportCSV` does not auto-scope the date range to the selected event when the caller supplies an event filter but no explicit valid `from`/`to`. The matrix path already does this via `parseMatrixFilters`; the CSV export path does not.

## Chosen Fix (fix_option: auto_scope_to_event_dates)

When `handleExportCSV` receives an event filter but **no explicit valid `from`/`to` dates**, auto-scope the date range to the selected event's `start_date` -> `end_date` (full span, **no 30-day cap**), mirroring the existing `parseMatrixFilters` auto-scoping behavior. This makes the export date range match the event's actual dates.

## Constraints

- **Language:** Go (server-rendered templates, HTMX + Alpine.js).
- **Framework:** PocketBase v0.39 embedded, JS-style camelCase API; **no `app.dao()`**. Queries use `s.pb.FindRecordsByFilter(col.Id, filter, sort, limit, offset)`. Filter strings built in Go and escaped with `mcpmod.EscapeFilter`.
- **Timezone:** All timestamps HST (UTC-10, no DST) via the `hst` fixed zone.
- **Design system:** Public Sans + Lora, accent `#b5502e`. No visual changes required.
- **Data model:** `events` has `start_date` and `end_date` (string dates `2006-01-02`). `attendance` has `event` (relation), `date`, `status`. `loadExportRows` filters on `date` and `event`.

## Scope

**Minimal and focused.** Only `handleExportCSV` in `r3-intake/internal/server/attendance.go` is modified.

- **DO NOT change** `loadExportRows` (already builds the filter correctly from the `from`/`to` it receives).
- **DO NOT change** the test file.
- **DO NOT change** templates or any other handler.

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `r3-intake/internal/server/attendance.go` | **CHANGE** | Add `explicitRange` flag + auto-scope block in `handleExportCSV` (only change). |
| `r3-intake/internal/server/attendance_export_integration_test.go` | KEEP | No change. |
| `r3-intake/internal/server/attendance.go` (`loadExportRows`, `parseMatrixFilters`) | KEEP | No change. |

## Implementation Steps

### Step 1 — Track whether the range was explicitly provided

In `handleExportCSV` (attendance.go, ~line 766), after parsing `from`/`to`:

```go
fromT, errFrom := time.Parse("2006-01-02", from)
toT, errTo := time.Parse("2006-01-02", to)
explicitRange := errFrom == nil && errTo == nil
```

Place this immediately after the two `time.Parse` calls, before the `if errFrom != nil || errTo != nil` fallback block. This mirrors `parseMatrixFilters` (attendance.go:171).

### Step 2 — Auto-scope to the event's dates

After the `requireEventID(w, eventID)` check (attendance.go, ~line 800) and **before** the `s.loadExportRows(eventID, from, to)` call, add:

```go
// Auto-scope to the selected event's dates when the caller did not
// provide an explicit valid from/to range. The event span is
// authoritative: no 30-day cap applies.
if !explicitRange {
    eventsCol, err := s.eventsCollection()
    if err == nil {
        if evRec, err := s.pb.FindRecordById(eventsCol.Id, eventID); err == nil {
            start, errStart := time.Parse("2006-01-02", evRec.GetString("start_date"))
            end, errEnd := time.Parse("2006-01-02", evRec.GetString("end_date"))
            if errStart == nil && errEnd == nil && !start.After(end) {
                from = evRec.GetString("start_date")
                to = evRec.GetString("end_date")
            }
        }
    }
}
```

Notes:
- This block runs **after** the swap and 30-day-cap logic, so it overrides both when auto-scoping applies — the event span is used verbatim with no cap.
- Uses existing patterns: `s.eventsCollection()` (attendance.go:511), `s.pb.FindRecordById` (used in `loadExportRows`), `evRec.GetString("start_date")` / `"end_date"` (same field names as `loadEvents`).
- No `mcpmod.EscapeFilter` needed here — the values are read from the DB record, not user input, and are only used to build the filter string in `loadExportRows` (which escapes them).

### Step 3 — Verify

Run the target test:

```bash
cd r3-intake
go test ./internal/server/ -run TestExportCSVEventFilter -v
```

Then run the full CSV export test set to confirm no regressions:

```bash
go test ./internal/server/ -run 'TestExportCSV' -v
```

## Logical Consequences (downstream site trace)

| Site | Action | Rationale |
|------|--------|-----------|
| `handleExportCSV` (attendance.go) | **CHANGE** | Add `explicitRange` flag + auto-scope block. This is the only change. |
| `loadExportRows` (attendance.go:822) | **KEEP** | Already builds `date>='from' && date<='to'` + event OR-clause correctly from the `from`/`to` it receives. No change needed. |
| `parseMatrixFilters` (attendance.go:164) | **KEEP** | Already auto-scopes to event dates when no explicit range. The CSV fix mirrors this behavior; no change. |
| Export URL construction in templates | **KEEP** | Templates that build export links with explicit `from`/`to` are unaffected (explicit range -> no auto-scope). Templates without dates now get a correct event-scoped range. No change. |
| `TestExportCSVEventFilter` | **KEEP** | `ev1_only` now passes because the range auto-scopes to ev1's `2026-08-01`->`2026-08-31`, covering the 08-01/08-02 records. `ev2_only` still passes. Test unchanged. |
| Other CSV export tests with explicit dates (e.g. `TestExportCSVHeaderAndFormatting` uses `?from=2026-08-01&to=2026-08-10&event=...`) | **KEEP** | Explicit valid range -> `explicitRange` is true -> auto-scope block skipped. Unaffected. |
| `TestExportCSVEmptyResultSet` | **KEEP** | Uses explicit 1990 dates -> `explicitRange` true -> auto-scope skipped. Unaffected. |
| `TestExportCSVRequiresEvent` | **KEEP** | No event -> `requireEventID` still returns 400 before the auto-scope block. Unaffected. |

## Risks / Edge Cases

- **Event record not found / missing dates**: `FindRecordById` error or parse failure -> block is a no-op, defaults remain. Safe.
- **Inverted event dates** (`start > end`): guarded by `!start.After(end)` -> no-op. Safe.
- **No event filter**: `requireEventID` returns 400 first, so the block never runs without an event. Safe.
- **Explicit range present**: `explicitRange` true -> block skipped entirely. No behavior change for existing callers.

## Definition of Done

- `TestExportCSVEventFilter` passes (both `ev1_only` and `ev2_only`).
- All `TestExportCSV*` tests pass.
- Only `handleExportCSV` in `attendance.go` was modified.
