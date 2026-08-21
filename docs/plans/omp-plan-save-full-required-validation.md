# Working Plan: Make Save handler run full required-field validation

## Objective

Make the section-01 Save path in `r3-intake/internal/server/handlers.go`
(`handleSection`) run the full 12-field required-field validation instead of only
the current name-only check. On any missing required field, return a 400 JSON
validation response listing **every** missing field and do **not** persist the
record. Preserve existing behavior for valid records and for sections 02-05
autosaves.

This card owns **only** the server-side Go change in `handlers.go`. Client-side
JS inline validation across the 5 sections and the Go tests for this behavior are
owned by SIBLING cards (see Constraints); this plan does not touch `index.html`
JS or add test files.

## Constraints

- Scope is limited to `r3-intake/internal/server/handlers.go`. No changes to
  `internal/assets/public/index.html` (JS/template) and no new `*_test.go`
  files — those belong to sibling cards `t_9edc9530` (client JS) and
  `t_ea9f8cd8` (tests + template embed rebuild).
- The 12 `REQUIRED_FIELDS` (handlers.go L50): `event`, `name`, `dob`, `contact`,
  `race`, `sexAtBirth`, `servedMilitary`, `hasPets`, `employment`,
  `mentalHealth`, `substanceUse`, `fleeingViolence`.
- The form represents the person's name via `first_name` + `last_name` inputs;
  the record stores a single joined `name`. `validateRecord` produces a `name`
  key, but the template and client render `first_name`/`last_name`. The error
  response must translate the `name` requirement into `first_name` +
  `last_name` keys. All other REQUIRED_FIELDS keys pass through as-is.
- Full 12-field validation applies ONLY on the section-01 Save path. Sections
  02-05 autosaves continue to save progress without requiring all fields.
- Keep `validateNameFields` (it provides granular first/last strictness; see
  Logical Consequences).
- Preserve existing behavior for valid records and non-01 sections.

## File Structure

Only one file changes:

| Path | Change |
|------|--------|
| `r3-intake/internal/server/handlers.go` | Reorder + extend the section-01 validation gate in `handleSection`; add a `map[string]bool` -> `map[string]string` translation helper (`requiredFieldMessages`). |

No new files. No template/JS changes (siblings own those).

## Implementation Notes

### 1. Reorder `handleSection` — validate AFTER applySection, run full check on section 01

Move the `applySection` call ahead of the validation gate (validation reads the
record, which needs the applied form values), and replace the name-only gate
with the full 12-field check. Current block (handlers.go L414-421):

```go
	if section == "01" {
		if errs := validateNameFields(r); errs != nil {
			writeValidationErrors(w, errs)
			return
		}
	}
	if err := s.applySection(rec, r, section); err != nil {
		http.Error(w, "section apply failed: "+err.Error(), http.StatusBadRequest)
		return
	}
```

Replace with:

```go
	if err := s.applySection(rec, r, section); err != nil {
		http.Error(w, "section apply failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Section 01 is the canonical Save entrypoint. Run the full 12-field
	// required-field validation against the applied record: if any required
	// field is empty, return a 400 JSON error listing every missing field and
	// do NOT persist. Sections 02-05 are autosaves that intentionally save
	// progress without requiring all fields.
	if section == "01" {
		if missing := s.validateRecord(rec); len(missing) > 0 {
			writeValidationErrors(w, requiredFieldMessages(missing))
			return
		}
	}
```

The subsequent `saveIntake(rec)` and the redirect / 204 / 202 HTMX behavior stay
exactly as-is.

### 2. Add a `map[string]bool` -> `map[string]string` translation helper

`validateRecord` (L685) returns `map[string]bool` keyed by the REQUIRED_FIELDS
keys; `writeValidationErrors` (L825) expects `map[string]string`. The required
`name` key must be surfaced as `first_name` + `last_name` so the template and
client JS can render it (the form edits first/last separately; the record stores
a single joined `name`). Add near `writeValidationErrors`:

```go
// requiredFieldMessages translates validateRecord/validateState's
// map[string]bool of missing REQUIRED_FIELDS keys into the map[string]string the
// frontend and template expect. The form edits the name as first_name +
// last_name, but the record stores a single joined `name`, so the `name`
// requirement is surfaced as first_name + last_name errors. All other keys pass
// through unchanged. Messages mirror index.html's per-field error text.
func requiredFieldMessages(missing map[string]bool) map[string]string {
	if len(missing) == 0 {
		return nil
	}
	msgs := map[string]string{}
	for k := range missing {
		if k == "name" {
			msgs["first_name"] = "First name is required."
			msgs["last_name"] = "Last name is required."
			continue
		}
		msgs[k] = requiredFieldMessage(k)
	}
	return msgs
}

// requiredFieldMessage returns the human-readable message for a required field,
// matching the error text already rendered by index.html.
func requiredFieldMessage(k string) string {
	switch k {
	case "event":
		return "Please select an event."
	case "race":
		return "Please select at least one."
	case "dob":
		return "Date of birth is required."
	case "contact":
		return "Contact number is required."
	case "sexAtBirth", "servedMilitary", "hasPets", "employment",
		"mentalHealth", "substanceUse", "fleeingViolence":
		return "Please select one."
	default:
		return "This field is required."
	}
}
```

Message strings deliberately match the server-rendered `field-error` text in
`index.html` (e.g. `event` "Please select an event.", `dob` "Date of birth is
required.", `race` "Please select at least one.", the select fields "Please
select one.") so a 400 JSON response and the template's re-render never
disagree.

### 3. Validation behavior by section (unchanged semantics, reordered execution)

- **Section 01 (Save):** full 12-field validation AFTER `applySection`; on any
  missing field, 400 JSON listing every missing field, NO DB write.
- **Sections 02-05 (autosave):** `applySection` then `saveIntake`, no full-field
  gate — "save your progress as you go" is preserved.
- **Valid records:** validation passes, `saveIntake` runs, redirect/204/202
  unchanged.

### 4. Kept: `validateNameFields` (L796)

Left in place and unused by `handleSection`. It remains a package-level func, so
this is valid Go; the sibling client-JS card (`t_9edc9530`) still references the
name-error concept and may re-use it. Do not delete in this card.

### 5. No changes to `validateState` / `stateFromRecord`

`validateState` (L705) and the `/intake/<id>/finish` path (`stateFromRecord` +
`validateRecord` at L519, L508) already run the full 12-field check and render
errors into the re-displayed page. That behavior is correct and unchanged; this
card only fixes the POST save path that bypassed it.

## Verification Criteria

Build / vet / test (templates are go:embed, so a rebuild is required):

```bash
cd r3-intake && go build ./... && go vet ./... && go test ./...
```

Manual / route-level verification (the sibling test card `t_ea9f8cd8` will encode
these; this card verifies by inspection + build since no new tests are added
here):

- **Missing-field rejection:** POST `/section/01` (HTMX, section-01 form) with
  any subset of the 12 required fields empty returns `400` with body
  `{"errors":{...}}` containing **every** missing field key (`event`, `dob`,
  `contact`, `race`, `sexAtBirth`, `servedMilitary`, `hasPets`, `employment`,
  `mentalHealth`, `substanceUse`, `fleeingViolence`, and `name` expressed as
  `first_name` + `last_name`). No intake record is written (no new row in the
  `intake` collection).
- **No DB write on failure:** confirm the intake count is unchanged after a
  rejected POST.
- **All-filled acceptance:** POST `/section/01` with all 12 required fields
  filled returns the existing success response (204 / 202 / redirect) and the
  record IS persisted with the applied values.
- **Sections 02-05 autosave:** POST `/section/02`..`05` with only partial
  fields still saves progress (no 400).
- **Field-key translation:** a response missing only the name reports
  `first_name` and `last_name` (never a bare `name`), so the client JS
  (`R3F.setFieldErr('fn-input'/'ln-input', errs.first_name||'')`) and the
  template's `{{.Errors.first_name}}`/`{{.Errors.last_name}}` render correctly.
- **Regression:** `go vet ./...` clean; no unused-symbol errors from keeping
  `validateNameFields`.

## Logical Consequences

| # | Downstream site | Decision | Horizon | Type | Rationale |
|---|---|---|---|---|---|
| 1 | `handleSection` section-01 gate (handlers.go L414-421) | **change** | Immediate | Intended | The literal fix: reorder to validate after `applySection`, run `validateRecord` on the applied record, return 400 + full field list, do not persist. |
| 2 | `validateRecord` (L685) | **keep** (reuse as-is) | Immediate | Intended | Already implements the exact 12-field rule incl. `race` via `anyBool(asBoolMap)` and `contact` via `digitsOnly`; calling it on the applied record is the whole fix. |
| 3 | `writeValidationErrors` (L825) | **keep** (call with translated map) | Immediate | Intended | Contract unchanged: 400 + `{"errors":{field:msg}}`. Only the input map now carries all missing fields. |
| 4 | `name` key -> `first_name`/`last_name` translation | **new helper** `requiredFieldMessages` | Immediate | Intended | `validateRecord` emits a bare `name` key, but template (`.Errors.first_name/.last_name`) and client JS (`errs.first_name/errs.last_name`) only render the split keys. Without translation the error would be silently dropped on the client. |
| 5 | `validateNameFields` (L796) | **keep, unused** | Immediate | Intended | Sibling client card may reference it; harmless package-level func; leaving it is valid Go. Deleting it is out of scope and risks a sibling merge conflict. |
| 6 | `validateState` (L705) + `/intake/<id>/finish` + `stateFromRecord` (L519/L508) | **keep, unchanged** | Immediate | Intended | These already run the full check and render into the page. This card aligns the POST-save path with them, closing the bypass. No duplicate logic added. |
| 7 | Client JS `htmx:beforeSwap` (index.html L18-28) | **keep, unchanged this card** | Immediate / next sprint | Intended | It already `preventDefault()`s and renders `errs.first_name/errs.last_name`. This card's server change feeds it the right keys; sibling card `t_9edc9530` will extend it to the other 10 fields. |
| 8 | `index.html` per-field `{{if .Errors.X}}` error divs | **keep, unchanged this card** | Next sprint | Intended | The 400 JSON now reaches only the JS error path, not a re-render. Sibling card extends these for client-side inline validation. No dead UI here. |
| 9 | Sibling card `t_9edc9530` (client JS inline validation across 5 sections) | **keep / coordinate** | Next sprint | Intended | Depends on this server emitting all missing fields. This card defines the JSON contract (`first_name`/`last_name` + passthrough keys) the sibling's JS will consume. |
| 10 | Sibling card `t_ea9f8cd8` (tests for 12-field behavior + template embed rebuild) | **keep / coordinate** | Next sprint | Intended | Will encode this card's behavior as route-level tests. Needs this Go change present (or the sibling copies it in per the worktree pattern). |
| 11 | `REQUIRED_FIELDS` (L50) | **keep, unchanged** | Immediate | Intended | Single source of truth for the 12 fields; `validateRecord` already consumes it. No duplication. |
| 12 | `intake` collection write path | **change (now guarded)** | Immediate | Intended | Invalid records no longer persist. Existing partial/draft records saved before this change remain valid data; nothing is migrated or deleted. |
| 13 | "Save your progress as you go" UX for sections 02-05 | **keep** | Immediate | Intended (positive to amplify) | Sections 02-05 remain partial-save. Only section-01 Save becomes strict. This is the intended split. Stakeholder: end users mid-intake. |
| 14 | Progress meter / `countFilled` (L155-187) | **keep, unchanged** | Immediate | Intended | Independent of this change; continues to show completion against the same 12 fields. No contradictory state. |
| 15 | `ssn`, `email`, `livingWith`, `sleptWhere`, `raceOther`, `household`, `militaryDetail`, `pet*`, etc. | **keep, NOT required** | Immediate | Intended | Deliberately not in REQUIRED_FIELDS; the spec names only the 12. No accidental widening. |
| 16 | Error message strings duplicated between Go and template | **keep (align), flag for next sprint** | Next sprint | Intended | This card hard-codes messages matching `index.html`. A future card could centralize them to a single constant set to prevent drift; not needed for this card. |
| 17 | Docs / user guide describing the form | **keep / check** | Next quarter | Intended | No feature removal; save becomes stricter on section 01. Any docs that imply partial Save is always allowed should be reconciled when the sibling work lands. Low risk. |

### Trace: end-to-end data flow (reconciled)

Request -> `handleSection` (section 01) -> `getOrCreateIntake` -> (existing
access checks) -> **`applySection` (NEW position: populate record)** ->
**`validateRecord(rec)` reads the applied values** -> missing? -> 400 JSON with
all missing fields, return (no `saveIntake`) -> else `saveIntake(rec)` ->
redirect/204/202. The prior order validated BEFORE applying, so the record was
always empty and full validation could never pass. The reorder is the fix.

### Consequence questions

- **Intended:** invalid section-01 Saves no longer write garbage/partial
  intakes; users get a complete, actionable list of what's missing; the POST
  path finally matches the already-correct `/finish` validation. Stakeholders:
  end users, case managers reviewing records, the intake collection (data
  quality).
- **Positive to amplify:** a single 400 response lists ALL missing fields at
  once (not one-at-a-time), so users fix everything in one pass — better than the
  old first-error-only UX. The contract this defines enables the sibling
  client-side inline validation card.
- **Negative to mitigate:** a user who reached section 01 early and expected a
  partial save of name/dob/contact now gets a hard block until all 12 are filled.
  Mitigation: section 01 is the canonical Save entrypoint (per spec); autosave
  sections 02-05 still allow partial progress, and a fully-empty draft created by
  `getOrCreateIntake` before validation is only written when valid. Document the
  stricter section-01 rule when the sibling work lands.
- **Misleading state left behind:** none — `validateNameFields` is intentionally
  kept as a valid unused func (not misleading), and no template/UI is orphaned by
  this change.

### Pre-mortem flip

"If this shipped and a user was frustrated six months from now, what would they
say went wrong?" Likely: *"I clicked Save on section 1 and got errors for fields
in sections 2-5 that I hadn't reached yet, with no way to save my name/dob to
come back later."* This plan addresses that directly: (a) the error response is
the full list by design (the spec demands it), (b) sections 02-05 remain
partial-autosave so progress isn't lost, and (c) the sibling client card will add
inline field-level hints so the all-fields-at-once block is clear and actionable
rather than confusing. The remaining trade-off — section-01 Save requires all 12
fields — is an explicit product decision from the card spec, and the 400 lists
every missing field so the user is never guessing. If product later wants a
"draft" capability on section 01, that is a separate card (next quarter), not a
change to this one.
