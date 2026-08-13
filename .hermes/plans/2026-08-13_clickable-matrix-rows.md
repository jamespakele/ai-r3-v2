# Working Plan: Clickable Participant Rows with Navigation and Hover Cue

## Objective
Make each participant row in the attendance matrix (GET /attendance, matrix-content template) navigate to that participant's record page at GET /intake/{id} when clicked, and add a clear visual hover cue so users know the row is interactive — without breaking the per-cell attendance toggle forms.

## Constraints
- Go server-rendered templates — the link must be rendered in the Go template (index.html), not injected by JS, because the matrix is re-rendered via hx-get on filter change.
- HTMX + Alpine.js — the toggle cells are form hx-post=/attendance/toggle; the row navigation must not interfere with HTMX form submission.
- Vanilla CSS — no framework; add plain CSS rules to app.css.
- Design system — Public Sans (14px body), accent #b5502e, card #fffdfa, page bg #f7f1e6, 14px card radii, 8px input radii.
- Existing row states must be preserved — .row-dropout (bg #fbeeec, name #8f3a2e) and .row-no-location (bg #fbf6ee) backgrounds must remain distinguishable on hover.
- Sticky first column — .matrix-name-col is position:sticky; left:0; z-index:2; background:#fffdfa; hover styling must cover the sticky cell too (no visible seam).

## File Structure
Files to modify (no new files, no Go changes required — MatrixRow.IntakeID and the GET /intake/{id} route already exist):

| File | Change |
|------|--------|
| r3-intake/internal/assets/public/index.html | In the matrix-content block (~line 921), wrap the name span in an a href=/intake/{{.IntakeID}}. |
| r3-intake/internal/assets/public/app.css | Add link styling + row hover rules (after the .matrix-name rules, ~line 317). |
| r3-intake/internal/assets/public/app.js | (Optional) Add a row-level click handler with a guard, if the whole-row click is desired beyond the name link. |

## Implementation Notes

### 1. Clickable target: name link (primary) + optional guarded row click
Recommendation: wrap the name in a real a href=/intake/{{.IntakeID}} as the primary mechanism. This is the safest approach:
- It is a genuine link — works without JS, is keyboard-focusable, and is the natural row-identity affordance.
- It lives only in the name cell, so it structurally cannot interfere with the toggle form/.matrix-dot buttons in the other cells.
- It is server-rendered, so it survives HTMX re-renders automatically.

Optional enhancement (whole-row click): if the product wants the entire row clickable, add a delegated click handler on the tbody (or each tr) that navigates to /intake/{{.IntakeID}} only when the click does not originate from an interactive element. The guard must ignore clicks whose event.target is inside .matrix-dot, form, a, button, input, select, or label — e.g. if (e.target.closest('.matrix-dot, form, a, button, input, select, label')) return;. This prevents the toggle dots and the name link from double-firing. Because the name link already covers the primary affordance, the row-level handler is a progressive enhancement and can be omitted if risk is a concern.

### 2. Hover cue (CSS)
- Name link: .matrix-name-col a { color: inherit; text-decoration: none; } so it inherits the normal name color (#2b2320) and the dropout name color (#8f3a2e). On hover: color: #b5502e; text-decoration: underline; plus cursor: pointer. This gives an unambiguous, accessible cue.
- Row background: add a warm hover tint on the whole row. Because .matrix-table tr.row-dropout td / .row-no-location td have higher specificity than a plain tr:hover td, define explicit hover variants so the dropout/no-location backgrounds are respected (a slightly darker shade of each):
  - .matrix-table tbody tr:hover td { background: #f3ead9; } (normal rows)
  - .matrix-table tbody tr.row-dropout:hover td { background: #f6ddd9; }
  - .matrix-table tbody tr.row-no-location:hover td { background: #f5ecd8; }
  - .matrix-table tbody tr.row-dropout.row-no-location:hover td { background: #f6ddd9; }
- Sticky column: the hover td rules apply to .matrix-name-col too (it is a td), and their specificity exceeds the sticky cell's own background: #fffdfa, so the sticky cell will tint consistently with the rest of the row — no seam. Verify this visually.

### 3. Accessibility
- Use a real a href=/intake/{{.IntakeID}} so it is keyboard-focusable (Tab) and Enter activates navigation, with no JS required.
- Keep the existing aria-label on .matrix-dot buttons intact; the link does not need an aria-label since the visible name is its accessible name.
- Ensure the hover cue is not the only affordance — the link underline/color change plus cursor: pointer provides it, and the link is focusable (focus-visible styling can be added if not already present).

### 4. HTMX / re-render
- The link is authored directly in the Go template, so every hx-get re-render of #matrix-and-stats includes the correct href=/intake/{{.IntakeID}} per row. No JS injection needed.
- The toggle forms are untouched; the name link does not wrap or contain any form, so HTMX form submission and the row navigation are fully independent.

### Edge cases
- Empty/blank IntakeID: if a row could ever have an empty IntakeID, the href would be /intake/ which handleIntakeCmd treats as render blank form (id == ""). Confirm all matrix rows always carry a valid IntakeID (they do per attendance.go); if not, guard the link render with {{if .IntakeID}}.
- Double navigation: the optional row-level handler must not fire when the name link itself is clicked — the guard's a exclusion handles this.
- Dropout/no-location rows: hover must not wash out their distinct backgrounds — handled by the explicit hover variants above.

## Verification Criteria
1. Build/vet/test (from r3-intake/):
   - go build ./... — compiles clean.
   - go vet ./... — no issues.
   - go test ./... — existing tests pass (no Go changes, so this is a regression check).
2. Manual click-through (run the server, open /attendance):
   - Hover a normal row — row background tints and the name underlines/turns accent #b5502e; cursor is a pointer.
   - Hover a dropout row and a no-location row — their distinct backgrounds darken slightly but remain visually distinct from normal rows.
   - Click a participant's name — navigates to GET /intake/{id} and the record page renders.
   - Click a .matrix-dot toggle — attendance toggles via HTMX and does not navigate (guard works).
   - Keyboard: Tab to a name link — it is focusable; Enter navigates to the record.
   - Change a filter (event/site/date) — matrix re-renders via hx-get and the links persist with correct hrefs.
   - Scroll the matrix horizontally — the sticky name column's hover tint stays aligned with the row (no seam).
