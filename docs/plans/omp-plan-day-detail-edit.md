# Working Plan: Day-Level Detail View and Inline Attendance Edit UI (Story 4.3)

## Objective — what to build

Wire up the interactive layer for the per-person monthly attendance calendar so that:

1. Clicking any calendar day cell (`td.person-att-cell`) opens the day-detail fragment (`person-attendance-day`) showing the status dropdown, event name, recorded-by, check-in time, note textarea, and Save + Cancel buttons.
2. Changing status/note and clicking **Save** POSTs the update; the calendar re-renders with the new status and the detail view closes automatically.
3. Clicking a day with no record shows "No attendance recorded" plus a status dropdown + note field to create a new record.
4. **Delete** shows a `confirm("Delete this attendance record?")` prompt; confirming deletes and refreshes the calendar.

The backend (routes, view structs, handlers) and the template/CSS stubs already exist. **This card only adds the client-side wiring** — making day cells clickable and ensuring the modal auto-closes on save/delete. No Go files are modified.

## Constraints — language, framework, dependencies

- **Language:** Go (server) + Go `html/template` (server-rendered), vanilla JS via HTMX + Alpine.js (no build step, no framework).
- **HTMX** loaded via `<script src="/static/htmx.min.js" defer>`; **Alpine.js** also loaded (both already in the `person-attendance` page head).
- **CSS:** vanilla, in `r3-intake/internal/assets/public/app.css` under `/* Person attendance calendar */`. All needed classes already exist (`.person-att-cell` already has `cursor:pointer`). No new CSS required unless a small modal-container tweak is needed.
- **Templates:** single embedded file `r3-intake/internal/assets/public/index.html` with `{{define}}` blocks.
- **Do NOT modify** any Go file under `r3-intake/internal/server/` (backend is owned by sibling card `t_aefcfde6`). Only touch `index.html` (and optionally `app.css`).
- **Verification:** `go build ./...`, `go vet ./...`, and `go test ./...` must pass from `r3-intake/`. UI behavior is verified by build + manual browser check (deferred to post-merge review).

## File Structure — files to create/modify

| File | Action | Change |
|------|--------|--------|
| `r3-intake/internal/assets/public/index.html` | **Modify** | Add `hx-get` to day cells in `person-attendance-calendar`; add a detail container inside `#person-attendance-calendar`; (optional) add `hx-on`/`hx-swap-oob` for auto-close. |
| `r3-intake/internal/assets/public/app.css` | **Modify (optional)** | Only if a small rule is needed for the detail container (e.g. `.person-att-detail-slot`). Likely zero changes. |
| `r3-intake/internal/server/person_attendance.go` | **Do NOT touch** | Backend already complete. |

## Implementation Notes — key design decisions, edge cases

### Design decision: WHERE the day-detail fragment renders

**Recommendation: render the day-detail fragment INSIDE `#person-attendance-calendar`.**

Rationale — the day-detail fragment Save and Delete forms already target `#person-attendance-calendar` with `hx-swap="outerHTML"`. When the fragment lives inside that div, the successful save/delete response replaces the **entire** calendar div — including the day-detail fragment — so the modal **auto-closes with zero extra JS**. This is the least-JS option and satisfies acceptance criteria 2 and 4 for free.

The alternative (a separate container outside the calendar div) would require an explicit `hx-on::after-request` or Alpine `x-effect` to close the modal after every save/delete — more JS, more edge cases. Rejected.

### Concrete wiring changes in `index.html`

1. **Add a detail slot inside the calendar div** (after the `<table class="person-att-calendar">` and before the legend, or after the legend — placement is cosmetic). Give it a stable id:
   ```html
   <div id="person-att-detail-slot"></div>
   ```
   The day-detail fragment is loaded into this slot via `hx-target="#person-att-detail-slot" hx-swap="innerHTML"`.

2. **Make day cells clickable.** On each `<td class="person-att-cell ..." data-date="{{.Date}}">` add:
   ```html
   hx-get="/intake/{{$.IntakeID}}/attendance/day?date={{.Date}}"
   hx-target="#person-att-detail-slot"
   hx-swap="innerHTML"
   hx-trigger="click"
   ```
   - Use `{{$.IntakeID}}` (root context) because the cell is inside `{{range .Weeks}}`/`{{range .}}` where `.` is a `PersonDayCell`, not the page view. `PersonDayCell` has no `IntakeID` field, so the root `$` is required.
   - `data-date="{{.Date}}"` already exists on the cell; the `hx-get` URL uses `{{.Date}}` directly (the cell own date), so no JS date extraction is needed.
   - The GET route `handlePersonAttendanceDayGet` validates `?date=YYYY-MM-DD` and renders the `person-attendance-day` fragment — no backend change needed.

3. **Auto-close on save/delete** — handled implicitly by the `hx-swap="outerHTML"` on the fragment forms replacing `#person-attendance-calendar` (which contains the slot). No extra code.

4. **Cancel button** — already works: `onclick="this.closest(".person-att-day-detail").remove()"` removes the fragment from the slot. No change.

5. **Validation-error path (edge case):** `handlePersonAttendanceDaySave` re-renders the **day fragment** (not the calendar) with a 400 status on validation error. Because the fragment form targets `#person-attendance-calendar` with `outerHTML`, a 400 response would try to replace the calendar with a day-detail fragment — **wrong target**. Mitigation: add `hx-on::after-request` on the Save form to detect a 400 and re-target the response into the slot:
   ```html
   hx-on::after-request="if(event.detail.xhr.status===400){this.closest("form").setAttribute("hx-target","#person-att-detail-slot");this.closest("form").setAttribute("hx-swap","innerHTML")}"
   ```
   (Or, simpler and acceptable for this card: leave the existing behavior and note the 400 path as a known minor issue to confirm in manual review. Prefer the `hx-on` guard if it stays minimal.)

### Edge cases

- **Other-month cells:** `PersonDayCell.IsOtherMonth` marks leading/trailing blank cells. These still have a valid `Date` and a working GET route, so clicking them is harmless (opens that date detail). Optionally skip wiring on `is-other-month` cells for UX, but not required.
- **Rapid double-click:** HTMX dedupes in-flight requests for the same element by default; no extra handling needed.
- **Month navigation:** Prev/Next links are full page loads (`?month=YYYY-MM`), so the slot is naturally cleared on navigation. No stale-detail risk.
- **`hx-trigger="click"` vs existing patterns:** The codebase uses `hx-trigger="change delay:300ms, submit"` for search inputs; for a click-to-open we use plain `click`. Consistent with the `hx-get` + `hx-target` + `hx-swap` pattern already used elsewhere (e.g. line 853 `hx-get="/attendance" hx-target="#matrix-and-stats" hx-swap="outerHTML"`).
- **Accessibility:** cells are `<td>` (not focusable). Optional enhancement: add `tabindex="0"` + `hx-trigger="click, keyup[key==Enter]"` so keyboard users can open the detail. Nice-to-have; not required by acceptance criteria.

## Verification Criteria — how to test correctness

1. **Build/vet/test (required):** from `r3-intake/` run `go build ./...`, then `go vet ./...`, then `go test ./...`. All must pass. This confirms the template edits parse and the embedded template compiles (Go validates `{{define}}` blocks at build/embed time).
2. **Template render sanity:** confirm `index.html` still parses by running the existing `person_attendance_test.go` / `person_attendance_integration_test.go` (they exercise the calendar/day templates).
3. **Manual browser check (deferred to post-merge review):**
   - Click a day with a record → detail opens with status dropdown, event/recorded-by/check-in (if present), note, Save + Cancel.
   - Change status/note, Save → calendar re-renders with updated status color; detail closes.
   - Click a day with no record → "No attendance recorded" + status/note form; Save creates a record and refreshes calendar.
   - Delete → confirm prompt appears; confirming deletes and refreshes calendar; Cancel on the prompt does nothing.
   - Cancel button in the detail closes it without a request.
   - Month Prev/Next still navigate correctly and clear the detail slot.
4. **No backend drift:** confirm no changes to `r3-intake/internal/server/person_attendance.go` or `server.go` in this card diff.
