# Working Plan: Remove walk-in UI, handlers, routes, CSS, and tests

> **For Hermes:** Use subagent-driven-development to implement this plan task-by-task.

**Goal:** Delete the "Add walk-in" button, its search/create panel, and every now-dead walk-in handler, route, template block, CSS class, and test from the attendance matrix — a PURE REMOVAL with no new UI.

**Architecture:** The walk-in feature is a self-contained vertical slice (template block + 2 handlers + 2 routes + CSS + tests). Removing it touches 5 files. The `walk_in` attendance STATUS is a separate, still-valid concept and is explicitly KEPT.

**Tech Stack:** Go server + embedded PocketBase, server-rendered Go templates (single `index.html` with `{{define}}` blocks), HTMX + Alpine.js, vanilla CSS.

---

## Objective

Remove the "Add walk-in" button and its search/create panel from the attendance matrix, and remove all the now-dead walk-in handlers, routes, template blocks, CSS classes, and tests. The walk-in concept is being retired because everyone is a walk-in and the "Add walk-in" button just created a blank intake form anyway. A sibling card adds an "Intake" button and standardizes button labels — this card does NOT add any UI.

## Constraints

1. **PURE REMOVAL.** Do NOT add any new buttons, panels, or UI. A sibling card owns adding the "Intake" button and standardizing button labels.
2. **KEEP the `walk_in` STATUS concept.** `walk_in` is a valid attendance status. The following MUST remain untouched:
   - `statusLabel` switch case `case "walk_in": return "Walk-in"` (attendance.go ~L1112-1113).
   - `person_attendance.go` walk_in references (status options ~L195, ~L391).
   - `attendance_export_test.go` walk_in references (status mapping, check-in counting).
   - `attendance_test.go` walk_in status references (WalkInCount, statusLabel tests, export CSV walk_in rows).
   - `attendance_roster_integration_test.go` L165 comment "an out-of-site walk-in under ev1" — this tests a `walk_in` STATUS record, not the walk-in UI. KEEP the test and comment.
3. **KEEP the `EventRequired` field and its `{{if not .EventRequired}}` / `{{if .EventRequired}}` guards in the person-attendance calendar template** (index.html ~L1447, ~L1458). Only the matrix-content walkin-panel block is removed. The `EventRequired` field itself stays (it is still read by the person-attendance calendar).
4. **KEEP the `event_id` fallback in `parseMatrixFilters`** (attendance.go ~L163) — it is still used by the toggle forms. Only the comment wording drops "walk-in".
5. **No schema/migration changes.** This is UI/handler/route/CSS/test removal only. No PB collections or fields change.
6. **No CSS cache-buster bump needed** unless the stylesheet link is touched — CSS classes are being removed, not added, so the `?v=` version can stay (verify; bump only if the link line changes).

## File Structure (files to modify)

| File | Change |
|------|--------|
| `r3-intake/internal/assets/public/index.html` | Remove the `{{if not .EventRequired}}` walkin-panel block in `matrix-content` (L1182-1208) and the entire `walkin-results` define block (L1276-1286). |
| `r3-intake/internal/server/attendance.go` | Remove `walkinResult` struct (L753-760), `handleWalkinSearch` (L765-818), `handleWalkin` (L823-944). Update 2 comments (L32, L163). |
| `r3-intake/internal/server/server.go` | Remove 2 routes (L147-148). |
| `r3-intake/internal/assets/public/app.css` | Remove 11 `.walkin-*` rules (L310-324). |
| `r3-intake/internal/server/attendance_toggle_integration_test.go` | Remove `TestWalkinRequiresEvent` (L212-238), `doWalkin` helper (L410-421), `TestWalkinStoresNoSite` (L425-~450). |
| `r3-intake/internal/server/attendance_test.go` | Update `TestMatrixContentRenderEventRequired` (L125) and `TestMatrixContentRenderEventScopedFormDates` (L243) comments/assertions. |
| `r3-intake/internal/server/attendance_matrix_default_integration_test.go` | Update `TestMatrixDefaultsToFirstEvent` (L27) and `TestMatrixNoEventsEmptyState` (L60). |

## Implementation Notes

### Step 1 — Template: remove the walkin-panel block (index.html)

In the `matrix-content` define block, delete the entire block from the `{{if not .EventRequired}}` line (L1182) through its matching `{{end}}` (L1208), inclusive. This removes:
- The "Add walk-in" toggle button (`#walkin-toggle`).
- The `walkin-search` div with hidden `site_id`/`from`/`to`/`event_id` inputs.
- The search input (`hx-get="/attendance/walkin-search"`).
- The `walkin-results` div.
- The `walkin-create` form (`action="/attendance/walkin"`) with "Check in walk-in" button.

The surrounding content stays: the `{{if .NoEvents}}` empty-state block above (L1178-1180) and the `{{if and .IsAdmin .EventLocation}}` location line below (L1210-1212). After removal, `matrix-content` no longer references `.EventRequired`, `.DateFrom`, `.DateTo`, `.SiteID`, or `.EventID` for the walk-in panel — but these view fields are still used elsewhere (matrix-cell forms use `.From`/`.To`/`.SiteID`/`.EventID`; `.DateFrom`/`.DateTo` are still used by the matrix header/date row). Verify with `grep -n "DateFrom\|DateTo\|EventRequired" index.html` that remaining uses are legitimate.

### Step 2 — Template: remove the `walkin-results` define block (index.html)

Delete the entire `{{define "walkin-results"}}` block (L1276-1286), including the trailing blank line. This block is only referenced by `handleWalkinSearch` (being removed in Step 4), so it becomes dead. Verify no other `ExecuteTemplate(w, "walkin-results", ...)` call remains.

### Step 3 — CSS: remove `.walkin-*` rules (app.css)

Delete lines L310-324 (the 11 rules: `.walkin-panel`, `.walkin-search`, `.walkin-search.hidden`, `.walkin-search .field-input`, `.walkin-results`, `.walkin-result`, `.walkin-result-form`, `.walkin-result-btn`, `.walkin-result-btn:hover`, `.walkin-create`, `.walkin-create .field-input`). Note: `.walkin-search-filters` has NO CSS rule (grep found none) — it is a class-only wrapper, so nothing to remove for it. Keep the `.matrix-*` rules above and the `/* Stat cards */` section below intact.

### Step 4 — Handlers: remove walk-in handlers (attendance.go)

Delete, in order:
- `type walkinResult struct` (L753-760).
- `handleWalkinSearch` (L765-818) — the GET fragment handler.
- `handleWalkin` (L823-944) — the POST create/update handler.

Update the two comments:
- L32: `// EventRequired reports that no event is selected, so the matrix must disable toggling and hide the walk-in panel.` → drop the "hide the walk-in panel" clause, e.g. `// EventRequired reports that no event is selected, so the matrix must disable toggling.` (The field is still used by the person-attendance calendar.)
- L163: `// and falls back to "event_id" (toggle/walk-in forms).` → `// and falls back to "event_id" (toggle forms).` (The fallback itself is KEPT — toggle forms still send `event_id`.)

### Step 5 — Routes: remove walk-in routes (server.go)

Delete L147-148:
```go
mux.HandleFunc("/attendance/walkin-search", s.csrfMiddleware(s.requireAuth(s.handleWalkinSearch)))
mux.HandleFunc("/attendance/walkin", s.csrfMiddleware(s.requireAuth(s.handleWalkin)))
```
Keep the `/attendance/toggle` route (L146) and the PB admin proxy comment (L150) intact.

### Step 6 — Tests: remove walk-in tests (attendance_toggle_integration_test.go)

Delete:
- `TestWalkinRequiresEvent` (L212-238).
- `doWalkin` helper (L410-421).
- `TestWalkinStoresNoSite` (L425 through its closing brace ~L450).

Keep `TestToggleEventScoped`, `TestToggleScopesPerEvent`, `cloneValues`, and all other toggle tests. Verify no remaining reference to `doWalkin` or `/attendance/walkin` in this file.

### Step 7 — Tests: update matrix render tests (attendance_test.go)

- **`TestMatrixContentRenderEventRequired`** (L125): The `forbid` list contains `"Add walk-in"` and `"walkin-search"` (L175-176). After removal these become trivially-true (the strings can never appear) but harmless. **Clean them up** — remove both from the forbid list and update the doc comment (L123-124) to drop "hides the walk-in panel". The test still verifies the create-an-event empty state and disabled cells.
- **`TestMatrixContentRenderEventScopedFormDates`** (L243): The 4 assertions are:
  - `<input type="hidden" name="from" value="2026-08-01">` — matches BOTH the walk-in panel's hidden input AND the matrix-cell toggle form's hidden input (matrix-cell renders `<input type="hidden" name="from" value="{{.From}}">`). **KEEP** — still matches the toggle forms.
  - `<input type="hidden" name="to" value="2026-08-31">` — same, **KEEP**.
  - `name="from" value="2026-08-01"` — matches the toggle forms. **KEEP**.
  - `name="to" value="2026-08-31"` — matches the toggle forms. **KEEP**.
  All four assertions still pass after removal because the matrix-cell toggle forms render the same hidden from/to inputs. **Only update the doc comment** (L243-245) to drop the "walk-in panel's hidden from/to inputs" mention and state it verifies the matrix-cell toggle forms' event-scoped dates. No assertion changes needed.

### Step 8 — Tests: update matrix default integration tests (attendance_matrix_default_integration_test.go)

- **`TestMatrixDefaultsToFirstEvent`** (L27): The `want` list contains `"Add walk-in"` (L40). **Remove it** — the string no longer appears in the body. Update the doc comment (L27-28) to drop "and the walk-in panel is available". Keep the other wants (`<option ... selected>`, "Located Alice", "NoSite Bob").
- **`TestMatrixNoEventsEmptyState`** (L60): The `forbid` list contains `"Add walk-in"` and `"walkin-search"` (L79-80). These become trivially-true but harmless. **Clean them up** — remove both, and update the doc comment (L60-61) to drop "hides the walk-in panel". Keep `"Select an event…"` in forbid.

## Logical Consequences (MANDATORY — trace every downstream reference)

Trace every reference to the removed symbols to confirm nothing else breaks:

1. **`handleWalkinSearch` / `handleWalkin`** — referenced ONLY by the two routes in server.go (L147-148). No other call sites. Removing routes + handlers is self-consistent. `grep -rn "handleWalkin" internal/` must return only the (now-removed) definitions and routes.

2. **`walkinResult` struct** — referenced only by `handleWalkinSearch`. Safe to remove.

3. **`walkin-results` template block** — referenced only by `handleWalkinSearch` (`ExecuteTemplate(w, "walkin-results", results)`). Safe to remove. `grep -rn "walkin-results" internal/` must return only the template block (now removed).

4. **`/attendance/walkin-search` and `/attendance/walkin` routes** — referenced only by the template (hx-get and form action) and the tests. Template refs removed in Step 1-2; test refs removed in Step 6. `grep -rn "walkin-search\|/attendance/walkin" internal/` must return nothing after all steps.

5. **`.walkin-*` CSS classes** — referenced only by the removed template blocks. No other template uses them. `grep -rn "walkin-" internal/assets/public/index.html` must return nothing after Steps 1-2.

6. **`EventRequired` field** — KEPT. Still read by:
   - `matrix-content`? NO — after Step 1 the matrix-content block no longer references it. But it IS still read by the person-attendance calendar template (index.html L1447 `{{if .EventRequired}}`, L1458 `{{if not $.EventRequired}}`). The field and its Go population logic in attendance.go must stay. `grep -n "EventRequired" internal/` must still show the struct field, its population, and the person-attendance template uses.

7. **`DateFrom` / `DateTo` view fields** — still used by the matrix header/date row and the matrix-cell forms (via `.From`/`.To`). Do NOT remove. Verify remaining uses with grep.

8. **`event_id` fallback in `parseMatrixFilters`** — KEPT. The toggle forms still POST `event_id` (matrix-cell template L1268). Only the comment changes.

9. **`walk_in` STATUS** — KEPT everywhere: `statusLabel` (attendance.go L1112), `person_attendance.go` status options (L195, L391), export check-in counting (attendance.go L1128), and all walk_in status tests. `grep -rn "walk_in" internal/` must still show these. The `WalkInCount` field (attendance.go L58) and its population (L439-440, L461) are KEPT — they count walk_in STATUS records, not the walk-in UI.

10. **`doWalkin` helper** — referenced only by `TestWalkinStoresNoSite`. Removing both is consistent. `grep -n "doWalkin" attendance_toggle_integration_test.go` must return nothing after Step 6.

11. **`requireEventID`** — used by `handleWalkin` (L826) AND by other handlers (toggle). KEEP the function; only its walk-in call site disappears.

12. **`resolveSite`** — used by `handleWalkin` (L824) and other handlers. KEEP.

13. **`intakeCollection` / `attendanceCollection` / `eventsCollection`** — used by `handleWalkinSearch`/`handleWalkin` and many other handlers. KEEP.

14. **`mcpmod.EscapeFilter`** — used by walk-in handlers and many others. KEEP.

15. **`seedToggleData` / `findAttendance` / `countAttendance` / `adminCookie` / `doMatrix` / `newTestServer`** test helpers — used by walk-in tests AND many other tests. KEEP all; only the walk-in-specific tests/helper are removed.

16. **`TestWalkinRequiresEvent`** asserts the canonical message `"an event must be selected before recording attendance"` — this message is produced by `requireEventID`, which is still used by toggle. The message itself is NOT removed (it is a shared helper), only the walk-in test that exercised it via the walk-in route. No other test asserts this message via `/attendance/walkin`.

## Verification Criteria

1. **Build:** `cd r3-intake && go build ./...` — must pass with no references to removed symbols.
2. **Vet:** `go vet ./...` — must pass.
3. **Tests:** `go test ./...` — must pass. Specifically:
   - `attendance_toggle_integration_test.go` compiles (no `doWalkin`/`TestWalkin*` references).
   - `attendance_test.go` `TestMatrixContentRenderEventScopedFormDates` still passes (the 4 from/to assertions match the matrix-cell toggle forms).
   - `attendance_matrix_default_integration_test.go` `TestMatrixDefaultsToFirstEvent` passes without "Add walk-in" in want.
4. **Grep cleanliness** (each as its own command, joined with `;` not `&&`):
   - `grep -rn "walkin" internal/` → only `walk_in` STATUS matches (statusLabel, person_attendance, export, WalkInCount) and the roster test comment. NO `walkin-panel`, `walkin-search`, `walkin-result`, `walkin-create`, `handleWalkin`, `/attendance/walkin`.
   - `grep -rn "Add walk-in" internal/` → nothing.
   - `grep -n "EventRequired" internal/server/attendance.go internal/assets/public/index.html` → still shows the field + person-attendance calendar uses.
   - `grep -n "walk_in" internal/server/attendance.go` → still shows statusLabel case + check-in counting.
5. **Manual render check:** The attendance matrix must still render and toggle correctly without the walk-in panel — the matrix table, matrix-cell toggle forms, event dropdown, and stat cards all present; toggling a cell still works (covered by `TestToggleEventScoped`/`TestToggleScopesPerEvent`).
6. **No new UI:** `git diff` shows only deletions + comment/test updates. No new buttons, panels, or CSS classes added.
