# Epic 10: Attendance screen state persistence and realtime stats refresh

**Status:** COMPLETE — child story branches merged into `epic/10-attendance-screen-persist-event-selectio`.

## Stories implemented

- 10.1 Persist event selection on refresh — `wt/t_f7a72ce9` — the matrix filter form uses `hx-push-url="true"` so the browser URL reflects the selected event, site, from/to dates. Refresh restores the same matrix state.
- 10.2 Realtime stats refresh for attendance dots — `wt/t_95a3e1c7` — new `GET /attendance/stats` renders the `stat-cards` fragment; each matrix cell `<td>` triggers a stat refresh on `hx-on::after-request` after a toggle.

## Files changed

- `r3-intake/internal/assets/public/index.html` — matrix filter form `hx-push-url="true"`; matrix cell `<td>` `hx-on::after-request` stat refresh; `id="stat-cards"` on the stat-cards div.
- `r3-intake/internal/server/attendance.go` — `parseMatrixFilters` shared helper; new `handleStats` endpoint handler.
- `r3-intake/internal/server/server.go` — registered `GET /attendance/stats` behind `requireAuth`.
- `r3-intake/internal/server/attendance_stats_integration_test.go` — `TestStatsEndpoint` (auth gate, event-scoped counts, no-event render).
- `docs/plans/omp-plan-persist-event-selection-hx-push-url.md` — working plan for 10.1.
- `docs/plans/omp-plan-realtime-stats-refresh.md` — working plan for 10.2.
- `WORKING_PLAN_realtime_stats_refresh.md` — working plan for 10.2.

## Merge resolution notes

- `wt/t_f7a72ce9` and `wt/t_95a3e1c7` both touched `r3-intake/internal/assets/public/index.html` in non-overlapping regions, so the template merges cleanly as the superset of both changes.
- Both branches replaced `RESULT.md` with their own story-level summary; the file was synthesized into this Epic 10 document.

## Verification

- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass
- Conflict-marker sweep — none
