# Working Plan: Add tests for event-scoped date range behavior

## Objective

Write Go unit tests that verify the event-scoped date range auto-scoping
behavior in the attendance matrix. The behavior lives in `parseMatrixFilters`
in `r3-intake/internal/server/attendance.go` (implemented by sibling card in
worktree `t_8863d450`, NOT yet present in this worktree).

The behavior under test:

- When the user has NOT explicitly set a valid `from`/`to` range and an event
  is in effect, the matrix date range auto-scopes to that event's
  `start_date` -> `end_date` (full span, authoritative, NO 30-day cap).
- Explicit user `from`/`to` ranges still win over event auto-scoping.
- Invalid/missing event dates, inverted event dates, and no-event cases all
  fall back to the 14-day default (today and the prior 13 days).

## Constraints

- Go module `r3-intake`, toolchain 3.12.3. Tests live in
  `r3-intake/internal/server/*_test.go`, `package server` (white-box, so
  unexported funcs are callable directly).
- `parseMatrixFilters` is a method on `*Server` but reads NO Server fields; it
  only reads the request query and the `events` slice. Tests can construct a
  bare `&Server{}` and an `httptest.NewRequest`; no PocketBase, no DB, no
  `newTestServer` harness needed.
- The 14-day default depends on `time.Now().In(hst)`. Tests MUST assert
  against the same now-based computation (recompute `defFrom`/`defTo` inside
  the test) or assert structural properties (`len(dates) == 14`, both parse as
  `2006-01-02`). NEVER hardcode wall-clock date strings.
- The current worktree does NOT yet contain the sibling's `parseMatrixFilters`
  change. The code under test must be copied in FIRST so the package compiles
  before any test is written.
- `hst` and `time` are already imported in `attendance.go`; the new test file
  needs only `net/http`, `net/http/httptest`, `strings`, `testing`, and
  `time`.

## File Structure

- `r3-intake/internal/server/attendance.go` — MODIFIED: copy the full modified
  file from sibling worktree `t_8863d450` (see Implementation Notes step 0).
- `r3-intake/internal/server/attendance_matrix_filters_test.go` — NEW: the
  table-driven test for `parseMatrixFilters` event-scoped date range behavior.

No other files change. The existing `attendance_test.go` is left untouched.

## Implementation Notes

### Step 0 (PREREQUISITE): copy the sibling implementation in first

The current worktree's `parseMatrixFilters` has signature
`func (s *Server) parseMatrixFilters(r *http.Request)` and does NOT auto-scope.
The sibling worktree `t_8863d450` has the modified version. Copy the FULL
modified `attendance.go` from the sibling to avoid partial-edit errors:

    cp /srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_8863d450/r3-intake/internal/server/attendance.go \
       r3-intake/internal/server/attendance.go

The sibling diff changes exactly:
- `handleMatrix` calls `s.parseMatrixFilters(r, events)` (the separate
  `s.effectiveEventID(eventID, events)` call is removed; the events are loaded
  before the call).
- `handleStats` calls `s.parseMatrixFilters(r, events)` the same way.
- `parseMatrixFilters` signature gains an `events []Event` param; the
  `effectiveEventID` call moves inside; the auto-scope block is added.
- `effectiveEventID` and `buildDateRange` are unchanged.

After copying, confirm the new signature is present:
`grep -n "func (s *Server) parseMatrixFilters" r3-intake/internal/server/attendance.go`
should show `(r *http.Request, events []Event)`.

### parseMatrixFilters signature (post-copy)

    func (s *Server) parseMatrixFilters(r *http.Request, events []Event) (from, to, eventID string, dates []string)

Supporting types/functions (unchanged, already in attendance.go):

    type Event struct {
        ID        string
        Name      string
        SiteID    string
        StartDate string
        EndDate   string
        Status    string
    }

    func (s *Server) effectiveEventID(eventID string, events []Event) string
    func buildDateRange(from, to string) []string

### How to construct the Server and request

    srv := &Server{}                       // bare; parseMatrixFilters uses no fields
    req := httptest.NewRequest("GET", "/matrix?event=ev1", nil)
    from, to, eventID, dates := srv.parseMatrixFilters(req, events)

Set query params via `req.URL.Query().Set(...)` then `req.URL.RawQuery =
req.URL.Query().Encode()`, or build the URL string directly. `events` is a
plain `[]Event` slice passed directly; no DB.

### 14-day default computation (reuse in assertions)

    now := time.Now().In(hst)
    defTo := now.Format("2006-01-02")
    defFrom := now.AddDate(0, 0, -13).Format("2006-01-02")

Assert `from == defFrom` and `to == defTo` (and `len(dates) == 14`) rather than
hardcoding dates. `hst` is the package-level `time.FixedZone("HST", -10*60*60)`
var already defined in attendance.go, so the test can reference it directly.

### Table-driven test structure

    func TestParseMatrixFiltersEventScopedDates(t *testing.T) {
        tests := []struct {
            name   string
            query  map[string]string   // query params to set
            events []Event
            wantFrom, wantTo string
            wantEventID string
            wantLen int
        }{ ... }
        for _, tt := range tests {
            t.Run(tt.name, func(t *testing.T) {
                srv := &Server{}
                req := httptest.NewRequest("GET", "/matrix", nil)
                q := req.URL.Query()
                for k, v := range tt.query { q.Set(k, v) }
                req.URL.RawQuery = q.Encode()
                from, to, eventID, dates := srv.parseMatrixFilters(req, tt.events)
                // assert from/to/eventID/dates
            })
        }
    }

For the default-fallback cases, compute `defFrom`/`defTo` inside the test and
assert equality. For event-scoped cases, assert exact event dates.

### Test cases to cover

1. **No explicit from/to + event selected** -> range auto-scopes to the
   event's `StartDate` -> `EndDate`; `dates` built from that full span.
   Query `event=ev1`; events `[{ID:"ev1", StartDate:"2026-01-05",
   EndDate:"2026-01-10"}]`; want `from="2026-01-05"`, `to="2026-01-10"`,
   `len(dates)==6`, `eventID=="ev1"`.

2. **No explicit from/to + no events** -> falls back to 14-day default.
   Query empty; `events=nil`; want `from==defFrom`, `to==defTo`,
   `len(dates)==14`, `eventID==""`.

3. **No explicit from/to + event selected but event has invalid/missing
   dates** -> falls back to 14-day default. Event with `StartDate:""` and
   `EndDate:""` (or non-`2006-01-02` values); want `from==defFrom`,
   `to==defTo`, `len(dates)==14`, `eventID=="ev1"` (event still resolved).

4. **No explicit from/to + event selected but start after end (inverted
   event dates)** -> falls back to 14-day default. Event `StartDate:"2026-01-10"`,
   `EndDate:"2026-01-05"`; want `from==defFrom`, `to==defTo`, `len(dates)==14`.

5. **Explicit from/to set** -> user range wins, NOT auto-scoped to event
   dates. Query `from=2026-02-01&to=2026-02-05` plus `event=ev1`; want
   `from=="2026-02-01"`, `to=="2026-02-05"`, `len(dates)==5`.

6. **Explicit from/to set but event also selected** -> user range wins.
   Same as case 5 (event present); assert the event dates are NOT used.

7. **Event span longer than 30 days** -> NOT capped (authoritative, full
   span). Event `StartDate:"2026-01-01"`, `EndDate:"2026-03-15"` (74 days);
   want `from=="2026-01-01"`, `to=="2026-03-15"`, `len(dates)==74`. This
   proves the 30-day cap is bypassed for event auto-scoping.

8. **eventID resolution** -> explicit `event` query wins; falls back to
   `event_id`; defaults to first event when neither given.
   - `event=evA&event_id=evB` -> `eventID=="evA"`.
   - `event_id=evB` only -> `eventID=="evB"`.
   - neither given, `events=[{ID:"ev1",...},{ID:"ev2",...}]` ->
     `eventID=="ev1"` (first event), and range auto-scopes to ev1's dates.

### Notes on edge behavior to assert precisely

- The auto-scope block runs only when `!explicitRange` (i.e. the user did not
  provide BOTH a valid `from` AND a valid `to`). A single invalid/missing
  param also counts as non-explicit, so it falls into the default path first
  and then auto-scopes if an event is in effect. Optionally add a case:
  `from` valid but `to` missing + event selected -> auto-scopes to event dates.
- The 30-day cap (`toT.Sub(fromT) > 30*24*time.Hour`) still applies to
  EXPLICIT user ranges (case 5/6 with a >30-day explicit range would be capped
  to 30 days) but is bypassed for event auto-scoping (case 7). Do not conflate
  the two.
- `eventID` is resolved via `effectiveEventID` BEFORE the auto-scope block, so
  the default-to-first-event path (case 8) also auto-scopes to that first
  event's dates when no explicit range is given.

## Verification Criteria

- `go build ./...` passes (proves the copied sibling `attendance.go` compiles
  with the new `parseMatrixFilters(r, events)` signature and updated
  `handleMatrix`/`handleStats` call sites).
- `go vet ./...` passes with no new warnings.
- `go test ./...` passes, including the new
  `TestParseMatrixFiltersEventScopedDates` and all pre-existing tests
  (`TestComputeSummary`, `TestMatrixContentRender`, etc.).
- Run the new test in isolation to confirm it exercises the behavior:
  `go test ./internal/server/ -run TestParseMatrixFiltersEventScopedDates -v`.
- The new test file contains no hardcoded wall-clock dates for the default
  cases; all default assertions derive from the same `time.Now().In(hst)`
  computation used by the implementation.
