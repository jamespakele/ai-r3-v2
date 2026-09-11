# MERGE VERIFICATION — Remove the Intake Status Feature Completely (Stories 1–4)

Branch: `epic/36-remove-intake-status-field-and-complete`
Pre-merge base of all children: `97519fc` (== master, plan doc)
Final epic HEAD after child merges: `7b8ae90` (close-out commit to follow)

## What this epic delivers (the actual change)
The `intake.status` select field, all stored status data, the List-screen Status column
and badges, the status filter dropdown, the per-row Complete button and its
`POST /admin/intake/{id}/complete` route+handler, and every status trace in the MCP API
are removed entirely. After this epic a record is just a record — `created_by`
provenance and the notes audit trail remain the only state notions.
Known consequence (accepted, stated not relitigated): nothing marks a record complete
anymore; the List shows every record equally. That is exactly what James asked for.

## Merge sequence executed (preserve each child worktree first)
| Order | Branch | Story | Result |
|-------|--------|-------|--------|
| — | `wt/t_f902df13` committed `63254eb` first | Story 1 (migration 018) | Worktree preserved before merging |
| 1 | `wt/t_36fab24a` `1b67ef4` | Story 2+3 (server/UI/MCP removal) | `--no-ff`, clean |
| 2 | `wt/t_f3446de5` `f3c526b` | Story 4 test sweep + migration 018 integration | `--no-ff`, clean (contains Stories 1,2,3) |
| 3 | `wt/t_f902df13` `7b8ae90` | Story 1 (migration 018) | `RESULT.md` conflict only -> keep ours (integration); remaining 4 files byte-identical across both branches |

No source-file conflicts occurred. The only conflict was `RESULT.md` (a doc rewritten by
every branch); resolved by keeping the integration (Story-4/full-gate) version, matching
the prior epic's "keep incoming RESULT.md" precedent. No conflict markers remain
(`grep '<<<<<<<|=======|>>>>>>>'` -> none).

## Migration 018 (schema + data)
`r3-intake/pocketbase/migrations/018_remove_intake_status.go` registered at
`migrations.go:27` alongside 015/017. Idempotent: `up` removes `intake.status` (no data
rewrite — stored status values drop with the field); `down` re-adds it as
`["unassigned","completed"]`. The 017 round-trip test was reworked to
down018 -> down017 -> seed -> up017 -> up018 so it composes in the full chain.

## Full verification gate (run from `r3-intake/`)
Working dir: `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_b46ad11f/r3-intake`
Go env in worker shell: GOPATH=/tmp/gopath GOMODCACHE=/tmp/gopath/pkg/mod GOCACHE=/tmp/gocache
($HOME is unset in the ai-coder worker shell, so Go's default module root is unreachable).

- `go build ./...` : **rc=0**
- `go vet ./...`   : **rc=0**
- `go test ./...`  : **rc=0** — `ok r3-intake/internal/server (17.1s)`, `ok r3-intake/pocketbase/migrations`. Every package.
- Note: `cmd/` does not exist at repo root or under `r3-intake/` (stale Makefile leg);
  the `cmd` portion of the gate is vacuously satisfied, matching the prior epic.

## Final mechanical token gate (whole app tree incl. test files, excl. migrations + docs)
Patterns `adminComplete|StatusFilter|status-unassigned|status-completed|ByStatus|by_status|
CompletedThisMonth|unassigned` and the intake `.../complete` route over `internal/` (server,
mcp, assets, config, crypto) incl. `*_test.go`:
- **ZERO genuine hits.** The only `status-completed` matches are the preserved event
  feature `.event-status-completed` (CSS + events test) — unrelated, must remain.
- Intake `.../complete` route: zero.
- Preserved and verified intact: finish route (`POST /intake/{id}/finish`,
  `handleIntakeCmd`, `finish-form`), `events.status` + `.event-status-*` CSS, `attendance.status`
  (matrix / CSV export / person-attendance), shared `.status-badge`, `created_by`
  provenance + public-resume rule, `casemanagerName` + `ensureCaseManager`.
