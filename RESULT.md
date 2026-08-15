# RESULT — Remove walk-in UI and add Intake button with standardized labels

## What was built
Merged the two child story branches for `epic/26-replace-add-walkin-with-intake-button-st`:
- `wt/t_9ce22e69` removed the "Add walk-in" UI, handlers, routes, CSS, and tests from the attendance matrix.
- `wt/t_adfb1011` added an "Intake" button to the attendance matrix topbar and standardized nav button labels across screens.

## Changes
- `r3-intake/internal/assets/public/index.html`
  - Removed the `{{if not .EventRequired}}` walkin-panel block (Add walk-in toggle, search, results, create form) and the `{{define "walkin-results"}}` block.
  - Added `<a href="/public/intake" class="btn btn-primary">Intake</a>` in the matrix topbar immediately after the Records button.
  - Standardized `/public/intake` nav buttons to label "Intake" and `/` nav buttons to label "Records" across the intake form, records list, admin, notes, note-history, and person-attendance screens.
- `r3-intake/internal/assets/public/app.css` — removed the `.walkin-*` rules.
- `r3-intake/internal/server/attendance.go` — removed `walkinResult`, `handleWalkinSearch`, `handleWalkin`, and updated 2 comments.
- `r3-intake/internal/server/server.go` — removed `/attendance/walkin-search` and `/attendance/walkin` routes.
- `r3-intake/internal/server/attendance_toggle_integration_test.go` — removed `TestWalkinRequiresEvent`, `TestWalkinStoresNoSite`, `doWalkin`, and the unused `time` import.
- `r3-intake/internal/server/attendance_test.go` — removed trivial walk-in forbid assertions; updated 2 doc comments.
- `r3-intake/internal/server/attendance_matrix_default_integration_test.go` — removed walk-in assertions; updated 2 doc comments.
- Added both child plans under `docs/plans/`:
  - `omp-plan-remove-walkin-ui.md`
  - `omp-plan-intake-button-standardize-labels.md`
- Added working plan `WORKING_PLAN_remove_walkin_ui.md`.

## Preserved
- The `walk_in` **STATUS** concept: `statusLabel` case, `WalkInCount`, export check-in counting, person_attendance status options, and all walk_in status tests.
- `EventRequired` field and person-attendance calendar reads.
- `event_id` fallback in `parseMatrixFilters` (toggle forms still POST it).

## Verification
- `cd r3-intake && go build ./...` — PASS
- `cd r3-intake && go vet ./...` — PASS
- `cd r3-intake && go test ./...` — PASS
- Grep checks:
  - `grep -F 'Add walk-in' r3-intake/internal/assets/public/index.html` — zero hits
  - `grep -F 'walkin-panel' r3-intake/internal/assets/public/index.html r3-intake/internal/assets/public/app.css` — zero hits
  - `grep -F '>Intake<' r3-intake/internal/assets/public/index.html` — hits on matrix, records list, and intake form topbars
  - `grep -F '>Records<' r3-intake/internal/assets/public/index.html` — hits on `/` nav buttons
