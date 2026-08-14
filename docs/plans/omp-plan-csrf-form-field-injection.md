# Working Plan: Inject `csrf_token` Hidden Field into Plain POST Forms via JS

## Objective

Add a tiny, self-contained, non-HTMX JavaScript snippet to the R3 Intake Go templates that copies the `r3_csrf` cookie value into a hidden `<input name="csrf_token">` on every **plain** `<form method="post">` that does **not** already carry one. This lets the double-submit CSRF middleware's form-field fallback validate all plain HTML POST forms (Add Event, site set-default/toggle, add site, add/update user, bulk-delete/claim/complete/delete intake, notes add/edit/delete, walk-in create, intake finish/cancel, event status) without editing each template individually. HTMX (`hx-post`) forms are explicitly excluded because they are already covered by the existing `X-CSRF-Token` header injection. The server-rendered hidden field on the login form must not be duplicated.

## Constraints

- **Language:** Vanilla JavaScript (ES5-style, matching existing inline scripts). No build step, no modules.
- **Stack:** Go server + embedded PocketBase; server-rendered Go templates in a single embedded file `internal/assets/public/index.html` with multiple `{{define}}` blocks, each with its own `<head>` and inline `<script>` tags; HTMX + Alpine.js; vanilla CSS.
- **Single file to modify:** `internal/assets/public/index.html` only. **No Go files, no other assets** (sibling card `t_6607a21e` adds the regression test; parent epic merges all worktrees at the end).
- **Double-submit pattern (parent master, uncommitted — not present in this worktree):**
  - Cookie `r3_csrf` (`csrfCookieName`), non-HttpOnly, SameSite=Lax, Path `/`, readable by JS.
  - Accepted sources: `X-CSRF-Token` header (`csrfHeaderName`) **or** form field `csrf_token` (`csrfFormName`, `PostFormValue` -> `FormValue` fallback). Compared with `hmac.Equal`; mismatch -> 403.
  - Existing HTMX helper (in each template head, uncommitted on parent): IIFE with `getCookie()` and a `document.addEventListener('htmx:configRequest', ...)` that sets `e.detail.headers['X-CSRF-Token']`.
- **Snippet must be self-contained:** define its own cookie reader (or use `document.cookie` directly); do not depend on the HTMX helper.
- **Scope of injection:** only `method="post"` forms **without** an `hx-post` attribute; skip forms already containing an `input[name="csrf_token"]`.
- **No-JS fallback preserved:** login form already renders `{{if .CsrfToken}}<input type="hidden" name="csrf_token" value="{{.CsrfToken}}">{{end}}` server-side; JS must not duplicate it.

## File Structure

Single file modified: `r3-intake/internal/assets/public/index.html`.

**Template defines whose `<head>` gets the snippet** (those that render plain POST forms). Insert the snippet immediately after the existing CSRF/HTMX helper block (or near the other head scripts) in each:

| Define | Line of `<head>` | Plain POST forms rendered |
|---|---|---|
| `page` | 3 | finish-form (`/intake/.../finish`, L56), cancel-form (`/intake/.../cancel`, L61) |
| `login` | 396 | login-form (`/login`, L415) — already has server-side hidden field |
| `list` | 436 | bulk-delete (L481), claim (L498), complete (L499), delete (L500) |
| `admin` | 514 | site default (L561), toggle (L564), add site (L574), user update (L598), add user (L614), add event (L650) |
| `notes` | 699 | note add/edit (`/notes/...`, L728), note delete (L760) |
| `matrix` | 834 | walk-in create (`/attendance/walkin`, L912) |
| `event-manage` | 1007 | event status (`/admin/events/.../status`, L1051, L1055) |
| `person-attendance` | 1176 | attendance/day POSTs (in the `person-attendance-day` fragment L1266, L1293, L1302 — handled dynamically) |

**Optional but recommended for consistency** (render no plain POST forms today, so not strictly required): `note-history` (775), `event-report` (1141). Adding the snippet there is harmless and future-proofs them; the implementer may add or omit. `list`, `admin`, `notes`, `matrix`, `event-manage`, `person-attendance` are required.

**Fragment defines** (`site-fragment`, `dup-fragment`, `section-01..05`, `matrix-content`, `walkin-results`, `event-roster`, `enroll-search-results`, `person-attendance-calendar`, `person-attendance-day`, `stat-cards`, `matrix-cell`) have **no** `<head>` — they are swapped into a full page via HTMX. They are covered by the parent page's initial scan **and** by the dynamic handler described below (e.g. `walkin-result-form` L994, attendance/day L1266/1293/1302).

## Implementation Notes

### Reference snippet (place in each head, self-contained)

```html
<script>
(function () {
  function getCookie(n) { var v = document.cookie.match('(?:^|; )' + n + '=([^;]*)'); return v ? v[1] : ''; }
  function inject(form) {
    if (!form || form.tagName !== 'FORM') return;
    if ((form.method || 'get').toLowerCase() !== 'post') return;           // only plain POST
    if (form.hasAttribute('hx-post')) return;                              // HTMX already covered by header
    if (form.querySelector('input[name="csrf_token"]')) return;            // already present (login form)
    var t = getCookie('r3_csrf');
    if (!t) return;                                                        // no cookie -> nothing to inject
    var h = document.createElement('input');
    h.type = 'hidden'; h.name = 'csrf_token'; h.value = t;
    form.appendChild(h);
  }
  function scan(container) {
    var roots = container ? [container] : document.querySelectorAll('form');
    for (var i = 0; i < roots.length; i++) inject(roots[i]);
    if (container) {
      var fs = container.querySelectorAll ? container.querySelectorAll('form') : [];
      for (var j = 0; j < fs.length; j++) inject(fs[j]);
    }
  }
  // Run once the DOM is parsed (head script executes before body exists).
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () { scan(document); });
  } else {
    scan(document);
  }
  // Handle forms added dynamically by HTMX swaps (walkin-results, attendance/day, etc.).
  document.addEventListener('htmx:afterProcessNode', function (e) {
    if (e.detail && e.detail.el) scan(e.detail.el);
  });
})();
</script>
```

### Design decisions

- **DOMContentLoaded vs immediate inline:** The snippet lives in `<head>`, which executes **before** the `<body>` is parsed, so no forms exist yet. It therefore waits for `DOMContentLoaded` (with a `readyState` guard to also work if ever moved to end-of-body). It does **not** run immediately.
- **Selecting plain POST forms:** `document.querySelectorAll('form')` then filter — `method` lowercased must equal `post` (forms without an explicit `method` default to `get` and are skipped). Exclude any form with an `hx-post` attribute. This is exact and avoids a selector-only approach (a CSS attribute selector cannot express "has method=post but not hx-post" cleanly).
- **Detecting existing `csrf_token`:** `form.querySelector('input[name="csrf_token"]')` — if found, skip. This prevents duplicating the login form's server-rendered field (which is conditionally present via `{{if .CsrfToken}}`) and any future server-side-injected field.
- **Cookie parse:** local `getCookie('r3_csrf')` reading `document.cookie` via a `(?:^|; )name=([^;]*)` match — identical technique to the existing HTMX helper, kept local so the snippet is standalone.
- **Idempotency:** `querySelector` check makes injection idempotent — re-scanning the same form never adds a second field. Safe to call repeatedly.
- **Dynamic forms (HTMX swaps):** Plain-POST forms inside swapped fragments (`walkin-results` -> `walkin-result-form` L994; `person-attendance-day` -> attendance/day L1266/1293/1302) do not exist at initial `DOMContentLoaded`. They are covered by an `htmx:afterProcessNode` listener that re-scans each newly-processed element and its descendant forms. `afterProcessNode` is chosen over `afterSwap` because it fires per-element with `e.detail.el`, letting the handler inject into just the swapped subtree (cheap and precise). This only applies on pages that load `htmx.min.js` (`page`, `matrix`, `person-attendance`); on pages without HTMX the listener simply never fires, which is correct since their forms are all present at initial scan.
- **No-JS fallback:** With JS disabled, no field is injected anywhere; the login form still works via its server-rendered `{{if .CsrfToken}}` field. Other plain POST forms would 403 without JS, but this is the accepted behavior — the story is explicitly a JS enhancement, and the parent's regression test exercises the JS path.
- **Coexistence with HTMX helper:** The snippet is fully independent of the existing `htmx:configRequest` helper. `hx-post` forms are skipped by this snippet, so HTMX continues to send the `X-CSRF-Token` header exclusively for them — no double submission, no redundancy.
- **Cookie availability on login page:** If `r3_csrf` is not yet set when login renders, `getCookie` returns empty and nothing is injected — harmless, because the login form already carries the server-side field. No change needed.

## Verification Criteria

**Manual browser test** (run server from this worktree after the parent's uncommitted CSRF middleware is in place; if middleware isn't present locally, verify DOM behavior only and defer end-to-end 403 checks to the merged tree):
1. **Login still works:** Open `/login`, submit — succeeds; inspect the form -> exactly **one** `csrf_token` input (the server-rendered one), no duplicate injected.
2. **Plain POST forms now pass:** Add Event (`/admin/events`), add site, set-default, toggle, add/update user, bulk-delete intakes, claim/complete/delete intake, note add/edit/delete, walk-in create, intake finish/cancel, event status — each submits successfully (200/redirect, no 403). Inspect each form in DevTools -> a hidden `input[name="csrf_token"]` was injected.
3. **HTMX forms still work and are not polluted:** attendance toggle, event enroll/unenroll, section autosaves, person-attendance day forms — still work via the `X-CSRF-Token` header; confirm no duplicate hidden field was injected into any `hx-post` form (they should remain clean).
4. **No duplicate fields anywhere:** On every page with plain POST forms, each such form has at most one `csrf_token` input.
5. **Dynamic forms:** Trigger an HTMX swap that inserts a fragment with a plain POST form (e.g. walk-in results, person-attendance-day) and confirm the injected hidden field appears on the newly-inserted form.
6. **No-JS:** Disable JS, load login -> server-rendered field present, login works. (Other forms will 403 by design.)
7. **Console clean:** No JS errors in DevTools console across the pages above.

**Go build/test:**
- `cd r3-intake && go build ./...` — must pass (touching only the embedded template does not affect Go compilation, but confirms the embed is valid).
- `go test ./...` — existing tests pass. (The new regression test is the **sibling** card `t_6607a21e` and is out of scope for this card.)
