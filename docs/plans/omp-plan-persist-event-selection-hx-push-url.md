# Working Plan: Persist event selection on refresh with hx-push-url

## Objective

Make the attendance matrix screen survive a browser refresh with the selected
event (and site/date-range) intact. Today the matrix filter form issues
`hx-get="/attendance"` without `hx-push-url`, so the browser URL never reflects
the selected event. On refresh the server receives no `event` query param and
renders the empty "no event selected" state, even though the dropdown visually
keeps its selection via browser form-state restore.

The fix is a single client-side attribute: add `hx-push-url="true"` to the
matrix filter form. HTMX will then push the serialized form values
(`event`, `site`, `from`, `to`) into the URL on every `hx-get`. On refresh the
server already reads those query params and restores both the dropdown and the
grid — no Go handler changes are required.

## Constraints

- **Scope is Issue 1 only** (persist event selection on refresh). A SIBLING card
  owns Issue 2 (realtime stats refresh via a `/attendance/stats` endpoint).
  Do NOT add any stats endpoint, do NOT touch the stat-cards template, and do
  NOT modify the matrix-cell toggle form.
- **No Go handler changes.** `handleMatrix` in
  `r3-intake/internal/server/attendance.go` already reads `event`, `site`,
  `from`, `to` from `r.URL.Query()` and sets `view.EventID` /
  `view.EventRequired` (`eventID == ""`). The dropdown already restores via
  `{{if eq $.EventID .ID}}selected{{end}}`. The server side is complete.
- **Minimal diff.** The only file touched is
  `r3-intake/internal/assets/public/index.html`, and the only change is adding
  one attribute to the matrix filter form.
- **Stack:** Go server + embedded PocketBase, server-rendered Go templates,
  HTMX + Alpine.js, vanilla CSS. All timestamps HST. Design system: Public Sans
  + Lora, accent `#b5502e`.
- **No new dependencies.** `hx-push-url` is a built-in HTMX feature.

## File Structure

Only one file changes:

- `r3-intake/internal/assets/public/index.html` — the `{{define "matrix-content"}}`
  block, matrix filter form (currently ~line 871).

No new files. No Go files. No CSS. No migrations.

## Implementation Notes

The matrix filter form currently reads:

```html
<form method="get" action="/attendance" class="inline-form"
      hx-get="/attendance" hx-target="#matrix-and-stats" hx-swap="outerHTML"
      hx-trigger="change, submit">
```

Change it to add `hx-push-url="true"`:

```html
<form method="get" action="/attendance" class="inline-form"
      hx-get="/attendance" hx-target="#matrix-and-stats" hx-swap="outerHTML"
      hx-trigger="change, submit" hx-push-url="true">
```

Behavior after the change:

- On any `change` or `submit` of the form, HTMX issues the `hx-get` to
  `/attendance` and pushes the serialized form values into the URL, e.g.
  `/attendance?event=<ID>&site=<ID>&from=2026-08-01&to=2026-08-13`.
- The `hx-target="#matrix-and-stats"` + `hx-swap="outerHTML"` behavior is
  unchanged — the grid and stat cards still swap in place without a full reload.
- On a hard refresh (F5 / reload), the browser requests the pushed URL. The
  server's `handleMatrix` reads `event` from the query param, sets
  `view.EventID`, and renders the dropdown with the matching option `selected`
  plus the populated grid. The empty "Select an event to record attendance."
  state is not shown because `EventRequired` is false.
- `hx-push-url="true"` uses the form's serialized values (the same values sent
  in the request), so `event`, `site`, `from`, `to` all round-trip correctly.
  The non-admin branch (a `<span>` instead of a `site` select) simply omits
  `site` from the URL, which the server already tolerates via
  `resolveSite(u, "")`.

Notes / pitfalls:

- Do NOT add `hx-push-url` to the matrix-cell toggle form or any other form —
  that is out of scope and would change unrelated URL behavior.
- Do NOT add a `hx-replace-url` or any stats endpoint. Keep the change to the
  single attribute.
- The `method="get" action="/attendance"` fallback (non-JS) already produces
  the same query string, so the no-JS path is consistent with the pushed URL.

## Verification Criteria

1. **Attribute present:** `grep -n 'hx-push-url' r3-intake/internal/assets/public/index.html`
   shows `hx-push-url="true"` on the matrix filter form only (one occurrence).
2. **No Go changes:** `git diff --stat` on the worktree shows only
   `r3-intake/internal/assets/public/index.html` modified.
3. **Build passes:** `cd r3-intake && go build ./...` exits 0.

4. **Manual (browser):**
   - Load `/attendance`, select an event from the dropdown. The URL updates to
     `/attendance?event=<ID>&site=<ID>&from=...&to=...` (no full reload).
   - Press F5 / reload. The page reloads with the same event selected in the
     dropdown and the grid populated — NOT the "Select an event to record
     attendance." empty state.
   - Change the date range and Apply; the URL reflects the new `from`/`to` and
     survives a refresh.
   - As a non-admin user, the URL omits `site` and still restores correctly.
5. **Regression:** the matrix-cell toggle still works (grid swaps in place, no
   page reload, no URL change from toggling).
