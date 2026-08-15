# RESULT — Add Intake button and standardize button labels across screens

## What was built
Template-only change to `r3-intake/internal/assets/public/index.html` (8 edits, 1 file).

**Change 2 — Intake button on attendance matrix:** added `<a href="/public/intake" class="btn btn-primary">Intake</a>` in the matrix topbar immediately after the Records button (L1152).

**Change 3 — Standardized button labels:**
- `/public/intake` buttons → "Intake": intake form topbar (L66, was "New Form"), records list topbar (L559, was "New Intake").
- `/` (records list) buttons → "Records": intake form topbar authed branch (L65, was "View All"), admin topbar (L678, was "Back to List"), notes topbar (L948, was "View All"), note-history topbar (L1060, was "View All"), person-attendance topbar (L1414, was "View All"). Matrix topbar Records (L1151) already correct.

## Excluded (per plan)
- L93 `link-btn-muted` "New Form" (semantically "start a new form", not nav-to-list)
- L496 login-hint prose link
- L591 "Clear" filter button
- L65 "Sign in" branch
- Walk-in panel/code (sibling t_9ce22e69's scope)

## Verification
- `go build ./... && go vet ./... && go test ./...` all PASS (server 16.5s, migrations 0.2s). Template-render tests exercise matrix, person-attendance, admin, notes, note-history blocks.
- Grep: every `href="/public/intake"` button reads `>Intake<`; every `href="/"` nav button reads `>Records<`; excluded links unchanged.
- No Go handler, route, CSS, or test files changed.

## Artifacts
- Plan: `docs/plans/omp-plan-intake-button-standardize-labels.md`
- Diff: 1 file, 8 insertions / 7 deletions
