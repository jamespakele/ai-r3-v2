# BLOCKED — final deploy/close-out requires an authenticated operator

## What is DONE (verified, on the epic branch)
Epic "Remove the Claim Feature Completely" is fully implemented + merged + verified:
- Merge: all 4 child stories merged into epic/remove-the-claim-feature-completely-sche-0910064449 (final source commit 47a805dffe092d627db097b07441e7f5a2908b59; close-out docs committed f29edea..
- Full gate (independently re-run): gofmt clean, go build RC=0, go vet RC=0, go test ./... all packages ok (internal/server 16.7s; migrations ok 0.27s,, incl new 017 down->seed->up round-trip migration test TestRemoveClaimMigration PASS), zero-hit grep for claimed|assigned_to across internal/ + deploy/ (migrations/ + docs/ excluded per epic.. migrations.go registers BOTH 015 + 017.

## The capability wall (why blocked)
The remaining card-body steps cannot run from this spawned worker shell:
1. Story  ́5 step2 — production deploy via the vps-deploy-go skill (ships binary + migrations together, snapshot live DB, restart, verify live: service active, 017 in _migrations, no claimed status, cross-user access, public-resume rule..
2. epic-close.sh — merge epic branch -> master, push origin, close the github issue, scoped worktree+branch cleanup.
 It needs $HOME=/home/pakele (unset here) + authenticated gh (HTTP 401 here.. It also resolves the kanban DB via $HOME.

## Requested action for the operator
From an interactive shell with HOME=/home/pakele + authenticated gh, run:
1. vps-deploy-go against r3.aipono.org (Story 5 step2; verify live claims above..
2. /srv/data/hermes/skills/git-issue-to-kanban/scripts/epic-close.sh --repo jamespakele/ai-r3-v2 --issue <N> --epic-branch epic/remove-the-claim-feature-completely-sche-0910064449 --workdir <repo-root>  (resolve <N> to the epic GitHub issue number.)

Known consequence (state in close-out,: claimed rows -> unassigned + created_by empty -> publicly resumable, like all anon intakes} — accepted, single consistent rule James asked for.
