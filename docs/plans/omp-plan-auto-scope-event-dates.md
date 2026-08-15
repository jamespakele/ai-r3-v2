# Working Plan: Auto-scope attendance matrix date range to selected event dates

## Objective

Modify `parseMatrixFilters` in `r3-intake/internal/server/attendance.go` so that when an event is in effect on the attendance matrix, the date range auto-scopes to that event's `start_date` → `end_date` instead of the current default 14-day window. Manual `from`/`to` overrides by the user must still win. When no event is in effect (or the event lacks valid dates), fall back to the existing 14-day default (today and prior 13 days).

## Constraints

- **Manual override wins.** Only when the user did *not* explicitly provide valid `from` AND `to` should the event's dates auto-scope the range. Explicit `from`/`to` always take precedence.
- **Event resolution must happen inside `parseMatrixFilters`.** The function needs to know which event is in effect to read its dates. This requires the events list, so the signature must change to accept `events []Event` and resolve the eventID internally via `effectiveEventID`.
- **Event span is authoritative — no 30-day cap.** The existing 30-day cap applies only to the default window. For event-scoped ranges, respect the full event duration (do NOT cap at 30 days).
- **Fallback rules.** If no event is in effect, or the in-effect event has a missing/invalid `start_date` or `end_date`, fall back to the 14-day default.
- **Consistency across callers.** Both `handleMatrix` and `handleStats` must be updated to the new signature so the matrix, stat cards, and toggle/walk-in forms all use the event-scoped range. The walk-in/toggle forms carry `from`/`to` as hidden fields (`DateFrom`/`DateTo`) from the view, so they inherit the scoped range automatically.
- **All timestamps in HST (UTC-10).** Date parsing/formatting uses the existing `"2006-01-02"` layout and `hst` location.
- **No behavior change to `effectiveEventID` or `loadEvents`.** They stay as-is; only their call sites move.

## File Structure

- `r3-intake/internal/server/attendance.go` — the only file modified by this card.
  - `parseMatrixFilters` (lines ~137–175): signature change + event auto-scope logic.
  - `handleMatrix` (lines ~85–92): update call to new signature; remove the now-redundant `effectiveEventID` call.
  - `handleStats` (lines ~196–203): update call to new signature; remove the now-redundant `effectiveEventID` call.
- `r3-intake/internal/server/attendance_test.go` — **no changes required** (existing tests render templates directly with a populated `MatrixViewData` and never call `parseMatrixFilters`; a sibling card `t_024eea03` adds new tests for this behavior).

## Implementation Notes

**1. Signature change (design decision #2).**
Change to:
```go
func (s *Server) parseMatrixFilters(r *http.Request, events []Event) (from, to, eventID string, dates []string)
```
Passing `events []Event` (rather than a pre-resolved eventID) is the cleanest approach: it keeps all event-resolution logic in one place, lets `parseMatrixFilters` call `effectiveEventID` internally, and avoids duplicating the "explicit wins, else first active" rule in the callers. Both callers already load `events` before calling, so the ordering is natural.

**2. Caller updates.**
- `handleMatrix`: load `events` first, then call `s.parseMatrixFilters(r, events)`. Delete the line `eventID = s.effectiveEventID(eventID, events)` — the returned `eventID` is now already effective.
- `handleStats`: same pattern — load `events`, call `s.parseMatrixFilters(r, events)`, delete the redundant `effectiveEventID` line.

**3. Auto-scope logic inside `parseMatrixFilters` (design decisions #1, #3, #4).**
- Track whether the user explicitly supplied valid `from` AND `to` (both parse successfully). Introduce a boolean, e.g. `explicitRange`, set `true` only when both `errFrom == nil && errTo == nil`.
- Resolve the effective event: `eventID = s.effectiveEventID(eventID, events)`.
- If `!explicitRange` and `eventID != ""`, look up the event in `events` by ID and read its `StartDate`/`EndDate`.
  - If both parse as `"2006-01-02"` and `start <= end`, set `from`/`to` to the event's dates and rebuild `dates = buildDateRange(from, to)`. **Skip the 30-day cap** for this path (event span is authoritative).
  - Otherwise (missing/invalid dates, or `start > end`), fall through to the default 14-day window.
- If `explicitRange` is true, keep the existing behavior: swap inverted ranges, apply the 30-day cap, build `dates`.
- The default 14-day window path (`defFrom`/`defTo`) remains unchanged and still applies the 30-day cap.

**4. Ordering of operations.** The current code parses/validates `from`/`to` first, then reads the event. Keep that structure but insert the event resolution and auto-scope after the explicit-range determination. The `eventID` query parsing (`event` then `event_id` fallback) stays as-is.

**5. Edge cases to handle.**
- Event selected but `start_date`/`end_date` empty or unparseable → default 14-day window.
- Event with `start_date > end_date` → treat as invalid, default 14-day window.
- No events at all → `effectiveEventID` returns `""` → default 14-day window.
- User explicitly sets `from`/`to` → those win regardless of event dates (manual override).
- Event span longer than 30 days → full span used, no truncation.

## Verification Criteria

- **Build/vet/test pass:**
  ```
  cd r3-intake && go build ./... && go vet ./... && go test ./...
  ```
- **Existing tests keep passing.** `attendance_test.go` (`TestMatrixContentRender`, `TestMatrixContentRenderEventRequired`, and the third render test) must still pass. They render templates directly with a populated `MatrixViewData` and do not call `parseMatrixFilters`, so the signature change must not break them.
- **No compile errors from the signature change.** Both `handleMatrix` and `handleStats` must be updated to the new `(r, events)` signature; no other callers of `parseMatrixFilters` exist.
- **Behavioral spot-checks (manual or via sibling card `t_024eea03` tests):**
  - Event with valid dates, no explicit `from`/`to` → range equals event `start_date`→`end_date`.
  - Event with valid dates, explicit `from`/`to` → explicit range wins.
  - Event with missing/invalid dates → 14-day default.
  - No event selected → 14-day default.
  - Event span > 30 days → full span used (no cap).
- **No changes to `attendance_test.go`** in this card; new behavior tests are owned by sibling card `t_024eea03`.
