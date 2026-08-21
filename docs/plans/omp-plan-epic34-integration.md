# Working Plan: Epic 34 integration — remove per-section Save buttons + equal-size Save/New

## Objective
Compose the two child story changes (t_2e1c5fdd, t_22fbf8d0) onto the epic branch
`epic/34-remove-persection-save-buttons-single-sa`. Both child branches are at
e553265 (their work is uncommitted in sibling worktrees). The composite result
must keep BOTH children's feature sets, build/vet/test clean, and be committed.

## Files to assemble (source worktrees)
- t_2e1c5fdd worktree: /srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_2e1c5fdd
- t_22fbf8d0 worktree: /srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_22fbf8d0
- Epic worktree (CWD): /srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_8c9e9c7f

## Composition (disjoint — no conflicts)
1. index.html → take t_2e1c5fdd's version wholesale. It is the superset:
   removes all 5 "Save section 0X" buttons, adds R3F.saveAll() (both top Save
   buttons call it), removes dead .section-save-btn references, updates help text,
   and bumps ALL 9 cache-buster tags v8->v9 (identical to t_22fbf8d0's only
   index.html change). Copy file over.
2. app.css → combine BOTH:
   - t_2e1c5fdd: delete the `.section-save-btn` block (lines ~84-90).
   - t_22fbf8d0: replace
     `.finish-actions .btn-primary { padding: 13px 26px; font-size: 15px; }`
     with the combined selector:
     ```
     .finish-actions .btn-primary,
     .finish-actions .btn-ghost { box-sizing: border-box; padding: 13px 26px; font-size: 15px; line-height: 1; }
     ```
   These are disjoint lines. Apply both.
3. RESULT.md → write a composite epic-level RESULT.md summarizing both children
   (remove per-section save buttons + saveAll fix; equal-size Save/New via
   border-box combined selector) and the integration verification.
4. docs/plans/ → copy both child plan files into the epic worktree:
   - omp-plan-remove-per-section-save-buttons.md (from t_2e1c5fdd)
   - omp-plan-save-new-buttons-same-size.md (from t_22fbf8d0)
   - omp-plan-epic34-integration.md (this one)

## Verification (REQUIRED before commit)
- grep "Save section" -> 0 matches in index.html
- grep "section-save-btn" -> 0 matches in index.html AND app.css
- grep "R3F.saveAll" -> 2 matches in index.html
- grep "app.css?v=9" -> 9 matches; "app.css?v=8" -> 0
- .finish-actions has BOTH .btn-primary and .btn-ghost in the combined selector
- cd r3-intake && go build ./...  -> exit 0
- cd r3-intake && go vet ./...    -> exit 0
- cd r3-intake && go test ./...   -> PASS
- git diff --check clean

## Commit
Commit on the epic branch with message:
"feat(intake): remove per-section Save buttons (saveAll) + equal-size Save/New (epic/34)"
Include index.html, app.css, RESULT.md, docs/plans/omp-plan-remove-per-section-save-buttons.md,
docs/plans/omp-plan-save-new-buttons-same-size.md, docs/plans/omp-plan-epic34-integration.md.
