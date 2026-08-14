# RESULT — Add realtime stats refresh for attendance dots

## What was built
Implemented Issue 2 of the parent epic (t_64014c5e): the summary stat cards
(Total check-ins, Active participants, Stopped, Avg attendance rate) now refresh
in realtime when an attendance dot is toggled.

- **New endpoint** `GET /attendance/stats` (auth-only, `requireAuth`) that
  renders only the `stat-cards` fragment for the current filters (event, site,
  from, to).
- **Shared filter parsing** — extracted `parseMatrixFilters(r, u)` from
  `handleMatrix` and reused it in `handleStats`, so the stats always match the
  matrix (same defaults, inversion, 30-day cap, event/site resolution).
- **Template** — the `stat-cards` div now has `id="stat-cards"`; each matrix
  cell `<td>` fires `htmx.ajax('GET', '/attendance/stats', {source: form,
  target: '#stat-cards', swap: 'outerHTML'})` on `hx-on::after-request`, so the
  form's hidden `site_id`/`from`/`to`/`event_id` are serialized as query params
  after each toggle.

## Files changed
- `r3-intake/internal/server/server.go` — registered `GET /attendance/stats`
- `r3-intake/internal/server/attendance.go` — added `handleStats` +
  `parseMatrixFilters` refactor
- `r3-intake/internal/assets/public/index.html` — `id="stat-cards"` + HTMX
  refresh trigger on cell `<td>`
- `r3-intake/internal/server/attendance_stats_integration_test.go` — new
  `TestStatsEndpoint`
- `docs/plans/omp-plan-realtime-stats-refresh.md` — working plan artifact

## Verification
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass (internal/server ok, 6.5s)
- New `TestStatsEndpoint` covers: unauthenticated → 303 /login; auth with event
  → 200 fragment with correct counts; unrelated event → 0 check-ins
  (event-scoped); no event → fragment still renders.

## Acceptance criteria
- [x] Toggling an attendance dot updates the stat cards immediately
- [x] Stats reflect the selected event's data
