# Working Plan: Add Tests for Records Filter with Attendance Join

## Objective

Add a new integration test file in `r3-intake/internal/server/` that exercises the Records screen event filter's **attendance join** behavior (an intake matches the selected event if its home `event` field equals it **OR** it has an `attendance` record with `attendance.intake == intake.id`). The sibling card's filter change lives in a separate worktree (`t_6e8ed9af`) and is **not** present in this worktree (`t_59fcfc64`, forked from master). This card must (1) bring the parent's `admin.go` `handleList` change into this worktree so the join is actually exercised, and (2) write a **new, differently-named** test file that adds **complementary** coverage beyond the sibling's 4 subtests, without colliding with the sibling's symbols when the epic merges all child worktrees.

## Constraints

- **Language/runtime:** Go, module `r3-intake`, package `server`.
- **PocketBase v0.39** embedded, real in-process instance booted by `newTestServer(t)` (defined in `attendance_export_integration_test.go`, present in master). It extracts embedded JS migrations from `../../pocketbase/migrations` to a temp dir, uses a fresh temp data dir, and runs `RunAllMigrations()`.
- **Shared helpers (reuse, do not redefine):** `newTestServer(t)`, `adminCookie(srv, id)`, `cmCookie(srv, id)` — all defined in `attendance_export_integration_test.go` (master). Reuse these.
- **Sibling's helpers (DO NOT reuse, DO NOT redefine with same names):** `listFixtures`, `seedListFixtures`, `doList`, `listRowNames`, `listCount` live in the sibling's `records_list_integration_test.go`, which is **not** in this worktree and will be merged in later. When the epic merges all child worktrees, both files coexist in package `server`; **any duplicate top-level symbol name is a compile error.** Therefore this card's new file must use **uniquely-named** helpers (e.g. `attJoinFixtures`, `seedAttJoinFixtures`, `doAttJoinList`, `attJoinRowNames`, `attJoinCount`).
- **Schema (from `007_events_attendance.js`):** `intake` fields `name`, `event` (relation to events), `status` (`unassigned`/`claimed`/`completed`). `attendance` fields `intake` (relation to intake, **required**), `event` (relation to events, **nullable**), `date` (text), `status` (`present`/`absent`/`excused`/`walk_in`). `events` fields `site`, `name`, `start_date`, `end_date`, `status`.
- **Filter semantics under test (from sibling's `admin.go` change):** when `?event=` is set, the filter becomes `(event='<id>' || id='<attendedIntake1>' || id='<attendedIntake2>' ...)` — a union of the home-event branch and the attendance-derived intake ids. All attendance statuses count. The `iid != ""` guard skips empty intake ids. The union is `&&`-composed with the status filter and the `?q=` free-text search.
- **Rendering assertions:** rows render as `class="admin-link">NAME</a>`; total renders as `class="admin-result-count">Showing N record(s)</p>`.

## File Structure

| Path | Action | Notes |
|------|--------|-------|
| `r3-intake/internal/server/admin.go` | **Modify** | Copy the sibling's `handleList` event-filter block (the attendance-join union) verbatim from `t_6e8ed9af` into this worktree. This is the only production-code change; it must match the sibling's diff exactly so the epic merge is clean. |
| `r3-intake/internal/server/records_list_attendance_join_integration_test.go` | **Create** | New test file. **Different filename** than sibling's `records_list_integration_test.go`. Contains uniquely-named fixtures/seed/doList/assert helpers plus the new test functions. |

No other files change. The sibling's `records_list_integration_test.go` is **not** copied into this worktree (it arrives via the epic merge).

## Implementation Notes

### 1. Bring the parent's filter change in
- Apply the exact diff from `t_6e8ed9af` (`git diff master -- r3-intake/internal/server/admin.go`) to this worktree's `admin.go`. The change replaces the single `parts = append(parts, fmt.Sprintf("event='%s'", ...))` line with the union block that queries `attendanceCollection()` via `s.pb.FindRecordsByFilter(attCol.Id, "event='<id>'", "intake", 5000, 0)`, dedups intake ids, and builds `(event='<id>' || id='...' || ...)`.
- Verify `s.attendanceCollection()` exists in this worktree (it does — `attendance.go:83`). No other production code is needed.

### 2. New test file — uniquely-named helpers
Because the sibling's `records_list_integration_test.go` will coexist in the same package after the epic merge, define **distinct** helper names:
- `type attJoinFixtures struct { site1, ev1, ev2, admin1, intakeA, intakeB, intakeC string }`
- `func seedAttJoinFixtures(t *testing.T, pb *pocketbase.PocketBase) attJoinFixtures`
- `func doAttJoinList(srv *Server, cookie *http.Cookie, query string) *httptest.ResponseRecorder`
- `func attJoinRowNames(t *testing.T, rec *httptest.ResponseRecorder, names ...string) []string`
- `func attJoinCount(t *testing.T, rec *httptest.ResponseRecorder) string`

These mirror the sibling's `doList`/`listRowNames`/`listCount` logic (GET `/` + query, scan for `class="admin-link">NAME</a>`, extract `class="admin-result-count">`), but under non-colliding names.

### 3. Fixture design (distinct from sibling's, richer)
Seed a site, two events (`ev1`, `ev2`), an admin user, and **three** intakes to cover the union edge cases:
- `intakeA` — home `ev1`, **attended `ev2`** (cross-event; surfaces via attendance for `ev2`).
- `intakeB` — home `ev2`, **also attended `ev2`** (home-event AND attendance both match to dedup case).
- `intakeC` — home `ev1`, **attended `ev1`** (home-event AND attendance both match for `ev1` to dedup case).
- Give distinct statuses (e.g. A=claimed, B=unassigned, C=completed) and distinct names so status+search composition is assertable.

### 4. Test functions (complementary coverage beyond sibling's 4 subtests)
- **`TestListEventFilterDedupsHomeAndAttendance`** — filter `?event=ev2`: `intakeB` matches via both home-event and attendance branches; assert it appears **exactly once** (row count == 1 for B, and total count matches). Also `?event=ev1` for `intakeC` (home + attendance for same event) to once.
- **`TestListEventFilterSurfacesMultipleAttendees`** — give `intakeA` and `intakeB` both attendance records for `ev2`; filter `?event=ev2` and assert **both** surface (union scales to N attendees), plus `intakeB`'s home-event match does not duplicate it.
- **`TestListEventFilterSkipsEmptyIntakeAttendance`** — **constraint:** the schema marks `attendance.intake` as `required: true`, so a record with an empty intake id **cannot be created** through normal `pb.Save`. Decision: do **not** attempt to create one (Save would fail). Instead, document that the `iid != ""` guard is defensive and covered by code review. *(If a runtime guard test is wanted, it would require bypassing schema validation — out of scope; note as a known limitation.)*
- **`TestListEventFilterComposesWithSearch`** — filter `?event=ev2&q=Al` (search matches `intakeA` "Alice" only): assert only `intakeA` surfaces (union `&&` search). Use a query >= 2 chars per the `?q=` min-length rule.
- **`TestListEventFilterComposesWithStatusAndSearch`** — `?event=ev2&status=claimed&q=Al`: assert only `intakeA` (claimed + name matches + attended ev2). This proves the three-way `&&` composition.
- **`TestListEventFilterCrossEventDistinct`** — distinct from sibling's subtest: `intakeA` (home `ev1`) attended `ev2`; filter `?event=ev2` and assert `intakeA` surfaces **even though its home event is a different event**, and `intakeC` (home `ev1`, no ev2 attendance) does **not**.
- **`TestListNoEventFilterReturnsAll`** — regression guard: `?event=` empty (and no other filters) returns **all** intakes (A, B, C), proving the join code path does not break the unfiltered list.

### 5. Edge cases / pitfalls
- **Dedup is the key risk:** the union `(event='ev2' || id='B' || ...)` — if `intakeB`'s home event is `ev2` AND it has an attendance record for `ev2`, the filter matches it via both branches but PocketBase returns each record once. Assert via row-name count (not just presence) to catch accidental duplication.
- **`?q=` min length:** search only activates at `len(query) >= 2`; use `q=Al`/`q=Bo`, never a 1-char query.
- **Status values:** only `unassigned`/`claimed`/`completed` activate the status filter; use those exact strings.
- **Count assertion:** `attJoinCount` must match the number of distinct rows (e.g. "Showing 2 records"), guarding against phantom/duplicate rows.
- **Merge safety:** never reuse the sibling's helper names; never copy the sibling's test file into this worktree.

## Logical Consequences

Every downstream site affected by this card, and the decision for each:

| Site | Location | Decision |
|------|----------|----------|
| `handleList` event filter | `r3-intake/internal/server/admin.go` | **Change** — copy the sibling's attendance-join union block into this worktree so the new tests exercise the real join. Must match the sibling's diff exactly for a clean epic merge. |
| `records_list_integration_test.go` (sibling) | `t_6e8ed9af` worktree | **Keep** — do not copy into this worktree; it arrives via the epic merge. Do not reuse its helper names. |
| `attendance_export_integration_test.go` (master) | this worktree | **Keep** — reuse `newTestServer`, `adminCookie`, `cmCookie`; do not redefine. |
| `attendance.go` (`attendanceCollection`) | this worktree | **Keep** — already present; required by the copied filter code. No change. |
| `007_events_attendance.js` (schema) | `pocketbase/migrations/` | **Keep** — no schema change; `attendance.intake` is `required:true`, which makes the empty-intake-id subtest non-creatable (documented limitation). |
| New test file | `records_list_attendance_join_integration_test.go` | **Create** — uniquely-named helpers + 7 test functions covering dedup, multi-attendee, search composition, status+search composition, cross-event, and no-filter regression. |
| `admin.go` `handleList` in sibling | `t_6e8ed9af` | **Keep** — source of the diff; do not modify. |

## Verification Criteria

1. **Build:** `cd r3-intake && go build ./...` succeeds in this worktree after copying the `admin.go` change.
2. **Vet:** `go vet ./internal/server/` clean.
3. **Run the new tests:** `go test ./internal/server/ -run 'TestListEventFilter(DedupsHomeAndAttendance|SurfacesMultipleAttendees|ComposesWithSearch|ComposesWithStatusAndSearch|CrossEventDistinct)|TestListNoEventFilterReturnsAll' -v` — all pass against the real in-process PocketBase.
4. **Run the full server package suite:** `go test ./internal/server/` — no regressions in existing tests (the `admin.go` change is additive to the event-filter branch only).
5. **Merge-safety check:** confirm the new file's top-level identifiers (`attJoinFixtures`, `seedAttJoinFixtures`, `doAttJoinList`, `attJoinRowNames`, `attJoinCount`, and the test funcs) do **not** collide with any identifier in the sibling's `records_list_integration_test.go` (`listFixtures`, `seedListFixtures`, `doList`, `listRowNames`, `listCount`, `TestListEventFilterJoinsAttendance`). A quick `grep` across both files for shared symbol names is sufficient.
6. **Behavioral spot-check:** the dedup test must fail if the filter ever returns a duplicate row; the no-filter test must fail if the join breaks the unfiltered list.
