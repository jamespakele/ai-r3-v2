# Working Plan: Walk-in Check-in & Dropout Visual Highlighting (Story 1.4)

## Objective

Add a "Add walk-in" flow to the attendance matrix so a case manager can check in a participant who was not enrolled in an event: search existing intake records by name (HTMX), select one or create a minimal name-only intake, then create a `walk_in`` attendance record for today (HST) at the resolved site. Also align the existing dropout highlighting to the exact Story 1.4 colors (``#fbeeec`` row background, `STOPPED` badge at 10px / `#8f3a2e` / `#fbeeec` / 4px radius). The matrix grid, filters, cell toggle, and basic dropout logic already exist and must NOT be redone.

## Constraints

- **Scope guard:** Do NOT redo the grid/toggle. Do NOT add CSV export, summary stats, per-person calendar, or events CRUd (other cards).
- **Stack:** Go server + embedded PocketBase v0.39 JS migration API (camelCase, no `app.dao()`), server-rendered Go templates, HTMX + Alpine.js, vanilla CSS. All timestamps in HST (`hst = time.FixedZone("HST", -10*60*60)` in `admin.go:16`).
- **Auth:** Both new routes wrapped in `s.requireAuth(...)`. Case managers are pinned to their resolved site; walk-in must be created for the resolved site, never a user-supplied site.
- **Design tokens:** accent `#"b5502e`, card `#fffdfa`, page bg `#f7f1e6`. Walk-in dot `#2a4d8a`. Dropout row `#fbeeec`, badge text `#8f3a2e`.
- **Idempotency:** A walk-in for an existing intake+date must not create a duplicate attendance record -- reuse the existing record (update to `walk_in`) or return it.
- **Dropout rule (AC 8):** Only participants with at least one `present` record whose last present date is >14 days before the view end are flagged. Participants with zero attendance records are NOT flagged. (Current `loadMatrixRows` already satisfies this via `row.LastPresent != "" && row.LastPresent < thresholdStr`.)
- **Name search:** min 2 chars (mirror `handleDuplicateSearch`); empty name rejected on create.

## File Structure

All changes are in the existing `r#3-intake` module. No new files required.

````
r3-intake/
  internal/server/
    attendance.go        # ADD handleWalkinSearch, handleWalkin ; (loadMatrixRows unchanged)
    server.go            # ADD 2 route registrations near lines 122-123
  internal/assets/public/
    index.html           # ADD walk-in UI to "matrix" define; ADD "walkin-results" define
    app.css              # ADD walk-in button/input/results styles; UPDATE dropout colors
````

## Implementation Notes

### 1. Routes (register in `server.go`, after line 123)

```go
mux.HandleFunc("/attendance/walkin-search", s.requireAuth(s.handleWalkinSearch))
mux.HandleFunc("/attendance/walkin", s.requireAuth(s.handleWalkin))
````

### 2. `handleWalkinSearch` (in `attendance.go`)

Mirror `handleDuplicateSearch` (handlers.go:758) exactly:

- `GET` only; `q := strings.TrimSpace(r.URL.Query().Get("name"))`.
- Set `Content-Type: text/html; charset=utf-8`; if `len(q) < 2` return empty body (no results).
- `col, _: = s.intakeCollection()`; `filter := fmt.Sprintf("name ~ \"%s\"", mcpmod.EscapeFilter(q))`.
- `recs, _ := s.pb.FindRecordsByFilter(col.Id, filter, "-created", 10, 0)`.
-  Build a `[]walkinResult{ID, Name}` slice (skip empty names → `"(unnamed)``), render `"walkin-results"` fragment.
- **Site scoping:** unlike duplicate search (cross-caseload), filter results to the resolved site so a case manager only sees their own site's intakes. Resolve via `siteID, _ := s.resolveSite(u, "")`; if `siteID != ""` append ` && site='<escaped>' ` to the filter. Admins with `siteID == ""` (All locations) see all.

### 3. `handleWalkin` (in `attendance.go`)

`POST` only. Flow:

1. `u := s.currentSession(r)`; `siteID, _ := s.resolveSite(u, "")`. If `siteID == ""` → `http.Error(w, "no site resolved", http.StatusBadRequest)` (a walk-in must belong to a concrete site).
2. `today := time.Now().In(hst).Format("2006-01-02")`.
3. **Resolve intake:**
   - If `intake_id` form value is non-empty: validate it exists via `s.pb.FindRecordById(intakeCol.Id, intakeID)`; 400/404 if missing. (Case-manager ownership check optional here -- walk-in is a site-level check-in, but keep the existing `assigned_to == u.ID` guard for consistency if desired.)
   - Else read `name` form value; if `strings.TrimSpace(name) == ""` → 400 "name required". Create minimal intake: `rec := core.NewRecord(intakeCol)`; set `name`, `site` = `siteID`, `created_by` = `u.ID`, `status` = `""unassigned�`; `s.pb.Save(rec)`; `intakeID = rec.Id`.
4. **Idempotent attendance record:** `attCol, _ := s.attendanceCollection()`; `filter := fmt.Sprintf("intake='%s' && date='%s'", mcpmod.EscapeFilter(intakeID), mcpmod.EscapeFilter(today))`; `recs, _ := s.pb.FindRecordsByFilter(attCol.Id, filter, "", 1, 0)`.
   - If a record exists: set `status = "walk_in"`, `recorded_by = u.ID`, `check_in_time` = now` (HST  `2006-01-02 15:04:05.000Z`), `site = siteID`; `s.pb.Save(existing)`.
   - Else create: `rec := core.NewRecord(attCol)`; set `intake`, `site = siteID`, `date = today`, `status = "walk_in"`, `recorded_by = u.ID`, `check_in_time` = now`; `s.pb.Save(rec)`.
5. **Response:** `http.Redirect(w, r, "/attendance", http.StatusSeeOther)` (303). Preserve filters by appending `?site=...&from=...&to=...&event=...` from the form's hidden fields, matching the no-JS fallback in `handleToggle`. A full-page redirect is simplest and guarantees the new row renders with the walk-in dot; no HTMX swap needed for the create path.

### 4. Template changes (`index.html`)

**`"+matrix"` define (line 770):*** Add an "Add walk-in" button + search panel above the matrix table (after the filter form, before `{{if .Rows}}`):

```html
<div class="walkin-panel">
  <button type="button" class="btn btn-primary" id="walkin-toggle"
          onclick="document.getElementById('walkin-search').classList.toggle('hidden')">Add walk-in</button>
  <div id="walkin-search" class="walkin-search hidden">
    <input type="text" name="walkin_q" class="field-input" placeholder="Search existing participant…"
           hx-get="/attendance/walkin-search" hx-trigger="keyup changed delay:500ms"
           hx-target="#walkin-results" hx-swap="innerHTML">
    <div id="walkin-results" class="walkin-results"></div>
    <form method="post" action="/attendance/walkin" class="walkin-create">
      <input type="hidden" name="site_id" value="{{.SiteID}}">
      <input type="hidden" name="from" value="{{.DateFrom}}">
      <input type="hidden" name="to" value="{{.DateTo}}">
      {{if .EventID}}<input type="hidden" name="event_id" value="{{.EventID}}">{{end}}
      <input type="text" name="name" class="field-input" placeholder="Or create new participant (name only)">
      <button type="submit" class="btn btn-primary">Check in walk-in</button>
    </form>
  </div>
</div>
````

**New `"walkin-results" define** (mirror `"dup-fragment"` at line 74):

```html
{{define "walkin-results"}}
{{range .}}<div class="walkin-result">
  <form method="post" action="/attendance/walkin" class="walkin-result-form">
    <input type="hidden" name="intake_id" value="{{.ID}}">
    <input type="hidden" name="site_id" value="{{.SiteID}}">
    <input type="hidden" name="from" value="{{.From}}">
    <input type="hidden" name="to" value="{{.To}}">
    {{if .EventID}}<input type="hidden" name="event_id" value="{{.EventID}}">{{end}}
    <button type="submit" class="walkin-result-btn">{{.Name}}</button>
  </form>
</div>{{end}}
{{end}}
````

Define a `walkinResult` struct in Go with `ID, Name, SiteID, From, To, EventID` so the fragment can carry the filter context.

**Dropout badge (line 830):** keep `{{if .IsDropout}}<span class="status-badge">STOPPED</span>{{end}}` -- only the CSS changes (below).

### 5. CSS (`app.css`)

**Update dropout colors (lines 281-283):**

```css
.matrix-table tr.row-dropout td { background: #fbeeec; }
.matrix-table tr.row-dropout .matrix-name { color: #8f3a2e; }
````

**Update `.status-badge` (line 201) to the exact AC spec **-- but note this class is shared with the intake form (line 492). Add a scoped override for the matrix badge instead of changing the shared rule:

```css
.matrix-table .status-badge {
  font: 600 10px 'Public Sans', sans-serif;
  color: #8f3a2e;
  background: #fbeeec;
  border-radius: 4px;
  padding: 3px 9px;
  margin-left: 6px;
}
````

**Add walk-in UI styles:**

```css
.walkin-panel { margin: 16px 0; }
.walkin-search { margin-top: 10px; padding: 12px; background: #fffdfa; border: 1px solid #e4d9c8; border-radius: 10px; }
.walkin-search.hidden { display: none; }
.walkin-search .field-input { width: 100%; margin-bottom: 8px; }
.walkin-results { margin-bottom: 8px; }
.walkin-result { margin-bottom: 4px; }
.walkin-result-form { margin: 0; }
.walkin-result-btn {
  display: block; width: 100%; text-align: left; padding: 8px 10px;
  background: #f7f1e6; border: 1px solid #e4d9c8; border-radius: 6px;
  font: 500 14px 'Public Sans', sans-serif; color: #2b2320; cursor: pointer;
}
.walkin-result-btn:hover { background: #efe6d4; border-color: #b5502e; }
.walkin-create { display: flex; gap: 8px; }
.walkin-create .field-input { flex: 1; margin-bottom: 0; }
````

### 6. Edge cases

- **Name search min 2 chars:** `handleWalkinSearch` returns empty body for `len(q) < 2`.
- **Empty name on create:** `handleWalkin` returns 400.
- **Duplicate walk-in for same intake+date:** reuse existing attendance record (update to `walk_in`) -- never create a second.
- **Case-manager site scoping:** walk-in always created for `resolveSite(u, "")` result; search results filtered to that site; admin with All locations sees all sites.
- **Today's date in HST:** `time.Now().In(hst).Format("2006-01-02")`,
- **No-JS fallback:** both new handlers degrade to 303 redirects; HTMX only enhances the search input.

## Verification Criteria

1. `cd r#3-intake && go build ./...` -- compiles clean.
2. `go vet ./...` -- no vet warnings.
3. `go test ./...` -- existing tests pass (no new tests required, but add one for `handleWalkin` idempotency if a test harness exists).
4. **Template parse check:** confirm `index.html` parses with the new `"walkin-results"` define and the modified `"+matrix"` define -- run the server and GET `/attendance` (auth'd) to confirm no `template: ... undefined` errors; the page renders the "Add walk-in" button.
5. **Manual flow (HTMX):** type ≠2 chars in the walk-in search → `#walkin-results` populates from `/attendance/walkin-search`; clicking a result POSTs `/attendance/walkin` with `intake_id` and 303-redirects to `/attendance` showing the row with a `dot-walk_in` (`#2a4d8aa) cell for today.
6. **Create flow:** submit the name-only form → a minimal intake (name+site+created_by+status=unassigned) is created, a `walk_in` attendance record for today is created, and the matrix shows the new row with the blue walk-in dot.
7. **Idempotency:** re-submitting the same walk-in does not create a second attendance record (verify in PB admin or via a count query).
8. **Dropout colors:** a participant whose last `present` is >14 days before view end renders with `row-dropout` → row bg `#fbeeec`, name `#8f3a2e`, `STOPPED` badge at 10px/`#8f3a2e`/`#fbeeec`/4px radius. A participant with zero attendance records is NOT flagged.
9. **Regression:** existing cell toggle (`/attendance/toggle`) and duplicate search (`/search/duplicates`) still work unchanged.
