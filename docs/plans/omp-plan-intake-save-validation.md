# Working Plan: Server-side validation on the intake save endpoint

## Objective

Add server-side validation to the intake save endpoint (`POST /section/{01-05}`,
handler `handleSection`) so that First Name and Last Name are required before a
record is persisted. On validation failure, return HTTP 400 with a JSON body
matching the contract `{"errors":{"first_name":"First name is required","last_name":"Last name is required"}}`
and do NOT save the record. Existing valid saves must continue to work.

## Critical design decision — reconciling first/last name with the single `name` column

**The schema has ONE `name` column (text, max 200), not separate first_name/last_name
columns.** The card's error contract uses keys `first_name`/`last_name`, and the
sibling frontend card (t_0c5c0ad3) will add first/last name inputs to the form.

**Decision:** The backend reads the submitted `first_name` and `last_name` form
fields (which the sibling frontend will submit), validates each is non-empty, and
on success **joins them into the single `name` column** (`first_name + " " + last_name`)
for persistence. The errors map uses the `first_name`/`last_name` keys exactly as
the contract specifies, so the frontend can display them inline.

**Backward compatibility:** If `first_name` and `last_name` are BOTH empty but a
legacy `name` field is submitted (no-JS fallback or an older client), the backend
falls back to using `name` directly and does NOT emit first/last validation errors.
This satisfies "Ensure existing valid saves still work."

**Rationale:**
- The error-contract keys (`first_name`/`last_name`) are the frontend's contract;
  the backend must return exactly those keys.
- The single `name` column is the persistence contract; the backend must store the
  full name there.
- Joining on save reconciles the two WITHOUT a schema migration. The parent epic
  owns schema migrations; adding a competing first_name/last_name migration in this
  child worktree would create a merge conflict at parent merge time.
- The sibling frontend owns the template change (splitting the input into first/last);
  this backend card owns the write-path that consumes those fields.

## Scope of validation

The card says "Consider validating other required fields similarly." **This card
validates ONLY the name fields (first/last) on the save endpoint.** Full 12-field
validation stays in the finish flow (`validateRecord`/`validateState`). Rationale:
the save endpoint persists ONE section at a time (autosave). Requiring all 12
required fields on every section save would block progressive entry (a user filling
section 01 before sections 02-05 would be unable to save). Extending save-endpoint
validation to other required fields is a follow-up (next sprint) and must be designed
to not break progressive saving.

## Implementation steps

### Step 1 — Add a name-validation helper (handlers.go)

Add a helper that validates the submitted first/last name and returns the errors map:

```go
// validateNameFields returns a map of field->message for empty first/last name.
// It reads first_name/last_name form fields; if both are empty it falls back to
// the legacy single `name` field. Returns nil when valid.
func validateNameFields(r *http.Request) map[string]string {
    first := strings.TrimSpace(r.FormValue("first_name"))
    last := strings.TrimSpace(r.FormValue("last_name"))
    // Legacy single-name path (no first/last inputs submitted).
    if first == "" && last == "" {
        if strings.TrimSpace(r.FormValue("name")) != "" {
            return nil
        }
        return map[string]string{"name": "Name is required"}
    }
    errs := map[string]string{}
    if first == "" {
        errs["first_name"] = "First name is required"
    }
    if last == "" {
        errs["last_name"] = "Last name is required"
    }
    if len(errs) == 0 {
        return nil
    }
    return errs
}
```

### Step 2 — Add a JSON error writer (handlers.go)

Add a small helper to write the JSON error body with the correct content type:

```go
// writeValidationErrors writes a 400 with the JSON body the frontend expects:
// {"errors":{"field":"message"}}.
func writeValidationErrors(w http.ResponseWriter, errs map[string]string) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusBadRequest)
    _ = json.NewEncoder(w).Encode(map[string]any{"errors": errs})
}
```

`encoding/json` is already imported in handlers.go.

### Step 3 — Wire validation into handleSection (handlers.go ~397)

In `handleSection`, after `getOrCreateIntake` succeeds and BEFORE `applySection`
(so no fields are written to the record on failure), add:

```go
if errs := validateNameFields(r); errs != nil {
    writeValidationErrors(w, errs)
    return
}
```

Because this runs before `applySection` and `saveIntake`, a failed validation
never mutates the in-memory record and never calls `pb.Save` — satisfying
"no record is created."

### Step 4 — Join first/last into the single `name` column (applySection, handlers.go ~538)

In `applySection` case "01", replace the current single-name write:

```go
rec.Set("name", r.FormValue("name"))
```

with logic that prefers the joined first/last and falls back to the legacy name:

```go
first := strings.TrimSpace(r.FormValue("first_name"))
last := strings.TrimSpace(r.FormValue("last_name"))
if first != "" || last != "" {
    rec.Set("name", strings.TrimSpace(first+" "+last))
} else {
    rec.Set("name", r.FormValue("name"))
}
```

This keeps the stored value in the single `name` column (max 200) while consuming
the sibling's first/last inputs. The `strings.TrimSpace(first+" "+last)` collapses
the case where one of the two is empty (e.g. only first filled) to a clean single
token; the validation gate in Step 3 already rejects that case for the save
endpoint, but this guard keeps `applySection` safe for any other caller.

### Step 5 — Rebuild note (go:embed)

Templates are embedded via `go:embed`. This card changes NO template markup (the
sibling owns the first/last input split), so no template rebuild is strictly needed
for THIS card. However, the sibling's template change WILL require a rebuild. The
build/test command `cd r3-intake && go build ./... && go vet ./... && go test ./...`
re-embeds templates automatically. No action beyond the normal build is required.

### Step 6 — Tests (new file: internal/server/intake_save_validation_test.go)

Add integration tests using the existing `newTestServer(t)` harness and CSRF
helpers (`addCSRFToRequest`, `adminCookie`). Drive requests through
`srv.Mux().ServeHTTP(rec, req)`.

**Test A — missing first/last returns 400 + JSON errors, no record created:**
- POST `/section/01` with `first_name=""`, `last_name=""`, `id=""`, plus a valid
  `event` value, with CSRF cookie+header and an admin session cookie.
- Assert status 400, `Content-Type` is `application/json`, body parses to
  `{"errors":{"first_name":"First name is required","last_name":"Last name is required"}}`.
- Assert the intake collection has ZERO records (query `intake` collection, count == 0).

**Test B — missing only last name returns 400 with only last_name error:**
- POST with `first_name="Jane"`, `last_name=""`. Assert 400, errors map has only
  `last_name`, and no record created.

**Test C — both fields populated saves successfully:**
- POST `/section/01` with `first_name="Jane"`, `last_name="Doe"`, `event=<valid id>`.
- Assert 204 (HX-Request header set) or 303 redirect (no HX header).
- Assert exactly ONE intake record exists and its `name` == "Jane Doe".

**Test D — legacy single `name` field still saves (backward compat):**
- POST `/section/01` with `name="Jane Doe"`, no first/last fields.
- Assert 204/303 and record `name` == "Jane Doe".

**Test E — no-JS fallback path:** repeat Test C without the `HX-Request` header;
assert 303 redirect to `/public/intake?id=<id>` (anonymous) or `/intake/<id>`
(authed), and the record was saved.

**Seeding:** reuse the `seedExportData` helper (attendance_export_integration_test.go)
to get a valid `event` id, or create a minimal event via `core.NewRecord` +
`pb.Save`. The `intake` collection has no `deleted` field, so count via
`FindRecordsByFilter(col.Id, "", "", 0, 0)`.

## Verification criteria

1. `cd r3-intake && go build ./... && go vet ./... && go test ./...` passes.
2. Test A: POST with missing first/last returns 400, JSON body with both
   `first_name` and `last_name` errors, and zero intake records created.
3. Test C: POST with both fields populated returns 204/303 and creates a record
   whose `name` is the joined "First Last".
4. Test D: legacy `name`-only POST still saves (no regression).
5. Error response `Content-Type` is `application/json` so the frontend can parse it.

## Logical Consequences

| Site | Decision | Horizon | Type | Notes |
|------|----------|---------|------|-------|
| `handleSection` (handlers.go ~397) | Change | Immediate | Intended | Add validation gate before applySection/saveIntake; return JSON 400 on failure |
| `applySection` case "01" (handlers.go ~538) | Change | Immediate | Intended | Join first/last into single `name`; keep legacy `name` fallback |
| `validateNameFields` (new) | Add | Immediate | Intended | New helper; owns the first/last vs legacy-name decision |
| `writeValidationErrors` (new) | Add | Immediate | Intended | JSON 400 writer; `encoding/json` already imported |
| `intake` schema `name` column (001_init.js) | Keep | Immediate | Intended | No migration; single `name` column persists the joined full name |
| Sibling frontend t_0c5c0ad3 (first/last inputs) | Coordinate | Immediate | Intended | Backend consumes `first_name`/`last_name` form fields; sibling must submit them and handle the JSON error body inline |
| `validateRecord` / `validateState` (finish flow) | Keep | Immediate | Intended | Full 12-field validation stays in finish; save endpoint validates only name to preserve progressive entry |
| `REQUIRED_FIELDS` list (handlers.go ~50) | Keep | Immediate | Intended | Still includes `name`; finish flow unchanged |
| `applyToState` (handlers.go ~620) | Keep (dead) | Next sprint | Unintended-negative | Already has no callers; not touched by this card. Flag for removal in a cleanup card |
| `FormState.Name` + template `name="name"` input (index.html ~135) | Keep | Immediate | Intended | Legacy no-JS path still uses single `name`; sibling replaces the input with first/last but the field stays for backward compat |
| `handlePublicIntake` / `handleIntakeCmd` GET render | Keep | Immediate | Intended | Read path unchanged; `stateFromRecord` still reads `name` column |
| `admin.go` name writes (lines 265/418/436) | Keep | Immediate | Intended | Admin-created intakes use single `name`; unaffected by section-save validation |
| Duplicate search (`/search/duplicates`) | Keep | Immediate | Intended | Reads `name` column; joined full name still searchable |
| CSV export / matrix / roster | Keep | Immediate | Intended | All read `name` column; no change |
| `go:embed` templates | Rebuild | Immediate | Intended | No template change in this card; sibling's change requires rebuild via normal build command |
| Error response format (plain text → JSON) | Change | Immediate | Intended | `handleSection` currently returns `http.Error` (plain text) on applySection/saveIntake failure; only the NEW validation path returns JSON. Existing plain-text error paths unchanged |

## Pre-mortem flip

**"If this shipped and a user was frustrated by it six months from now, what went wrong?"**

1. **Frontend/backend contract drift.** If the sibling frontend submits different
   field names (e.g. `firstName`/`lastName` instead of `first_name`/`last_name`),
   the backend would never see the values and would reject every save. Mitigation:
   the error keys and form field names are BOTH `first_name`/`last_name` per the
   card contract; the plan states this explicitly so the sibling implements to the
   same contract. The parent merge must verify the sibling's submitted field names
   match.

2. **Progressive entry broken.** If validation were extended to all 12 required
   fields on the save endpoint, a user filling section 01 first would be unable to
   save. Mitigation: this card validates ONLY name; other required fields stay in
   the finish flow. Any future extension must be designed to not block progressive
   saving.

3. **Legacy no-JS users blocked.** If the backend required first/last unconditionally,
   the no-JS fallback (which submits a single `name`) would break. Mitigation: the
   legacy `name` fallback in `validateNameFields` and `applySection` keeps that path
   working.

4. **Orphaned records.** If validation ran AFTER `getOrCreateIntake` created a new
   record but the save was rejected, an empty record could linger. Mitigation: the
   validation gate runs before `applySection`/`saveIntake`, and `getOrCreateIntake`
   only creates an in-memory record (persisted only by `saveIntake`), so a rejected
   save creates nothing. Test A asserts zero records.

5. **Name truncation.** A very long first+last could exceed the `name` max 200.
   Mitigation: acceptable for this card (200 chars is generous for a name); the
   sibling's client-side maxlength on the inputs should keep combined length under
   200. Flag as a note for the sibling.

## Affected stakeholders

- **End users (intake clients / case managers):** get clear inline field errors
  instead of a silent failed autosave; valid saves unchanged.
- **Sibling frontend card (t_0c5c0ad3):** must submit `first_name`/`last_name` and
  render the JSON `{"errors":{...}}` body inline; must keep combined name under 200.
- **Parent epic (merge):** must verify the sibling's submitted field names match
  `first_name`/`last_name` and that no competing schema migration was added.
- **Operators / admins:** admin-created intakes (single `name`) unaffected.
