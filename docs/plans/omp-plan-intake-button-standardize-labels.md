# Working Plan: Add Intake button and standardize button labels across screens

## Objective
Add an **Intake** button to the attendance matrix topbar (next to Records) that opens a blank intake form at `/public/intake`, and standardize button labels across all screens so the same destination always has the same label:
- Every button linking to `/public/intake` is labeled **Intake**.
- Every button linking to `/` (the records list) is labeled **Records**.

This is a template-only change to `r3-intake/internal/assets/public/index.html`. No Go handler, route, CSS, or test changes are required (verified: no Go/test code references any of these button labels).

## Constraints
- Language/framework: Go server-rendered Go templates (single embedded `index.html` with `{{define}}` blocks), HTMX + Alpine.js, vanilla CSS.
- Design system: reuse existing `.btn`, `.btn-primary`, `.btn-ghost` classes. Intake buttons use `btn btn-primary`; nav-to-records buttons use `btn btn-ghost`.
- Do NOT touch the walk-in panel or any walk-in code — that is the sibling card t_9ce22e69's scope (already done in its own worktree, not merged here).
- Do NOT change the `link-btn-muted` "New Form" at line ~93 (semantically "start a new form", not in the enumerated list).
- Do NOT change the "Clear" filter button at line ~591 (filter action, not a nav button).
- Do NOT change the "Sign in" branch of the intake-form topbar conditional (line ~65) — only the "View All" branch that links to `/`.

## File Structure
- `r3-intake/internal/assets/public/index.html` — the ONLY file changed.

## Implementation Notes
### Change 2: Add Intake button to matrix topbar
In the `matrix` template topbar (line ~1151), add an Intake button next to the existing Records button:
```html
<a href="/" class="btn btn-ghost">Records</a>
<a href="/public/intake" class="btn btn-primary">Intake</a>
```
Place it immediately after the Records button, before the Admin conditional.

### Change 3: Standardize button labels
Buttons linking to `/public/intake` → label "Intake":
- Line ~66 (intake form topbar): `New Form` → `Intake`
- Line ~559 (records list topbar): `New Intake` → `Intake`

Buttons linking to `/` (records list) → label "Records":
- Line ~65 (intake form topbar, IsAuthed branch): `View All` → `Records`
- Line ~678 (admin topbar): `Back to List` → `Records`
- Line ~948 (notes topbar): `View All` → `Records`
- Line ~1060 (note-history topbar): `View All` → `Records`
- Line ~1413 (person-attendance topbar): `View All` → `Records`
- Line ~1151 (matrix topbar): `Records` — already correct, keep.

## Logical Consequences
Traced every button in the template that links to `/public/intake` or `/`:
- `/public/intake` links: L66 (New Form → Intake), L559 (New Intake → Intake). Both change.
- `/` links: L65 (View All → Records), L93 (link-btn-muted "New Form" — KEEP, semantically "start new form", not a nav-to-list button), L496 (login-hint prose link — KEEP, not a button), L591 (Clear filter — KEEP, filter action), L678 (Back to List → Records), L948 (View All → Records), L1060 (View All → Records), L1151 (Records — KEEP), L1413 (View All → Records).
- No Go handler, route, or test references any of these button labels (verified via grep) — no code changes needed.
- The new matrix Intake button uses `btn btn-primary` matching the other intake buttons for visual consistency.
- Purely presentational change: routing, auth, and CSRF are unaffected.

## Verification Criteria
1. `cd r3-intake && go build ./... && go vet ./... && go test ./...` all pass.
2. Grep confirms: every `href="/public/intake"` button reads `>Intake<`; every `href="/"` nav button reads `>Records<` (except the excluded L93/L496/L591 cases).
3. The matrix topbar renders an Intake button next to Records.
4. The intake form still works when reached via the new Intake button (route `/public/intake` unchanged).
