# RESULT — Remove walk-in UI, handlers, routes, CSS, and tests

## What was built
Pure removal of the "Add walk-in" feature from the attendance matrix. No new UI added (sibling card t_adfb1011 owns the Intake button + label standardization).

## Changes (7 files, 367 deletions / 9 insertions — all insertions are comment rewrites)
- `r3-intake/internal/assets/public/index.html` — removed the `{{if not .EventRequired}}` walkin-panel block (Add walk-in toggle button, walkin-search panel, search input, walkin-results div, walkin-create form) and the `walkin-results` define block.
- `r3-intake/internal/assets/public/app.css` — removed 11 `.walkin-*` rules.
- `r3-intake/internal/server/attendance.go` — removed `walkinResult` struct, `handleWalkinSearch`, `handleWalkin`; updated 2 comments.
- `r3-intake/internal/server/server.go` — removed `/attendance/walkin-search` and `/attendance/walkin` routes.
- `r3-intake/internal/server/attendance_toggle_integration_test.go` — removed `TestWalkinRequiresEvent`, `doWalkin`, `TestWalkinStoresNoSite`; dropped now-unused `time` import.
- `r3-intake/internal/server/attendance_test.go` — removed trivially-true forbid entries; updated 2 doc comments.
- `r3-intake/internal/server/attendance_matrix_default_integration_test.go` — removed "Add walk-in"/"walkin-search" assertions; updated 2 doc comments.

## Preserved (correctly kept)
- `walk_in` STATUS concept: `statusLabel` case, `WalkInCount`, export check-in counting, person_attendance status options, and all walk_in status tests.
- `EventRequired` field + person-attendance calendar reads (index.html L1405/L1416).
- `event_id` fallback in `parseMatrixFilters` (toggle forms still POST it).
- Roster test comment "an out-of-site walk-in under ev1" (tests a walk_in STATUS record).

## Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server 16.4s, migrations 0.2s)
- Grep: `walkin` → zero matches (only walk_in STATUS); `Add walk-in` → zero matches
- Diff inspection: pure removal, no new buttons/panels/CSS/schema changes

## Artifacts
- Working plan: `docs/plans/omp-plan-remove-walkin-ui.md`
