# RESULT — Epic 31: Replace intake form autosave with validated Save button

## Goal

Remove autosave from the R3 intake form, add an explicit Save button per
section, split the participant Name field into First/Last, and gate section-01
saves on First + Last Name being non-empty. Client-side validation blocks the
submit and shows inline errors; server-side validation returns a 400 JSON error
body that the frontend renders inline. No record is created when validation
fails.

## What was built

### Frontend (t_0c5c0ad3)

All changes are confined to `r3-intake/internal/assets/public/index.html`.

1. **Autosave removed** — all five section forms' `hx-trigger="change delay:300ms, submit"`
   → `hx-trigger="submit"`. No section autosaves anymore.
2. **Save buttons visible** — dropped `noscript-only` from all five
   `section-save-btn`; they stay `type="submit"` and serve both the htmx and
   no-JS paths.
3. **Name split** — single `name="name"` input → `first_name`/`last_name`
   field groups (`#fn-group`/`#ln-group`, `#fn-error`/`#ln-error`); inputs
   start empty. Authed duplicate search moved to the `last_name` input with
   `hx-vals` sending the combined first+last as the existing `?name=` param.
4. **Prefill** — `data-fullname="{{.Name}}"` on the form;
   `R3F.splitNameIntoFields` splits at first space on `DOMContentLoaded` /
   `afterProcessNode`, never clobbering typed values.
5. **Client-side validation** — `R3F.validateSection01` + `htmx:beforeRequest`
   blocks the section-01 POST when either name is empty, shows inline errors,
   focuses the first empty field.
6. **Server-side 400 inline** — `htmx:beforeSwap` parses the
   `{"errors":{"first_name":..,"last_name":..}}` JSON contract from
   `/section/01` and populates the inline error divs.
7. **Copy fixed** — topbar "Saves automatically" and intro "everything saves
   automatically" updated to reference the Save button.

### Backend (t_67747531)

Changes in `r3-intake/internal/server/handlers.go` and a new test file.

- `handleSection`: added a validation gate for section 01 that runs BEFORE
  `applySection`/`saveIntake`, so a rejected save never mutates the record and
  never calls `pb.Save` (no record created).
- `applySection` case "01": joins submitted `first_name` + `last_name` into the
  single `name` column (`first + " " + last`), falling back to the legacy
  single `name` field when neither first/last is submitted (no-JS / older client).
- New `validateNameFields(r)`: requires first/last when those keys are submitted;
  falls back to legacy `name` when both keys are absent; exact messages
  `First name is required` / `Last name is required`.
- New `writeValidationErrors(w, errs)`: writes `application/json; charset=utf-8`,
  status 400, body `{"errors":{...}}`.
- `r3-intake/internal/server/intake_save_validation_test.go` (new): 6 integration
  tests using the `newTestServer` harness + CSRF helpers.

## Design decision

The intake schema has a SINGLE `name` column (max 200), not separate
first_name/last_name columns. The card's error contract uses `first_name`/`last_name`
keys. Resolution: the backend validates the submitted `first_name`/`last_name` form
fields and joins them into the single `name` column on save. No schema migration.
The frontend submits `first_name`/`last_name` and renders the JSON error body inline.

## Bugs found & fixed during verification

- `form.id` shadowed by the hidden `input[name=id]` (Chromium named-control
  property): guards now use `getAttribute('id')`.
- `hx-vals='js:(...)'` outer parens caused a JS `SyntaxError` at request time
  (htmx requires the `js:` payload to start with `{`): fixed to `js:{...}`.

## Files changed

- `r3-intake/internal/assets/public/index.html` — frontend: autosave removal,
  visible Save buttons, First/Last split, client-side validation, inline error
  rendering.
- `r3-intake/internal/server/handlers.go` — backend: section-01 validation gate,
  first/last → `name` join, JSON error writer.
- `r3-intake/internal/server/intake_save_validation_test.go` — new backend
  integration tests.
- `docs/plans/omp-plan-replace-intake-autosave-with-save-button.md` — frontend plan.
- `docs/plans/omp-plan-intake-save-validation.md` — backend plan.

## Verification

- `cd r3-intake && go build ./...` — PASS
- `cd r3-intake && go vet ./...` — PASS
- `cd r3-intake && go test ./...` — PASS
- No conflict markers: `grep -rE '^[<>=]{7}' r3-intake/ RESULT.md docs/plans/` → no output.
- `git diff --check` — clean.

## Acceptance criteria

1. No autosave occurs — VERIFIED
2. Save button present — VERIFIED
3. Missing names → inline errors, no submit — VERIFIED
4. Names filled → save succeeds — VERIFIED
5. Server-side validation errors displayed inline — VERIFIED
6. POST with missing first/last returns 400 + field errors, no record created — VERIFIED
7. POST with both fields populated saves successfully — VERIFIED
8. Error response format matches `{"errors":{"field":"message"}}` — VERIFIED

## Plan artifacts

- `docs/plans/omp-plan-replace-intake-autosave-with-save-button.md`
- `docs/plans/omp-plan-intake-save-validation.md`
