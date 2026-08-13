# Working Plan: Add Unit and Integration Tests for CSV Attendance Export

> **For Hermes:** Implement this plan task-by-task. This card writes **tests only** — the CSV export implementation already exists (sibling card t_54a1f19a) and is present in this worktree. Do not modify `attendance.go`, `server.go`, or `auth.go`.

**Goal:** Add unit and integration tests in `r3-intake/internal/server/` that verify the CSV export route (`GET /attendance/export`) across date-range filters, site/event filters, header validation, field formatting, admin permissions, and empty result sets.

**Architecture:** Split into (a) pure unit tests for the pure functions `exportCSVRecords`, `summaryCSVRow`, `exportStatus` (no `*Server`/PB dependency) and (b) route-level integration tests that boot a real in-process PocketBase via the `runMCP` bootstrap pattern with a temp data dir, then drive the handler through `httptest` with a signed admin session cookie.

**Tech Stack:** Go 1.25, `net/http/httptest`, `encoding/csv`, `pocketbase v0.39.9` (in-process), embedded JS migrations.

---

## Objective

Verify the CSV export feature end-to-end:

1. **Pure unit tests** for the CSV table builder and summary math (fast, no DB).
2. **Integration tests** that exercise the real handler + PB data path: permissions (admin vs case_manager vs unauthenticated), date-range filter behavior (defaults, swap, 30-day cap), site/event filters, header/field formatting through the wire, and empty result sets.
3. All tests pass on branch `epic/3-csv-export` under `go build ./... && go vet ./... && go test ./...`.

The sibling already added `TestExportCSVRecords` (pure). This plan **extends** it with additional pure cases and adds the integration layer — it does not duplicate it.

---

## Constraints

- **Language:** Go 1.25.0 (`go.mod`), module `r3-intake`.
- **Package:** `server` (white-box, same package as code under test) — tests can call unexported `exportCSVRecords`, `summaryCSVRow`, `exportStatus`, `handleExportCSV`, `loadExportRows`, `nameFor`, `makeSession`, `parseSession`, `requireRole`.
- **No `app.dao()`:** PocketBase v0.39 uses `app.FindCollectionByNameOrId`, `app.FindRecordsByFilter`, `app.FindRecordById`, `app.Save`, `app.Delete` (the `core.App` interface). The `Server` struct holds `pb *pocketbase.PocketBase`; all data access goes through `s.pb.*`.
- **Timezone:** All timestamps are HST (`var hst = time.FixedZone("HST", -10*60*60)` in `admin.go`). `handleExportCSV` computes default from/to from `time.Now().In(hst)`. Tests must not assert on the exact default date strings (they depend on wall-clock "today"); assert structural properties instead (e.g. `from` is 13 days before `to`, both parse as `2006-01-02`).
- **Existing helpers:** `makeSession(u *sessionUser) string` (HMAC-signed cookie value), `parseSession`, `requireRole(role, h)`, `currentSession(r)`, `resolveSite(u, param)`, `formatTime(s)`, `mcpmod.EscapeFilter`. Reuse these; do not reimplement.
- **No existing PB integration harness:** all current tests are pure. This card introduces the first in-process PB boot in tests. It must be self-contained (temp data dir, `t.Cleanup`), not depend on a running server or the repo's `pocketbase/pb_data`.
- **No `cmd/` in this worktree:** the bootstrap pattern lives in the parent repo at `r3-intake/cmd/r3-intake/main.go` (`runMCP`). Replicate that pattern in the test helper; do not import `cmd`.
- **Do not modify production code** (`attendance.go`, `server.go`, `auth.go`, `handlers.go`, `admin.go`). Only add/modify `*_test.go` files.

---

## File Structure

**Create:**
- `r3-intake/internal/server/attendance_export_test.go` — pure unit tests for `exportStatus`, `summaryCSVRow`, and additional `exportCSVRecords` cases (header validation, field formatting, empty result set). Keeps the pure tests separate from the sibling's `attendance_test.go` `TestExportCSVRecords`.
- `r3-intake/internal/server/attendance_export_integration_test.go` — route-level integration tests. Contains the PB boot helper (`newTestServer`) and all handler tests.

**Modify:**
- `r3-intake/internal/server/attendance_test.go` — **no change required.** The sibling's `TestExportCSVRecords` stays as-is. (Optional, non-blocking: leave untouched to avoid churn.)

**No production files change.**

---

## Implementation Notes

### Integration-test approach: real in-process PocketBase (recommended)

**Recommendation: boot a real in-process PocketBase** via the `runMCP` bootstrap pattern, using a temp data dir. Rationale:

- `handleExportCSV` -> `loadExportRows` -> `nameFor` all depend on `s.pb` (a concrete `*pocketbase.PocketBase`). There is no interface seam to fake; a lightweight fake would require re-architecting production code, which is out of scope and against repo conventions.
- The repo already has the exact bootstrap recipe in `runMCP` (`pocketbase.NewWithConfig` + `jsvm.MustRegister` + `pbmigrations.Register` + `pb.Bootstrap()`), and `internal/migrations.Dir("")` extracts the embedded JS migrations to a temp dir. Reusing it is robust and matches how the app actually boots.
- The `007_events_attendance.js` migration creates the `events`, `event_enrollment`, and `attendance` collections with the exact fields the export reads, so the integration tests exercise the real schema.

**Test helper `newTestServer(t) (*Server, *pocketbase.PocketBase)`:**

```go
func newTestServer(t *testing.T) (*Server, *pocketbase.PocketBase) {
    t.Helper()
    migrationsDir, err := migrations.Dir("") // temp dir with embedded JS migrations
    if err != nil { t.Fatal(err) }
    dataDir := t.TempDir()
    pb := pocketbase.NewWithConfig(pocketbase.Config{
        DefaultDataDir:  dataDir + "/pb_data",
        HideStartBanner: true,
    })
    jsvm.MustRegister(pb, jsvm.Config{MigrationsDir: migrationsDir})
    pbmigrations.Register(pb)
    if err := pb.Bootstrap(); err != nil { t.Fatal(err) }
    t.Cleanup(func() { pb.Dispose() })

    cfg := config.Config{
        SessionKey: "test-session-key",
        PBInternalAddr: "127.0.0.1:8091", // unused by handlers under test
        PBRootDir:  dataDir,
        DataDir:    dataDir,
    }
    srv, err := New(cfg, pb)
    if err != nil { t.Fatal(err) }
    return srv, pb
}
```

**Seed helper `seedExportData(t, pb)`** — creates via `core.NewRecord` + `pb.Save`:
- 2 sites (`sites`): `site1` "Kona", `site2` "Waianae" (active=true).
- 2 events (`events`): `ev1` "Morning Program" @ site1, `ev2` "Job Fair" @ site2 (status=active).
- 2 users (`users`): `admin1` role=admin, `cm1` role=case_manager (name + email + password set).
- 2 intakes (`intake`): `i1` "Alice" @ site1, `i2` "Bob" @ site2.
- Attendance records (`attendance`) spanning a known date window, e.g. `2026-08-01`..`2026-08-05`, with a mix of statuses (`present`, `absent`, `excused`, `walk_in`), some with `event` set, some with empty `event`, `recorded_by` set on some, `check_in_time` and `note` populated on some. Return the created record IDs so tests can assert on resolved names.

**Session cookie helper `adminCookie(t, srv)` / `cmCookie(t, srv)`:** build a `*http.Cookie` from `srv.makeSession(&sessionUser{ID:..., Role:"admin"|"case_manager", ...})` and attach it to the request. For the unauthenticated case, send no cookie.

**Request helper:** `doExport(t, srv, cookie *http.Cookie, query string) *httptest.ResponseRecorder` — builds `httptest.NewRequest("GET", "/attendance/export"+query, nil)`, adds the cookie, calls `srv.Mux().ServeHTTP(rec, req)`, returns the recorder. Use `srv.Mux()` so `requireRole` wrapping is exercised exactly as in production.

**CSV parsing helper:** `parseCSVBody(t, rec)` — `csv.NewReader(strings.NewReader(rec.Body.String()))`, `ReadAll()`, return `[][]string`. Assert `Content-Type` and `Content-Disposition` headers separately.

---

### Pure unit tests (`attendance_export_test.go`)

**`TestExportStatus`** — table-driven over `exportStatus`:
- `"present"` -> `"Present"`, `"absent"` -> `"Absent"`, `"excused"` -> `"Excused"`, `"walk_in"` -> `"Walk-in"`.
- Unknown/empty values (`""`, `"foo"`) -> `""`.

**`TestSummaryCSVRow`** — table-driven over `summaryCSVRow`:
- Empty rows -> `"Summary: 0 check-ins, 0 unique participants, 0% avg rate"` (no division-by-zero panic).
- present + walk_in both count as check-ins; absent/excused do not.
- Unique participants counted by non-empty `ParticipantName` (duplicate names collapse; empty names ignored).
- Rate = `presentCount*100/(uniqueParticipants*days)` where days = distinct `Date` values; assert integer truncation (e.g. 1 present, 2 unique, 2 days -> 25%).
- All columns after index 0 are empty strings; row length is 8.

**`TestExportCSVRecords_Header`** — assert `records[0]` equals exactly `["Participant","Site","Event","Date","Status","Recorded By","Check-in Time","Note"]` (order-sensitive). Complements the sibling's header check by asserting the full slice equality.

**`TestExportCSVRecords_FieldFormatting`** — one row per status asserting the title-cased status lands in column 4, and that `CheckInTime`/`Note`/`RecordedByName` pass through verbatim (including a comma-containing note, which the sibling already covers — extend with a `check_in_time` value and a `walk_in` status to confirm `"Walk-in"`).

**`TestExportCSVRecords_Empty`** — `exportCSVRecords(nil)` returns exactly 2 rows: the header row + the empty summary row (`"Summary: 0 check-ins, 0 unique participants, 0% avg rate"`). This is the pure-function empty-result-set case.

> These extend, not duplicate, the sibling's `TestExportCSVRecords` (which already covers header cells, title-casing, empty relations, comma note, and a populated summary). Keep the sibling test untouched.

---

### Integration tests (`attendance_export_integration_test.go`)

**`TestExportCSVPermissions`** — route-level permission matrix via `srv.Mux()`:
- **Unauthenticated** (no cookie): expect `303` redirect to `/login` (Location header), empty body, no CSV headers.
- **case_manager** (valid `cm1` cookie): expect `303` redirect to `/login` (role mismatch), empty body.
- **admin** (valid `admin1` cookie): expect `200`, `Content-Type: text/csv`, `Content-Disposition` containing `attachment; filename="attendance_export_...csv"`, and a parseable CSV body.
- **Tampered cookie** (valid admin cookie value with one char flipped): expect `303` redirect to `/login` (parseSession returns nil on bad sig).

**`TestExportCSVDateRangeFilter`** — seed records on `2026-08-01` and `2026-08-10`; request `?from=2026-08-05&to=2026-08-12`:
- Assert only the `2026-08-10` record appears in the body (date filter excludes out-of-range rows).
- **Swap behavior:** request `?from=2026-08-12&to=2026-08-05` -> same result as the ordered range (handler swaps when from>to).
- **30-day cap:** request `?from=2026-01-01&to=2026-12-31` -> assert the summary row's day count reflects a capped range (records outside the capped 30-day window are excluded). Assert via the summary text (e.g. unique-participant/days math) rather than exact dates.
- **Defaults:** request with no from/to -> assert the response is `200` and the summary row is present; do **not** assert exact default dates (they depend on wall-clock HST). Optionally assert the body contains only records whose `date` falls within the last 14 days relative to `time.Now().In(hst)`.

**`TestExportCSVSiteFilter`** — seed records at site1 and site2:
- `?site=site1` -> only site1 records present; `Site` column shows `"Kona"`.
- `?site=site2` -> only site2 records present; `Site` column shows `"Waianae"`.
- `?site=does-not-exist` -> admin with invalid site resolves to `("", "All locations")` -> all records present (no filtering).
- No `site` param -> all records present.

**`TestExportCSVEventFilter`** — seed records with `event=ev1`, `event=ev2`, and empty event:
- `?event=ev1` -> only ev1 records present; `Event` column shows `"Morning Program"`.
- `?event=ev2` -> only ev2 records present; `Event` column shows `"Job Fair"`.
- No `event` param -> all records present (including empty-event rows).

**`TestExportCSVHeaderAndFormatting`** — full-wire assertion:
- Parse the CSV body; assert row 0 equals the exact header.
- Assert a known seeded record renders with title-cased status, resolved `Participant`/`Site`/`Event`/`Recorded By` names (via `nameFor`), and verbatim `Date`/`Check-in Time`/`Note`.
- Assert the last row is the summary row (first cell starts with `"Summary:"`, remaining 7 cells empty).
- Assert `Content-Type: text/csv` and `Content-Disposition` header values.

**`TestExportCSVEmptyResultSet`** — request with a date range that matches no records (e.g. `?from=1990-01-01&to=1990-01-31`):
- Assert `200`, valid CSV, exactly 2 rows: header + `"Summary: 0 check-ins, 0 unique participants, 0% avg rate"`.
- This is the handler-level empty-result-set case (complements the pure `TestExportCSVRecords_Empty`).

**`TestExportCSVNameResolution`** — direct unit-style test of `loadExportRows`/`nameFor` against the booted PB:
- `nameFor("sites", site1)` -> `"Kona"`; `nameFor("sites", "")` -> `""`; `nameFor("sites", "nonexistent")` -> `""` (failed lookup).
- `loadExportRows("", "", "2026-08-01", "2026-08-05")` returns rows with resolved names and correct field mapping (status raw, check_in_time formatted via `formatTime`).

---

### Edge cases to cover

- **Division-by-zero:** empty result set must not panic in `summaryCSVRow` (rate guard `len(seen)>0 && len(days)>0`).
- **Empty relations:** records with empty `event`/`recorded_by` render as `""` (not `"<nil>"`) — covered in pure + integration.
- **Comma/quote in fields:** a note containing a comma must be quoted by `csv.Writer` and round-trip through `csv.Reader` as a single field — assert in `TestExportCSVRecords_FieldFormatting` and the integration header/formatting test.
- **Role mismatch redirect:** case_manager and unauthenticated both get `303` to `/login` (not `403`) — matches `requireRole` behavior.
- **Tampered signature:** a corrupted cookie is treated as unauthenticated.
- **Date swap + cap:** from>to swaps; >30-day ranges are capped to 30 days from the start date.

---

## Verification Criteria

Run from the repo root (`/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_7c6efa05`):

```bash
cd r3-intake
go build ./...
go vet ./...
go test ./...
```

Expected:
- `go build ./...` — pass (no production code changed).
- `go vet ./...` — pass.
- `go test ./...` — all pass, including the sibling's `TestExportCSVRecords`, the new pure tests, and the new integration tests. The integration tests boot PB in-process with a temp data dir and must not require a running server or network.

**Acceptance checklist:**
- [ ] Pure tests: `TestExportStatus`, `TestSummaryCSVRow`, `TestExportCSVRecords_Header`, `TestExportCSVRecords_FieldFormatting`, `TestExportCSVRecords_Empty`.
- [ ] Integration tests: `TestExportCSVPermissions`, `TestExportCSVDateRangeFilter`, `TestExportCSVSiteFilter`, `TestExportCSVEventFilter`, `TestExportCSVHeaderAndFormatting`, `TestExportCSVEmptyResultSet`, `TestExportCSVNameResolution`.
- [ ] All required scenarios covered: date range filters, site/event filters, header validation, field formatting, admin permissions, empty result sets.
- [ ] No production files modified; only `*_test.go` files added.
- [ ] `go build ./... && go vet ./... && go test ./...` all green on `epic/3-csv-export`.
