# Working Plan

## Objective

Convert the intake records list (the landing screen at `/`) from a button-submitted GET form to live HTMX filtering. The list must re-render in place as the user interacts:

1. Selecting an **event** applies the filter immediately.
2. Selecting a **status** applies the filter immediately.
3. Typing a **name** applies the filter as they type, debounced ~300ms.

The existing Go handler `handleList` already reads `?q=`, `?status=`, `?event=` and renders the `list` template, so the server-side filtering logic is unchanged. The work is a template refactor (extract a swappable partial) plus a small handler branch to serve just that partial on HTMX requests, mirroring the existing attendance-matrix pattern.

## Constraints

- **Scope:** intake list only. Do **not** touch the attendance matrix (`matrix`/`matrix-content` templates, `attendance.go`) — that is a separate sibling task.
- **Minimal change:** reuse the existing `handleList` query parsing and `AdminView` population; only add an HTMX partial-render branch.
- **HTMX script:** the `list` template currently has **no** `<script src="/static/htmx.min.js">` tag (only `matrix`, `admin`, and one other template load it). The list template must add it.
- **CSRF:** the list template's CSRF script only injects `csrf_token` into `POST` forms. `hx-get` requests are GETs, so no CSRF concern. The `X-CSRF-Token` header is already attached globally via `htmx:configRequest`, which is harmless for GETs.
- **Clear link:** keep it pointing to `/` (full page reload) — it still works with live filtering and gives a clean reset.
- **Bulk-delete form** (admin only) is a separate `POST` form and must remain untouched; it lives inside the swappable region, so the partial must include it.
- **Design system:** Public Sans + Lora, accent `#b5502e`, card `#fffdfa`, page bg `#f7f1e6`, HST timezone. No new styling needed — reuse existing `.inline-form`, `.field-input`, `.admin-table`, `.admin-result-count`, `.bulk-bar` classes.
- **`hx-push-url`:** optional. The matrix uses it; for the list, pushing the query string to the URL is a nice-to-have so filters are shareable/bookmarkable. Keep it consistent with the matrix (`hx-push-url="true"`).

## File Structure

All changes are in two files under the repo root `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_9f9c1728`:

- `r3-intake/internal/assets/public/index.html` — template refactor (extract `list-content` partial, add HTMX script tag).
- `r3-intake/internal/server/admin.go` — add HTMX partial-render branch in `handleList`.

No new files. No changes to `attendance.go`, `mcp.go`, or any other handler.

## Implementation Notes

### 1. `index.html` — extract a `list-content` partial

Wrap the swappable region (form + result count + bulk bar + table) in a new partial `{{define "list-content"}}` with a wrapper `<div id="list-content">`. The `list` template keeps the full page shell (head, topbar, container heading) and calls the partial.

**New partial structure** (moved verbatim from the current `list` body, with the wrapper div added):

```html
{{define "list-content"}}
<div id="list-content">
  <form method="get" action="/" class="inline-form"
        hx-get="/" hx-target="#list-content" hx-swap="outerHTML"
        hx-trigger="change, keyup changed delay:300ms" hx-push-url="true">
    <input type="text" name="q" value="{{.Query}}" placeholder="Search name, email, phone" class="field-input">
    <select name="status" class="field-input">
      <option value="">All statuses</option>
      <option value="unassigned"{{if eq .StatusFilter "unassigned"}} selected{{end}}>Unassigned</option>
      <option value="claimed"{{if eq .StatusFilter "claimed"}} selected{{end}}>Claimed</option>
      <option value="completed"{{if eq .StatusFilter "completed"}} selected{{end}}>Completed</option>
    </select>
    <select name="event" class="field-input">
      <option value="">All Events</option>
      {{range .Events}}<option value="{{.ID}}"{{if eq $.EventFilter .ID}} selected{{end}}>{{.Name}}</option>{{end}}
    </select>
    {{if or .Query .StatusFilter .EventFilter}}<a href="/" class="btn btn-ghost">Clear</a>{{end}}
  </form>
  <p class="admin-result-count">Showing {{.Total}} record{{if ne .Total 1}}s{{end}}{{if .Query}} matching "{{.Query}}"{{end}}</p>
  {{if .IsAdmin}}<form id="bulk-delete-form" method="post" action="/admin/intake/bulk-delete"></form>
  <div class="bulk-bar"><button type="submit" form="bulk-delete-form" class="btn btn-tiny btn-danger" onclick="return confirm('Delete '+document.querySelectorAll('.bulk-check:checked').length+' selected records?')">Delete selected</button></div>{{end}}
  <table class="admin-table">
    ... (thead + tbody rows, unchanged) ...
  </table>
</div>
{{end}}
```

Key points:
- **Remove the `Filter` submit button** — it is no longer needed since `change` and `keyup` triggers fire automatically. Keep the **Clear** link (shown only when a filter is active).
- **`hx-trigger="change, keyup changed delay:300ms"`** — `change` fires immediately on select/event/status changes; `keyup changed` fires on typing with a 300ms debounce (the `changed` modifier avoids firing when the value didn't change, e.g. arrow-key navigation).
- **`hx-target="#list-content"` + `hx-swap="outerHTML"`** — the partial's wrapper div replaces itself in place, matching the matrix pattern.
- **`hx-push-url="true"`** — keeps the URL query string in sync so filters are shareable (consistent with the matrix).
- The **bulk-delete form** and **bulk-bar** stay inside the partial so they survive swaps. The CSRF script's `htmx:afterProcessNode` handler re-scans swapped-in nodes, so the bulk-delete `POST` form keeps its injected `csrf_token` after a swap.

**In the `list` template**, replace the inline form/count/bulk/table block with:

```html
{{template "list-content" .}}
```

**Add the HTMX script** to the `list` template's `<head>`, alongside the existing CSS link:

```html
<script src="/static/htmx.min.js" defer></script>
```

### 2. `admin.go` — serve the partial on HTMX requests

In `handleList`, after `view.Sites`/`view.Events` are populated and before the final render, branch on the HTMX header (mirroring `attendance.go` lines 139-143):

```go
view.Sites = must(s.loadSites(false))
view.Events = must(s.loadAllEvents())
if r.Header.Get("HX-Request") == "true" {
    _ = s.tpl.ExecuteTemplate(w, "list-content", view)
    return
}
_ = s.tpl.ExecuteTemplate(w, "list", view)
```

No other handler changes. The query parsing, filter building, and `AdminView` population are reused as-is.

### 3. Verification of template wiring

- `{{define "list-content"}}` must be defined in the same template set as `list` (it is, since both live in `index.html`). `ExecuteTemplate(w, "list-content", view)` will resolve it.
- The `list` template must still render the full page on a normal (non-HTMX) GET, including the topbar and heading.

## Verification Criteria

1. **Build passes:** `go build ./...` (or the project's build command) succeeds with no errors.
2. **Full page still renders:** a plain GET to `/` (no `HX-Request` header) returns the complete `list` page — topbar, heading, form, count, table — with no `Filter` button and the Clear link present only when a filter is active.
3. **HTMX partial renders:** a GET to `/?status=claimed` with header `HX-Request: true` returns only the `list-content` partial (the `<div id="list-content">` wrapper), not a full HTML document.
4. **Live event filter:** selecting an event in the dropdown updates the table in place (no page reload) and the URL reflects `?event=<id>`.
5. **Live status filter:** selecting a status updates the table in place and the URL reflects `?status=...`.
6. **Live name search:** typing in the name field updates the table after ~300ms debounce (no reload); typing fewer than 2 chars yields no `q` filter (matches handler's `len(query) >= 2` rule).
7. **Clear link:** clicking Clear returns to `/` and resets all filters.
8. **Admin bulk-delete still works:** on an admin session, the bulk-delete form and bar render after an HTMX swap, and the CSRF token is re-injected into the swapped-in `POST` form (verify via the `htmx:afterProcessNode` scan).
9. **Attendance matrix unaffected:** `/attendance` still renders and filters exactly as before (no changes to its templates or handler).
10. **No regressions:** the `admin` and other templates that already load `htmx.min.js` are untouched.
