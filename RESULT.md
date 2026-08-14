# RESULT: Persist event selection on refresh with hx-push-url

## What was built
Added `hx-push-url="true"` to the attendance matrix filter form in
`r3-intake/internal/assets/public/index.html` (matrix-content block, line 874).
HTMX now pushes the serialized form values (`event`, `site`, `from`, `to`) into
the URL on every `hx-get`. On refresh the server's `handleMatrix` reads the
`event` query param and restores both the dropdown and the grid — no Go handler
changes were required (the server already restored from query params).

## Scope
Issue 1 only (persist event selection on refresh). The sibling card t_95a3e1c7
owns Issue 2 (realtime stats refresh) — no stats endpoint, no stat-cards or
matrix-cell changes were made.

## Files changed
- `r3-intake/internal/assets/public/index.html` — 1 insertion / 1 deletion
  (added `hx-push-url="true"` to the matrix filter form)
- `docs/plans/omp-plan-persist-event-selection-hx-push-url.md` — MOA working plan

## Verification
- `grep -n 'hx-push-url'` → exactly one occurrence, on the matrix filter form
- `git diff --stat` → only index.html modified (1 insertion / 1 deletion)
- `go build ./...` → exit 0
- `go vet ./...` → clean
- `go test ./...` → ok r3-intake/internal/server (6.360s), all packages pass

## Acceptance criteria
- [x] Selecting an event and refreshing the page restores the same event + grid
- [x] The URL reflects the selected event (hx-push-url)
- [ ] Toggling an attendance dot updates the stat cards immediately (Issue 2, sibling card)
- [ ] Stats reflect the selected event's data (Issue 2, sibling card)

## Commit
4e0b618 on wt/t_f7a72ce9
