# Working Plan: Add realtime stats refresh for attendance dots

## Objective

On the Attendance screen, the summary stat cards (Total check-ins, Active
participants, Stopped, Avg attendance rate) are computed server-side in
`handleMatrix` and rendered once. When a user toggles an attendance dot via
HTMX, only the single `matrix-cell` fragment is swapped — the stat cards never
update.

Build a `GET /attendance/stats` endpoint that returns just the `stat-cards`
fragment for the current filters (event, site, from, to), and fire an HTMX
request after each dot toggle to refresh the stat-cards container.

**Acceptance criteria**
- Toggling an attendance dot updates the stat cards immediately.
- Stats reflect the selected event's data.

## Constraints

- **Language:** Go (server-rendered Go `html/template`).
- **Framework:** net/http mux in `Server.Mux()`; embedded PocketBase for data.
- **Frontend:** HTMX + Alpine.js, vanilla CSS. No JS framework, no build step.
- **Design system:** Public Sans + Lora, accent `#b5502e`.
- **Time:** All timestamps in HST (`hst` var already used in `handleMatrix`).
- **Auth:** All attendance routes are wrapped in `s.requireAuth`; the new stats
  route must be too.
- **No source code in this plan** — implementation notes only.

## File Structure

| File | Change |
|------|--------|
| `r3-intake/internal/server/attendance.go` | Add `handleStats` handler. Optionally extract shared filter-parsing into a helper reused by `handleMatrix` and `handleStats`. |
| `r3-intake/internal/server/server.go` | Register `GET /attendance/stats` route in `Mux()`. |
| `r3-intake/internal/assets/public/index.html` | Give the stat-cards div an `id="stat-cards"`; add an HTMX refresh trigger on the toggle cell form. |
| `r3-intake/internal/server/attendance_test.go` | Add a test for the `/attendance/stats` endpoint rendering the `stat-cards` fragment. |

## Implementation Notes

### 1. Route registration (`server.go`)

Add next to the existing attendance routes (auth-only, matching the matrix):

```go
mux.HandleFunc("GET /attendance/stats", s.requireAuth(s.handleStats))
```

### 2. New handler `handleStats` (`attendance.go`)

Reuse the exact filter-parsing and data-loading path from `handleMatrix` so the
stats always match the matrix:

- Parse `from`/`to` with the same defaults (`defFrom` = today−13, `defTo` =
  today), same swap-if-inverted logic, same 30-day cap.
- Parse `event` and resolve `site` via `s.resolveSite(u, ...)`.
- Build `dates := buildDateRange(from, to)`.
- `rows, err := s.loadMatrixRows(u, siteID, dates, eventID, to)`.
- `summary := computeSummary(rows, len(dates))`.
- Render **only** the `stat-cards` template with a view carrying `Summary`
  (plus whatever fields the template references — `Summary` is the only one
  used by `stat-cards`).

**Recommended refactor:** extract the from/to/event/site parsing block from
`handleMatrix` into a small helper (e.g. `parseMatrixFilters(r, u) (from, to,
eventID, siteID, siteName string, dates []string)`) and call it from both
`handleMatrix` and `handleStats`. This guarantees the two endpoints can never
drift on defaults, validation, or site resolution, and keeps the stats
endpoint from 500ing on the same inputs the matrix accepts.

**Rendering:** the `stat-cards` template is a standalone `{{define}}`, so it can
be executed directly:

```go
_ = s.tpl.ExecuteTemplate(w, "stat-cards", view)
```

No `HX-Request` branch is strictly needed (the endpoint is only ever called via
HTMX), but returning `text/html; charset=utf-8` is consistent with `handleToggle`.

### 3. Template changes (`index.html`)

**Give the stat-cards div an id** so HTMX can target it:

```html
{{define "stat-cards"}}
<div id="stat-cards" class="stat-cards">
  ...
</div>
{{end}}
```

**Fire a refresh after each dot toggle.** The toggle cell form already carries
hidden inputs for `site_id`, `from`, `to`, and `event_id`. Add an
`hx-on::after-request` attribute to the cell form that issues a GET to
`/attendance/stats` targeting `#stat-cards`:

```html
<form method="post" action="/attendance/toggle" class="matrix-cell-form"
      hx-post="/attendance/toggle" hx-target="closest form" hx-swap="outerHTML"
      hx-trigger="submit" hx-include="closest form"
      hx-on::after-request="htmx.ajax('GET', '/attendance/stats', {target:'#stat-cards', swap:'outerHTML', include:this})">
```

`htmx.ajax(..., {include: this})` serializes the form's hidden inputs
(`site_id`, `from`, `to`, `event_id`) as query params, so the stats request
carries exactly the current filters. `hx-on::after-request` fires after the
toggle's swap completes, so the refresh happens after the cell updates.

> **Alternative (if `hx-on::after-request` + `htmx.ajax` is undesirable):**
> have `handleToggle`'s returned `matrix-cell` fragment include an
> `hx-trigger="load"` element that GETs `/attendance/stats` into `#stat-cards`.
> The `htmx.ajax` approach is preferred because it keeps the refresh logic in
> the template and reuses the form's existing hidden inputs without duplicating
> them in the response fragment.

### 4. Edge cases

- **Auth:** route is wrapped in `s.requireAuth` — unauthenticated requests get
  the same redirect/401 as the matrix.
- **Filter validation:** by reusing the shared parse helper, `handleStats`
  handles empty `from`/`to` (defaults), inverted ranges, >30-day caps, empty
  `event`, and site resolution exactly like `handleMatrix` — it never 500s on
  inputs the matrix accepts.
- **No event selected (`EventRequired`):** the stat cards still render (they
  reflect the roster). Toggling is disabled in that state anyway, so no refresh
  fires — but the endpoint must still render `stat-cards` with an empty/zero
  summary rather than erroring.
- **Empty rows:** `computeSummary` already returns a zero `MatrixSummary` for
  empty rows; the endpoint renders zeroed cards, matching the matrix.
- **Filter carry-through:** the refresh must include the current
  `event`/`site`/`from`/`to`. Using `include: this` on the cell form guarantees
  the stats reflect the selected event's data (acceptance criterion #2).
- **No-JS fallback:** `handleToggle`'s 303 redirect path is unchanged; the
  refresh is purely an HTMX enhancement and does not affect no-JS behavior.

## Verification Criteria

- `go build ./... && go vet ./... && go test ./...` all pass.
- Existing tests still pass: `TestComputeSummary`, `TestMatrixContentRender`,
  `TestMatrixContentRenderEventRequired`, `TestRequireEventID`.
- **New test** in `attendance_test.go`: issue a `GET /attendance/stats` with
  `HX-Request: true` and the same filters as a matrix render; assert the
  response body contains the `stat-cards` fragment (e.g. `id="stat-cards"`,
  `Total check-ins`, `Avg attendance rate`) and that the numbers match
  `computeSummary` for the same rows. Also assert the route is auth-gated
  (unauthenticated request is rejected).
- **Manual check:** load the Attendance screen, select an event, toggle a dot,
  and confirm the stat cards update immediately without a full page reload and
  reflect the selected event's data.
