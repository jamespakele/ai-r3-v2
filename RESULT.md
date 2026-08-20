# RESULT — Add server-side validation to intake save endpoint

## What was built

Server-side validation on the intake save endpoint (`POST /section/01`, handler
`handleSection`). First Name and Last Name are now required before a record is
persisted. On failure the endpoint returns HTTP 400 with a JSON body
`{"errors":{"first_name":"First name is required","last_name":"Last name is required"}}`
and does NOT save the record.

## Files changed

- `r3-intake/internal/server/handlers.go` (+51/-1)
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
- `docs/plans/omp-plan-intake-save-validation.md` (new): the MOA working plan.

## Design decision

The intake schema has a SINGLE `name` column (max 200), not separate
first_name/last_name columns. The card's error contract uses `first_name`/`last_name`
keys. Resolution: the backend validates the submitted `first_name`/`last_name` form
fields and joins them into the single `name` column on save. No schema migration
(avoids a parent-merge conflict). The sibling frontend card (t_0c5c0ad3) submits
`first_name`/`last_name` and renders the JSON error body inline.

## Verification

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (server 18.9s, migrations ok)
- New tests (all 6 PASS):
  - TestSection01MissingNamesRejected — 400, both errors, 0 records
  - TestSection01MissingLastNameOnly — 400, only last_name error, 0 records
  - TestSection01SavesJoinedName — 202, name == "Jane Doe"
  - TestSection01LegacyNameSaves — 202, legacy name preserved
  - TestSection01NoJRedirect — 303 no-JS fallback, saved
  - TestSection02AutosaveSkipsNameValidation — 204, name untouched

## Acceptance criteria

1. POST to save endpoint with missing first/last name returns 400 + field errors,
   no record created — VERIFIED (Test A).
2. POST with both fields populated saves successfully — VERIFIED (Test C).
3. Error response format matches `{"errors":{"field":"message"}}` — VERIFIED
   (Content-Type application/json, exact keys/messages).

## Note

The harness deleted the original worktree `.worktrees/t_67747531` mid-run; omp
recreated it as `.worktrees/t_9bc17b74/r3-intake` on branch `wt/t_67747531` and
re-applied all edits there. All work, tests, and the plan live in that worktree.
