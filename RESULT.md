# RESULT — Run attendance matrix tests and verify fix

## What was verified
Assembled the combined fix (sibling worktrees) into this worktree and ran the
full attendance matrix test suite:

- **attendance.go** (from sibling t_e0d89c58): removed the `eventSite`-based
  intake filter in `loadMatrixRows` so the roster is always the full
  site/role-scoped participant list, independent of the selected event. Event
  scoping now applies only to the attendance map.
- **attendance_roster_integration_test.go** (from sibling t_6b3eff3f):
  restored `TestMatrixRosterEventIndependent` (renamed from
  `TestMatrixRosterEventScoped`), asserting the roster is identical with/without
  a selected event (full admin roster [iInSite1, iInSite2, iOtherSite,
  iAssignedCM] in both cases), with attendance dots differing only for the
  attendee whose record belongs to a different event.

## Verification results
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (internal/server 14.8s ok; migrations ok)
- `go test ./internal/server/ -run TestMatrixRosterEventIndependent -v` — PASS

## Conclusion
The regression is fixed: selecting an event no longer filters the participant
list; the roster is always the full site/role-scoped list, and the event scopes
only the attendance map. All tests pass with the combined fix.

## Note
This is a child story card. The parent epic task (t_eefa9c57) merges the child
worktrees (t_e0d89c58, t_6b3eff3f, t_69f207ad) onto the epic branch, then to
master, and closes the GitHub issue.
