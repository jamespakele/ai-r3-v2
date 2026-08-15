# RESULT — Convert intake list filter to live HTMX

## What was built
Converted the intake records list (landing screen at `/`) from a button-submitted
GET form to live HTMX filtering. The list now re-renders in place as the user
interacts — no Filter button needed.

## Changes (2 files)
- `r3-intake/internal/assets/public/index.html`
  - Added `<script src="/static/htmx.min.js" defer></script>` to the `list` template head.
  - Extracted the form + result count + bulk bar + table into a new
    `{{define "list-content"}}` partial wrapped in `<div id="list-content">`.
  - Form now uses `hx-get="/" hx-target="#list-content" hx-swap="outerHTML"
    hx-trigger="change, keyup changed delay:300ms" hx-push-url="true"`.
  - Removed the Filter submit button; kept the Clear link (shown only when a filter is active).
- `r3-intake/internal/server/admin.go`
  - `handleList` now branches on `HX-Request: true` to render only the `list-content`
    partial; non-HTMX GETs render the full `list` page as before.

## Behavior
- Selecting an event applies the filter immediately.
- Selecting a status applies the filter immediately.
- Typing a name applies the filter as they type (300ms debounce; min-2-char rule intact).
- URL query string stays in sync (`hx-push-url`) so filters are shareable.
- Bulk-delete form/bar survive HTMX swaps (CSRF re-injected via `htmx:afterProcessNode`).

## Verification
- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — full suite passes (internal/server ok, pocketbase/migrations ok)
- omp ran an httptest against the real Mux(): plain GET `/` returns full page with no
  Filter button; `HX-Request: true` returns only the `list-content` partial; `?q=`,
  `?status=`, `?event=` filters work; Clear link present only when filtered;
  bulk-delete form survives; `/attendance` unaffected.

## Scope
Intake list only. Attendance matrix untouched (separate sibling task t_ca71ac0b).
