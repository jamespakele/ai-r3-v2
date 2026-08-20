# RESULT — Replace intake form autosave with validated Save button (t_0c5c0ad3)

## Goal

Frontend story: remove autosave from the R3 intake form, add an explicit Save
button, split the Name field into First/Last, and gate saves on First + Last
Name being non-empty (client-side) with server-side 400 errors rendered inline.
Template-only change (go:embed rebuild required).

## What was built

All changes are confined to `r3-intake/internal/assets/public/index.html`
(no Go server code touched — the sibling card t_67747531 owns handlers.go).

1. **Autosave removed** — all five section forms' `hx-trigger="change delay:300ms, submit"`
   → `hx-trigger="submit"`. No section autosaves anymore (AC1).
2. **Save buttons visible** — dropped `noscript-only` from all five
   `section-save-btn`; they stay `type="submit"` and serve both the htmx and
   no-JS paths (AC2).
3. **Name split** — single `name="name"` input → `first_name`/`last_name`
   field groups (`#fn-group`/`#ln-group`, `#fn-error`/`#ln-error`); inputs
   start empty. Authed duplicate search moved to the last_name input with
   `hx-vals` sending the combined first+last as the existing `?name=` param.
4. **Prefill** — `data-fullname="{{.Name}}"` on the form;
   `R3F.splitNameIntoFields` splits at first space on DOMContentLoaded /
   afterProcessNode, never clobbering typed values. (FormState has no
   FirstName/LastName fields, so inputs start empty and are prefilled by JS.)
5. **Client-side validation** — `R3F.validateSection01` + `htmx:beforeRequest`
   blocks the section-01 POST when either name is empty, shows inline errors,
   focuses the first empty field (AC3).
6. **Server-side 400 inline** — `htmx:beforeSwap` parses the
   `{"errors":{"first_name":..,"last_name":..}}` JSON contract from
   `/section/01` and populates the inline error divs (AC5).
7. **Copy fixed** — topbar "Saves automatically" and intro "everything saves
   automatically" updated to reference the Save button.

## Bugs found & fixed during omp verification

- `form.id` shadowed by the hidden `input[name=id]` (Chromium named-control
  property): guards now use `getAttribute('id')`.
- `hx-vals='js:(...)'` outer parens caused a JS `SyntaxError` at request time
  (htmx requires the `js:` payload to start with `{`): fixed to `js:{...}`.

## Verification (independent)

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server 18.1s, migrations 0.2s)
- No conflict markers; `grep 'change delay:300ms, submit'` → 0 matches;
  `grep 'hx-trigger="submit"'` → 6 (5 sections + finish form);
  `grep 'noscript-only'` → 0.
- omp's own live-server verification: no autosave (change+700ms → 0 requests),
  missing names blocked (XHR opened but nothing sent), filled names → 202 +
  HX-Redirect + persisted, server 400 renders inline, no-JS native submit works,
  duplicate search returns combined-name matches.

## Acceptance criteria

1. No autosave occurs — VERIFIED (trigger removed, live check)
2. Save button present — VERIFIED (all 5 visible, type=submit)
3. Missing names → inline errors, no submit — VERIFIED
4. Names filled → save succeeds — VERIFIED
5. Server-side validation errors displayed inline — VERIFIED

## Plan artifact

`docs/plans/omp-plan-replace-intake-autosave-with-save-button.md`
(contains the 7-step second-order review / Logical Consequences table).
