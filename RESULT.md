# RESULT — Restore TestMatrixRosterEventIndependent assertions

## What was built
Restored `TestMatrixRosterEventIndependent` in
`r3-intake/internal/server/attendance_roster_integration_test.go` (renamed from
`TestMatrixRosterEventScoped`). The test now asserts the roster (participant
list) is **identical** with and without a selected event — an admin sees the
full intake roster `[iInSite1, iInSite2, iOtherSite, iAssignedCM]` in both
cases. All event-site-filtering assertions were removed. Attendance dots still
differ only for the attendee whose record belongs to a different event
(`withEvent(ev1)` → iInSite1=present, iInSite2=""; `noEvent` → both present).

## Deviation from plan (omp)
The plan's "out-of-site walk-in never rendered as a row" check was removed. It
was logically impossible: the shared `want` roster includes `iOtherSite`
(Carol) in both lists, so the check would fail on `noEvent` (and on `withEvent`
once the sibling's filter removal lands). Removing it is consistent with the
card's own AC ("remove assertions that expect event-site filtering").

## Verification
- `go build ./...` — ok
- `go vet ./internal/server/` — ok
- `go test ./internal/server/ -skip TestMatrixRosterEventIndependent` — ok (no collateral damage)
- `TestMatrixRosterEventScoped` no longer exists (grep: zero matches)
- `TestMatrixRosterEventIndependent` fails ONLY on the `withEvent` roster
  assertion (line 193) because the sibling card `t_e0d89c58` (removing the
  eventSite roster filter in `loadMatrixRows`) has not landed in this worktree.
  omp proved the test passes when that sibling's change is applied. This is the
  expected cross-worktree dependency — the parent epic task merges all child
  worktrees together, at which point the test passes.

## Artifacts
- `docs/plans/omp-plan-restore-testmatrixrosterindependent.md` — working plan
- `r3-intake/internal/server/attendance_roster_integration_test.go` — modified test
