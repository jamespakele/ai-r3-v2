# Working Plan: Attendance Dot Fix and "No Location" UI Grouping

## Objective

Fix the attendance matrix so participants **without a Location** (`intake.site`
is empty) can have their attendance dots toggled, and add a **"No Location"**
UX grouping at the top of the attendance list with clear visual separation and
an indicator.

Two deliverables:

1. **Dot fix (server):** `handleToggle` currently rejects the HTMX POST with
   HTTP 400 `missing required fields` when `site_id` is empty. No-Location
   participants send an empty `site_id` (the hidden input is populated from
   `rec.GetString("site")`, which is `""` for them), so their dots render
   clickable but the server rejects the toggle. Make the toggle work for
   no-Location participants by matching the already-working per-person
   day-detail path (`person_attendance.go` `handlePersonAttendanceDay`), which
   sets `rec.Set("site", intake.GetString("site"))` and does **not** require a
   non-empty site.

2. **"No Location" grouping (UI):** In the matrix, group participants with no
   site at the top under a "No Location" group header with clear visual
   separation, and add a small muted indicator. Because the dot fix makes
   attendance trackable WITHOUT a location, the indicator must reflect that
   attendance IS now trackable (the original "needs a location before
   attendance can be tracked" limitation is superseded by the fix).

## Constraints

- **Language:** Go (module `r3-intake`), server-rendered Go `html/template`.
- **Framework:** Embedded PocketBase v0.39 (Go API), HTMX + Alpine.js, vanilla
  CSS. No new dependencies.
- **Time:** All timestamps HST (`var hst = time.FixedZone("HST", -10*60*60)`).
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page
  bg `#f7f1e6`, 14px card radii, 8px input radii.
- **Schema:** `pocketbase/migrations/001_init.js` line 104 — `intake.site` is
  `required: false`, so no-Location is a legitimate state.
- **Filter escaping:** any user-supplied value interpolated into a PB filter
  must go through `mcpmod.EscapeFilter(s)` (import `mcpmod "r3-intake/internal/mcp"`).
- **CSS cache-buster:** bump the `?v=` on the stylesheet link for the matrix
  page whenever CSS changes.
- **No new migration required** — the fix only changes handler logic and
  template/CSS. `attendance.site` already accepts empty values.

## File Structure

All paths relative to the repo worktree root
`/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_9c4aa7e3`.

| File | Change |
|------|--------|
| `r3-intake/internal/server/attendance.go` | **Modify** — `handleToggle` site handling; `loadMatrixRows` row grouping; `MatrixRow`/`MatrixViewData` structs. |
| `r3-intake/internal/assets/public/index.html` | **Modify** — `matrix-content` template: "No Location" group header + row class + indicator; bump CSS `?v=` on the matrix page link (line ~819, currently `?v=4`). |
| `r3-intake/internal/assets/public/app.css` | **Modify** — add `.matrix-group-header`, `.row-no-location`, `.matrix-no-location-note` styles. |
| `r3-intake/internal/server/attendance_test.go` | **Modify** — extend `TestMatrixContentRender` fixture/assertions; add a `TestMatrixNoLocationGrouping` render test. |
| `r3-intake/internal/server/attendance_toggle_integration_test.go` | **Create** — in-process PB integration test proving a no-Location participant's dot toggle succeeds (mirror `attendance_export_integration_test.go` / `person_attendance_integration_test.go` harness). |

## Implementation Notes

### 1. Dot fix — `handleToggle` (attendance.go ~line 462)

- **Remove `siteID` from the required-fields check.** Keep `intakeID` and `date`
  required:
  ```go
  if intakeID == "" || date == "" {
      http.Error(w, "missing required fields", http.StatusBadRequest)
      return
  }
  ```
- **Resolve the effective site for the new record.** When `siteID` is empty,
  fall back to the intake's own site (which may still be empty). Load the intake
  record once and reuse it for both the case_manager auth check and the site
  fallback. Concretely:
  - After the auth block, if `siteID == ""`, fetch the intake record
    (`s.pb.FindRecordById(intakeCol.Id, intakeID)`) and set
    `siteID = intakeRec.GetString("site")`. This matches the day-detail path
    (`rec.Set("site", intake.GetString("site"))`).
  - **Edge case:** the intake fetch may fail (deleted intake). If the intake
    cannot be loaded and `siteID` is still empty, proceed with `siteID == ""`
    (create the record with empty site) rather than erroring — the toggle
    itself is still valid. Do not reintroduce a 400 for empty site.
- **On create** keep `rec.Set("site", siteID)` — now `siteID` is the resolved
  value (intake's site, possibly `""`). PB accepts empty relation values.
- **On update** no change needed (site is not updated on toggle).
- **HTMX fragment / 303 redirect:** unchanged. The returned `MatrixCell.SiteID`
  is the resolved `siteID`; for no-Location participants this is `""`, which the
  template already renders as an empty hidden input — correct.

### 2. "No Location" grouping — data model

- **Add `NoLocation bool` to `MatrixRow`** (attendance.go ~line 33). Set it in
  `loadMatrixRows` where `cellSiteID` is computed:
  ```go
  cellSiteID := siteID
  if cellSiteID == "" {
      cellSiteID = rec.GetString("site")
  }
  row.NoLocation = cellSiteID == ""
  ```
  This is the cleanest approach: it reuses the existing single-`Rows` slice and
  single-`{{range .Rows}}` loop, so `computeSummary(rows, len(dates))` and the
  stat cards keep working unchanged. Splitting into two slices would require
  reworking `computeSummary` and the template loop for no benefit.
- **Grouping order:** sort `rows` so no-Location rows come first. In
  `loadMatrixRows`, after building `rows`, do a stable partition: no-Location
  rows first, then located rows, preserving the existing `name` sort within
  each group. (Go's `sort.SliceStable` on a `NoLocation`-first comparator, or
  build two slices and concatenate.)

### 3. "No Location" grouping — template (`matrix-content`, index.html ~line 850)

- Inside the `<tbody>`, before the `{{range .Rows}}` loop, render a group
  header row that appears only when at least one no-Location row exists. Since
  the template cannot easily compute "any NoLocation" from a range, add a
  **`HasNoLocation bool` field to `MatrixViewData`** (set in `handleMatrix`
  from the rows) and gate the header on it:
  ```html
  {{if .HasNoLocation}}
  <tr class="matrix-group-header">
    <td class="matrix-name-col" colspan="{{len .Dates | add 2}}">
      <span class="matrix-group-title">No Location</span>
      <span class="matrix-no-location-note">Attendance is trackable without a location — assign one in Records to group here.</span>
    </td>
  </tr>
  {{end}}
  ```
  - `colspan` must cover the name column + one column per date + the Total
    column = `len .Dates + 2`. Add a small `add` template func if not already
    present in `templateFuncs()` (check `internal/server` for an existing
    `add`; if absent, add it to `templateFuncs()`).
- **Row class:** render the no-location class on the row:
  ```html
  <tr {{if .IsDropout}}class="row-dropout"{{end}} {{if .NoLocation}}class="row-no-location"{{end}}>
  ```
  Note: a row could be both dropout and no-location. Use a single `class`
  attribute built from both flags (e.g. `class="row-dropout row-no-location"`)
  rather than two `class` attributes (invalid HTML). Simplest: compute the
  class string in the template with `{{if .IsDropout}}row-dropout {{end}}{{if .NoLocation}}row-no-location{{end}}`.
- **Per-row indicator:** in the name cell, when `.NoLocation`, render a small
  muted note next to the name:
  ```html
  {{if .NoLocation}}<span class="matrix-no-location-note">no location</span>{{end}}
  ```

### 4. CSS (app.css)

- `.matrix-group-header td { background: #f7f1e6; font-weight: 600; color: #6b5f52; text-align: left; border-top: 2px solid #e4d9c8; }` — clear visual separation above the group.
- `.matrix-table tr.row-no-location td { background: #fbf6ee; }` — subtle distinct tint for no-location rows (keep it lighter than `row-dropout`'s `#fbeeec` so the two remain distinguishable).
- `.matrix-no-location-note { font-size: 12px; color: #8a7a68; font-weight: 400; margin-left: 6px; }`
- Bump the matrix page stylesheet link `?v=4` → `?v=5` (index.html line 819).

### 5. Edge cases

- **Case manager scoping:** `loadMatrixRows` for a case_manager filters intakes
  by `assigned_to`; a no-Location intake assigned to them still appears and
  gets `NoLocation=true`. The toggle auth check (own intakes only) is
  unaffected.
- **Admin "All locations" (`siteID == ""`):** every row's `cellSiteID` falls
  back to the intake's own site, so located rows keep their site and only truly
  no-Location rows get `NoLocation=true`. Correct.
- **Event filter selected:** rows come from enrolled participants + walk-ins;
  `cellSiteID` fallback still applies, so no-Location grouping works there too.
- **Empty matrix:** when `len(rows) == 0`, the `{{if .Rows}}` guard already
  shows the empty state; `HasNoLocation` will be false, so no stray header.
- **`add1` template func:** already exists in `templateFuncs()` (handlers.go). Use
  `{{len .Dates | add1 | add1}}` for the colspan. Do not hardcode a colspan number.

## Verification Criteria

1. **Unit — toggle validation:** a test (or code review) confirms `handleToggle`
   no longer 400s on empty `site_id`; `intakeID`/`date` are still required.
2. **Integration — no-Location toggle succeeds:** new in-process PB test
   (`attendance_toggle_integration_test.go`) seeds an intake with empty `site`,
   POSTs `/attendance/toggle` with `site_id=""`, and asserts HTTP 200 (HTMX) and
   that an `attendance` record was created with empty `site`. Mirror the
   `newTestServer(t)` harness from `attendance_export_integration_test.go`.
3. **Integration — located toggle unchanged:** same test seeds a located intake
   and asserts the record's `site` equals the intake's site.
4. **Render — grouping:** extend `TestMatrixContentRender` (or add
   `TestMatrixNoLocationGrouping`) with a fixture containing one no-Location
   row and one located row; assert the output contains `No Location`, the
   `row-no-location` class, the `matrix-group-header` row, and the
   `no location` per-row note. Also assert the no-Location row renders before
   the located row.
5. **Render — no regression:** existing `TestMatrixContentRender` still passes
   (template parses, `matrix` full page renders).
6. **Full suite:** `go test ./...` in `r3-intake/` passes.
7. **Manual smoke:** as admin with "All locations", a participant with no site
   appears under the "No Location" header, their dots are clickable and cycle
   status, and the stat cards still match the matrix.
