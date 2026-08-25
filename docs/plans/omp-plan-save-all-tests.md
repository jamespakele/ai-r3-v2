# Working Plan — Update tests for all-sections save flow and saveAll presence

Card: `t_1f69d076` — Update the test suite to match the restored all-sections
save flow (`R3F.saveAll()`).

## Objective

Make the `r3-intake` Go test suite reflect the restored `R3F.saveAll()` flow
that sibling card `t_8a980895` already restored in the embedded template:

1. Stop enshrining the broken sec-01-only submit. `TestEmbeddedTemplateIncludesValidationUI`
   must assert `saveAll`/`patchRecordId`/`applyErrors` are **present** and that
   both Save buttons are wired to `R3F.saveAll()` — instead of asserting
   `saveAll` is absent.
2. Rewrite `seedCompleteRecord` so it mirrors the real `R3F.saveAll()` order
   (`sec-02 → sec-03 → sec-04 → sec-05 → sec-01` LAST), not the old
   sec-02-first-then-sec-01-only dance.
3. Add end-to-end coverage of the full save flow: sequential POST of all five
   sections, 202 + `HX-Redirect` on the first save (sec-02 creates the record),
   record-id patching into the subsequent section forms, and 400 error
   surfacing when sec-01 fails the 12-field gate.

This is a **test-suite + template-copy** change. No server handler logic is
touched — `handlers.go handleSection` already implements the contract the
restored JS relies on (verified: `wasNew` ⇒ 202+`HX-Redirect` / 303; only
`section == "01"` runs `validateRecord`; 400 short-circuits before
`saveIntake`).

## Constraints

- Language: Go 1.23 (`go1.23.4 linux/amd64`, verified via `command -v go`).
- Package under test: `r3-intake/internal/server` (package `server`).
- Template is embedded via `//go:embed all:public` in
  `r3-intake/internal/assets/assets.go`; `assets.TemplateString()` returns the
  HTML the template test asserts against. Editing
  `r3-intake/internal/assets/public/index.html` is what the test sees — there is
  no build step to regenerate the embed (Go `//go:embed` reads the file at
  compile time, so `go test` re-embeds automatically).
- The sibling worktree at
  `/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_8a980895/r3-intake/internal/assets/public/index.html`
  is the reviewed/approved reference. `diff` confirms it differs from this
  worktree's template in **exactly three regions** (line 13 `R3F` object, the
  `htmx:beforeSwap` body, and the two Save-button `onclick` handlers). A
  wholesale `cp` of the sibling file is therefore safe and equivalent to a
  minimal 3-region diff.
- Server contract (verified `handlers.go:417-466`):
  - `wasNew := TrimSpace(r.FormValue("id")) == ""`.
  - Only `section == "01"` runs `s.validateRecord(rec)` (12-field gate from
    `REQUIRED_FIELDS = [event, name, dob, contact, race, sexAtBirth,
    servedMilitary, hasPets, employment, mentalHealth, substanceUse,
    fleeingViolence]`). Missing ⇒ `writeValidationErrors` (400 JSON
    `{"errors":{...}}`) and **return before `saveIntake`** — so a failing
    sec-01 leaves the prior sec-02..sec-05 save intact and persists nothing
    new for sec-01.
  - First save (new) ⇒ `202 + HX-Redirect` (authed → `/intake/<id>`,
    public → `/public/intake?id=<id>`) when `HX-Request=true`; `303` otherwise.
  - Subsequent saves ⇒ `204` (HX) / `303` (no-JS).
  - `applySection` (verified `handlers.go:568-621`): sec-03/04/05 set only
    optional fields (`hmis`, `hmisProvider`, `documents`,
    `healthInsuranceDetail`, `housing`, `income`, `casemanagerName`,
    `personal_N`, `servicePlan_N`). A minimal `url.Values{"id":{id}}` POST
    returns 204 cleanly — no gate, empty values are valid.
  - `validateRecord` reads from the in-memory `rec` returned by
    `getOrCreateIntake`. For sec-01 with a patched id, that record is loaded
    from the DB **with the persisted sec-02 radios already set**, so the gate
    passes. This is the load-bearing reason sec-01 must run **last**.

## File Structure

Exactly two files change. No new files, no deletions.

- **Modify:** `r3-intake/internal/assets/public/index.html`
  (copy the restored `saveAll` wiring from the sibling worktree so the
  template-assertion test passes here).
- **Modify:** `r3-intake/internal/server/intake_save_validation_test.go`
  (flip the `saveAll` assertion, rewrite `seedCompleteRecord`, add two new
  coverage tests).

## Implementation Notes

### File 1 — `r3-intake/internal/assets/public/index.html` (copy restored wiring)

Verified `diff` against the sibling shows only three differing regions, so
either a wholesale `cp` of the sibling file or a minimal 3-region patch is
correct. Recommended: `cp` the sibling file verbatim (it is already-reviewed
sibling code, not new invention — per the card's environment facts).

The three regions that change:

1. **Line 13** — the `window.R3F={...}` object literal gains three new methods
   after `validateAll`:
   - `applyErrors:function(errs){...}` — sets all field/group errors from the
     400 JSON `errors` map (same mapping the inline `htmx:beforeSwap` block had).
   - `saveAll:async function(){...}` — sequential `fetch` POST of
     `['sec-02','sec-03','sec-04','sec-05','sec-01']` with
     `headers:{'HX-Request':'true','X-CSRF-Token':getCookie('r3_csrf')},
     redirect:'manual'`; `202` ⇒ read `HX-Redirect`, call
     `R3F.patchRecordId(loc)`, continue; `204` ⇒ continue; `400` ⇒ parse JSON,
     `R3F.applyErrors((data&&data.errors)||{})`, return.
   - `patchRecordId:function(url){...}` — regex
     `/(?:[?&]id=|\/intake\/)([^/?&]+)/` extracts id from `/intake/<id>` or
     `/public/intake?id=<id>`; writes it into the hidden `id` input of
     `sec-01..sec-05` and `finish-form`.

2. **`htmx:beforeSwap` body** — the inline `R3F.setFieldErr/...`/`R3F.setGroupErr/...`
   block is replaced by a single call `R3F.applyErrors((data&&data.errors)||{});`
   (the mapping is now centralized in `applyErrors`).

3. **Both Save buttons** (topbar line 88, finish-form line 118) —
   `onclick="if(R3F.validateAll()){htmx.trigger(document.getElementById('sec-01'),'submit')}"`
   → `onclick="if(R3F.validateAll()){R3F.saveAll()}"`.

After this, `grep -c saveAll index.html` ≥ 3 (definition + 2 onclicks) and
`htmx.trigger(document.getElementById('sec-01'),'submit')` must be GONE from
both Save buttons (the unrelated `validateSection01` `htmx:beforeRequest`
listener stays). `section-save-btn` is absent in both templates, so the kept
negative assertion still passes.

### File 2 — `r3-intake/internal/server/intake_save_validation_test.go`

Five edits. All within the existing file; no new helpers beyond a small
record-id extractor (or reuse `firstIntakeRecord(t, srv).Id`).

#### Edit A — flip `TestEmbeddedTemplateIncludesValidationUI` (last test in file)

Keep the existing positive `want` slice (validateAll, setGroupErr, the group
containers, `if(R3F.validateAll())`). Then:

- **Add** to the `want` slice: `"R3F.saveAll"`, `"R3F.patchRecordId"`,
  `"R3F.applyErrors"`.
- **Replace** the two negative blocks:
  ```go
  if strings.Contains(tmpl, "saveAll") { t.Errorf("...still contains saveAll") }
  if strings.Contains(tmpl, "section-save-btn") { t.Errorf("...section-save-btn") }
  ```
  with:
  ```go
  // saveAll is restored and wired to both Save buttons.
  if !strings.Contains(tmpl, "R3F.saveAll") {
      t.Errorf("embedded template missing R3F.saveAll")
  }
  if !strings.Contains(tmpl, "R3F.patchRecordId") {
      t.Errorf("embedded template missing R3F.patchRecordId")
  }
  if !strings.Contains(tmpl, "R3F.applyErrors") {
      t.Errorf("embedded template missing R3F.applyErrors")
  }
  if c := strings.Count(tmpl, `if(R3F.validateAll()){R3F.saveAll()}`); c != 2 {
      t.Errorf("Save buttons wired to R3F.saveAll() = %d, want 2", c)
  }
  // Stale sec-01-only submit wiring must be gone from Save buttons.
  if strings.Contains(tmpl, `htmx.trigger(document.getElementById('sec-01'),'submit')`) {
      t.Errorf("embedded template still has stale htmx.trigger Save wiring")
  }
  // The old per-section section-save-btn class is still gone (kept assertion).
  if strings.Contains(tmpl, "section-save-btn") {
      t.Errorf("embedded template still contains section-save-btn")
  }
  ```

This satisfies acceptance criterion 1. **Hard prerequisite:** File 1 must land
first or this test fails (Logical Consequence 1).

#### Edit B — rewrite `seedCompleteRecord` to the sequential saveAll flow

Replace the current body (sec-02 then sec-01 only) with the
`02 → 03 → 04 → 05 → 01` order. Use the simplest id source —
`firstIntakeRecord(t, srv).Id` after sec-02 creates the record — so no new
regex helper is needed (this is the consensus simplest path; GLM/DeepSeek
proposed regex extractors but `firstIntakeRecord` already exists and the
record is queryable immediately after the 202):

```go
// seedCompleteRecord mirrors R3F.saveAll(): sec-02 creates the record
// (202 + HX-Redirect for htmx, 303 without JS), sec-03..sec-05 save with the
// patched record id, and sec-01 runs last so the 12-field validateRecord
// gate sees the persisted sec-02 radio values and passes. Returns the id.
func seedCompleteRecord(t *testing.T, srv *Server, cookie *http.Cookie, fx sectionFixture, hx bool) string {
    t.Helper()

    firstWant := http.StatusAccepted
    if !hx {
        firstWant = http.StatusSeeOther
    }
    rec02 := doSectionPost(t, srv, cookie, hx, "02", validSection02Form(""))
    if rec02.Code != firstWant {
        t.Fatalf("sec-02 seed status = %d, want %d", rec02.Code, firstWant)
    }
    id := firstIntakeRecord(t, srv).Id

    subsequentWant := http.StatusNoContent
    if !hx {
        subsequentWant = http.StatusSeeOther
    }
    // sec-03/04/05 only set optional fields — a minimal id-only POST returns 204.
    for _, sec := range []string{"03", "04", "05"} {
        rec := doSectionPost(t, srv, cookie, hx, sec, url.Values{"id": {id}})
        if rec.Code != subsequentWant {
            t.Fatalf("sec-%s seed status = %d, want %d", sec, rec.Code, subsequentWant)
        }
    }

    // sec-01 LAST: full 12-field gate passes (sec-02 radios already persisted).
    rec01 := doSection01Post(t, srv, cookie, hx, validSection01Form(fx, map[string][]string{"id": {id}}))
    if rec01.Code != subsequentWant {
        t.Fatalf("sec-01 seed status = %d, want %d", rec01.Code, subsequentWant)
    }
    return id
}
```

The signature is unchanged, so the existing call sites
(`TestSection02AutosaveSkipsValidation`) keep compiling and passing. This
satisfies acceptance criterion 2.

#### Edit C — add full-flow coverage: `TestSaveAllSequentialFlowCoversAllSections`

This is the new end-to-end test (acceptance criterion 3, first half). It posts
all five sections in the saveAll order, asserts 202+`HX-Redirect` on sec-02,
asserts record-id patching (subsequent sections carry the new id and reuse the
single record — count stays 1), asserts 204 for sec-03/04/05/01, and asserts
the joined `name` plus the sec-02 radios persist on the one record.

```go
func TestSaveAllSequentialFlowCoversAllSections(t *testing.T) {
    srv := newTestServer(t)
    fx := seedActiveEvent(t, srv.pb)
    admin := adminCookie(srv, fx.admin)

    // sec-02 first: creates the record, 202 + HX-Redirect (authed → /intake/<id>).
    rec02 := doSectionPost(t, srv, admin, true, "02", validSection02Form(""))
    if rec02.Code != http.StatusAccepted {
        t.Fatalf("sec-02 status = %d, want 202", rec02.Code)
    }
    loc := rec02.Header().Get("HX-Redirect")
    if !strings.HasPrefix(loc, "/intake/") {
        t.Fatalf("HX-Redirect = %q, want prefix /intake/", loc)
    }
    id := firstIntakeRecord(t, srv).Id

    // Record-id patching: subsequent section forms carry the new id so each
    // POST updates the same record (204) rather than creating duplicates.
    for _, sec := range []string{"03", "04", "05"} {
        r := doSectionPost(t, srv, admin, true, sec, url.Values{"id": {id}})
        if r.Code != http.StatusNoContent {
            t.Fatalf("sec-%s status = %d, want 204", sec, r.Code)
        }
    }
    if n := countIntakeRecords(t, srv); n != 1 {
        t.Fatalf("intake records after sec-02..sec-05 = %d, want 1 (id patching must reuse the record)", n)
    }

    // sec-01 last: carries the patched id; full 12-field gate passes because
    // the persisted sec-02 radios satisfy mentalHealth/substanceUse/fleeingViolence.
    rec01 := doSection01Post(t, srv, admin, true, validSection01Form(fx, map[string][]string{"id": {id}}))
    if rec01.Code != http.StatusNoContent {
        t.Fatalf("sec-01 status = %d, want 204", rec01.Code)
    }
    if n := countIntakeRecords(t, srv); n != 1 {
        t.Fatalf("intake records = %d, want 1", n)
    }
    saved := firstIntakeRecord(t, srv)
    if got := saved.Id; got != id {
        t.Fatalf("record id = %q, want %q (id must be patched, not a new record)", got, id)
    }
    if got := saved.GetString("name"); got != "Jane Doe" {
        t.Fatalf("name = %q, want %q", got, "Jane Doe")
    }
    if got := saved.GetString("mentalHealth"); got != "no" {
        t.Fatalf("mentalHealth = %q, want %q", got, "no")
    }
    if got := saved.GetString("substanceUse"); got != "no" {
        t.Fatalf("substanceUse = %q, want %q", got, "no")
    }
    if got := saved.GetString("fleeingViolence"); got != "no" {
        t.Fatalf("fleeingViolence = %q, want %q", got, "no")
    }
    if got := saved.GetString("event"); got != fx.event {
        t.Fatalf("event = %q, want %q", got, fx.event)
    }
}
```

#### Edit D — add 400-error surfacing coverage: `TestSaveAllSurfaces400OnInvalidSection01`

This is the second half of acceptance criterion 3. It exercises the 400 path
**within the saveAll flow**: sec-02 creates the record, sec-03/04/05 save,
then sec-01 fails the gate. The correct assertion — per Logical Consequence 4
and the server contract (400 short-circuits before `saveIntake`) — is
**count==1 and `name` is empty** (the record exists from sec-02; sec-01 added
nothing). It must NOT assert count==0 (that would be the standalone-sec-01
case, not the saveAll flow, and would enshrine a false "no partial save" claim
that contradicts the real behavior).

```go
func TestSaveAllSurfaces400OnInvalidSection01(t *testing.T) {
    srv := newTestServer(t)
    fx := seedActiveEvent(t, srv.pb)
    admin := adminCookie(srv, fx.admin)

    // sec-02 first: creates the record (202).
    rec02 := doSectionPost(t, srv, admin, true, "02", validSection02Form(""))
    if rec02.Code != http.StatusAccepted {
        t.Fatalf("sec-02 status = %d, want 202", rec02.Code)
    }
    id := firstIntakeRecord(t, srv).Id

    for _, sec := range []string{"03", "04", "05"} {
        r := doSectionPost(t, srv, admin, true, sec, url.Values{"id": {id}})
        if r.Code != http.StatusNoContent {
            t.Fatalf("sec-%s status = %d, want 204", sec, r.Code)
        }
    }

    // sec-01 last but missing first_name+last_name (and the rest): server
    // surfaces a 400 JSON body R3F.applyErrors would consume. The record
    // already exists from sec-02; sec-01's failed gate persists nothing new.
    form := validSection01Form(fx, map[string][]string{
        "id":         {id},
        "first_name": {}, // drop
        "last_name":  {}, // drop
    })
    rec01 := doSection01Post(t, srv, admin, true, form)
    if rec01.Code != http.StatusBadRequest {
        t.Fatalf("sec-01 status = %d, want 400", rec01.Code)
    }
    if ct := rec01.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
        t.Fatalf("Content-Type = %q, want application/json", ct)
    }
    errs := parseErrorsBody(t, rec01)
    if _, ok := errs["first_name"]; !ok {
        t.Errorf("errors = %v, want key %q present", errs, "first_name")
    }
    // Correct saveAll-flow assertion: record already created by sec-02;
    // failed sec-01 must not duplicate it nor persist the name.
    if n := countIntakeRecords(t, srv); n != 1 {
        t.Fatalf("intake records = %d, want 1 (sec-02 record persists; failed sec-01 must not create a duplicate)", n)
    }
    if got := firstIntakeRecord(t, srv).GetString("name"); got != "" {
        t.Fatalf("name = %q, want empty (failed sec-01 must not persist)", got)
    }
}
```

#### Edit E (optional, recommended) — refresh stale comments

- `validSection01Form` doc comment currently says "Tests that need a complete
  record save sec-02 first (see seedCompleteRecord)." Update to: "…use
  seedCompleteRecord, which mirrors R3F.saveAll() by saving sec-02..sec-05
  first, then sec-01 last so the full 12-field gate passes."
- `TestSection01AllFieldsPresentSucceeds` already posts sec-02→sec-01 (a
  valid subset of the new flow) and passes unchanged; only its doc comment
  ("sec-02 autosave first") is slightly stale. Optionally update the comment
  to reference the saveAll ordering. The test body need not change — it is a
  focused sec-01-happy-path test, not a full-flow test (that is Edit C's job).

### Decisions reconciled across source agents

- **id source:** Use `firstIntakeRecord(t, srv).Id` (Kimi/GLM/MiniMax path)
  rather than a regex extractor (DeepSeek/Qwen). The record is queryable the
  instant sec-02 returns 202, and the helper already exists — avoids adding
  `regexp` import or a new helper. **Minority preserved:** DeepSeek/Qwen's
  regex extractor is documented here as an acceptable alternative if a future
  test needs to assert the `HX-Redirect` URL shape directly (Edit C already
  asserts `HasPrefix("/intake/")`, which covers the same contract).
- **sec-03/04/05 form shape:** Minimal `url.Values{"id":{id}}` (Kimi/GLM/MiniMax)
  rather than explicit field-populated builders (DeepSeek/Qwen). Verified in
  `applySection`: sec-03/04/05 set only optional fields with no gate, so an
  id-only POST returns 204. This keeps `seedCompleteRecord` and the new tests
  focused on the save-flow contract. **Minority preserved:** if a future card
  wants to assert sec-03/04/05 *field* persistence, the DeepSeek/Qwen form
  builders (with `hmis`, `personal_N`, `servicePlan_N`) are the reference.
- **400 record-count assertion:** All five agents converge — in the saveAll
  flow the record already exists (created by sec-02), so the assertion is
  `count==1 && name==""`, NOT `count==0`. This is Logical Consequence 4 and is
  load-bearing: a `count==0` assertion would fail (sec-02 already saved) and
  would enshrine a false no-persist claim.
- **`TestSection01AllFieldsPresentSucceeds`:** Leave the body unchanged
  (GLM/DeepSeek); only optionally refresh the comment (Kimi/MiniMax). It is a
  focused happy-path test, not a full-flow test, and its sec-02→sec-01 subset
  still passes under the new wiring.

## Verification Criteria

Runnable checks, in order. All must be run from the worktree root
`/srv/data/1-projects/ai-projects/ai-r3-v2/.worktrees/t_1f69d076`.

1. **Template copy landed (File 1 prerequisite):**
   ```sh
   grep -c 'saveAll' r3-intake/internal/assets/public/index.html   # ≥ 3
   grep -c "if(R3F.validateAll()){R3F.saveAll()}" r3-intake/internal/assets/public/index.html  # 2
   ! grep -q "htmx.trigger(document.getElementById('sec-01'),'submit')" r3-intake/internal/assets/public/index.html  # exit 1 (absent)
   grep -c 'R3F.patchRecordId' r3-intake/internal/assets/public/index.html  # ≥ 1
   grep -c 'R3F.applyErrors' r3-intake/internal/assets/public/index.html    # ≥ 2 (def + beforeSwap call)
   ```

2. **Build:**
   ```sh
   go build ./...
   ```
   Expected: exit 0, no output.

3. **Vet:**
   ```sh
   go vet ./...
   ```
   Expected: exit 0, no output.

4. **Full server test suite (acceptance gate):**
   ```sh
   go test ./r3-intake/internal/server/... -count=1
   ```
   Expected: `PASS` with `ok` line. Specifically these tests must all pass:
   - `TestEmbeddedTemplateIncludesValidationUI` — now asserts `saveAll`
     present + 2 Save buttons wired to `R3F.saveAll()` (criterion 1).
   - `TestSection01MissingEachFieldRejected`,
     `TestSection01MultipleMissingReturnsAllErrors`,
     `TestSection01AllFieldsPresentSucceeds`,
     `TestSection01ValidationFailurePersistsNothing`,
     `TestSection01NoJSRedirect`,
     `TestSection02AutosaveSkipsValidation` — unchanged behavior, must still
     pass (the rewritten `seedCompleteRecord` keeps the same signature).
   - `TestSaveAllSequentialFlowCoversAllSections` — NEW, covers criterion 3
     (full sequential flow, 202+HX-Redirect, id patching, count stays 1).
   - `TestSaveAllSurfaces400OnInvalidSection01` — NEW, covers criterion 3
     (400 surfacing within saveAll; count==1, name=="").

5. **Targeted run of the two new tests (fast feedback):**
   ```sh
   go test ./r3-intake/internal/server/... -run 'TestSaveAll' -count=1 -v
   ```
   Expected: both `--- PASS` and an `ok` line.

6. **Negative regression guard (enshrined-broken test is gone):**
   ```sh
   ! grep -q 'still contains saveAll' r3-intake/internal/server/intake_save_validation_test.go  # exit 1
   grep -q 'R3F.saveAll' r3-intake/internal/server/intake_save_validation_test.go               # present
   grep -q 'sec-03' r3-intake/internal/server/intake_save_validation_test.go                     # new flow present
   ```
   The old `if strings.Contains(tmpl, "saveAll") { t.Errorf("...still contains saveAll") }`
   line must be gone (criterion 1's negative side).

7. **`seedCompleteRecord` reflects the new ordering (criterion 2):**
   ```sh
   grep -A2 'func seedCompleteRecord' r3-intake/internal/server/intake_save_validation_test.go | grep -E '"02"|"03"|"04"|"05"'
   ```
   Expected: all four section literals appear in the rewritten body (the old
   version only had `"02"` and the `"01"` via `doSection01Post`).

## Logical Consequences

1. **`TestEmbeddedTemplateIncludesValidationUI` ↔ template coupling (immediate).**
   Site: the template-assertion test. Decision: **change** — flip from
   "saveAll absent" to "saveAll present + wired". And then what? The test now
   hard-depends on the sibling's restored template being present in *this*
   worktree. And then what? If a future card reverts the template, this test
   fails loudly — which is the desired guard. Mitigation: File 1 (template
   copy) MUST land before File 2 edits are validated; the plan orders them so.
   Stakeholder: the test suite (intended — amplifies the restored-wiring
   invariant).

2. **`seedCompleteRecord` call sites (immediate).** Site:
   `TestSection02AutosaveSkipsValidation` (the only caller). Decision: **keep
   the call, change the helper body**. And then what? The caller still gets a
   complete record with sec-02 radios persisted, so its "autosave sec-02 with
   no required fields returns 204 and leaves name untouched" assertion still
   holds. And then what? The helper now also exercises sec-03/04/05, so
   `TestSection02AutosaveSkipsValidation` implicitly gains coverage of the
   full-save prerequisite — a positive side effect, no assertion change
   needed. Time horizon: immediate. Stakeholder: that test (intended).

3. **`TestSection01AllFieldsPresentSucceeds` (immediate).** Site: the existing
   sec-01 happy-path test. Decision: **keep body, refresh comment**. And then
   what? It posts sec-02→sec-01 (a valid subset of saveAll) and still passes —
   no behavior change. And then what? It does NOT cover sec-03/04/05, which is
   why Edit C adds the dedicated full-flow test rather than overloading this
   one. Time horizon: immediate. Stakeholder: test suite (intended — keeps the
   focused test focused; full-flow coverage lives in its own test).

4. **400-within-saveAll record-count semantics (immediate, load-bearing).**
   Site: the new `TestSaveAllSurfaces400OnInvalidSection01`. Decision: assert
   `count==1 && name==""`, NOT `count==0`. And then what? This correctly
   reflects that sec-02 already persisted the record before sec-01's gate
   fired. And then what? If a future card changes `handleSection` so a failed
   sec-01 rolls back the entire record (transactional save), this test would
   need to flip to `count==0` — but that is not the current server contract
   (400 returns before `saveIntake`, leaving prior saves intact). Time
   horizon: immediate (correctness now), revisited if transactional save is
   introduced. Stakeholder: backend contract owners. Mitigation: the test
   comment explicitly states the contract basis so a future refactorer knows
   which assertion to revisit.

5. **Stale `htmx.trigger` Save wiring (immediate).** Site: both Save buttons in
   `index.html`. Decision: **remove** the `htmx.trigger(document.getElementById('sec-01'),'submit')`
   onclick and replace with `R3F.saveAll()`. And then what? No code path
   references the old onclick string except the new test's negative assertion
   (which guards against regression). And then what? The unrelated
   `validateSection01` `htmx:beforeRequest` listener (which keys on
   `cfg.path.indexOf('/section/01')`) stays — it is now dead for the
   saveAll-driven sec-01 POST (which uses `fetch`, not htmx), but it is
   harmless and would still fire if a no-JS fallback submit of sec-01 ever
   happens. Time horizon: immediate (cleanup), next sprint (decide whether to
   prune the now-mostly-dead `validateSection01` listener — out of scope for
   this card). Stakeholder: frontend. Negative consequence to mitigate: the
   dead listener is left in place intentionally to avoid scope creep; a
   follow-up card should evaluate removing it.

6. **`section-save-btn` class (immediate).** Site: the kept negative assertion
   in the template test. Decision: **keep** asserting it absent. And then
   what? This guards against re-introducing the old per-section save buttons
   that the original removal card (03e5742) deleted. And then what? It remains
   a stable negative invariant independent of the saveAll restoration. Time
   horizon: immediate (no change). Stakeholder: test suite (intended).

7. **Embedded template re-embed on `go test` (immediate, environmental).**
   Site: `//go:embed all:public` in `assets.go`. Decision: **no change** —
   `go test` re-reads `public/index.html` at compile time, so editing the HTML
   is sufficient; no codegen step. And then what? CI caches must be
   invalidated if they cache the embed (normal Go behavior handles this). Time
   horizon: immediate. Stakeholder: CI.

## Consensus & Divergence

- **Consensus (all 5 agents):** Copy the sibling's restored template into this
  worktree first (File 1 is a hard prerequisite); flip the `saveAll` assertion
  to present+wired; rewrite `seedCompleteRecord` to `02→03→04→05→01`; add a
  full-flow test asserting 202+HX-Redirect + id patching + count==1; add a
  400 test asserting count==1 + name empty (NOT count==0) because sec-02
  already created the record. Server contract confirmed in `handlers.go`.
- **Divergence — id extraction:** GLM proposed a string-index helper;
  DeepSeek/Qwen proposed a `regexp` extractor; Kimi/MiniMax proposed
  `firstIntakeRecord(t, srv).Id`. **Resolved:** use
  `firstIntakeRecord(t, srv).Id` (simplest, helper exists, no new import).
  Regex extractor documented as an acceptable alternative.
- **Divergence — sec-03/04/05 form shape:** DeepSeek/Qwen proposed explicit
  field-populated builders; GLM/Kimi/MiniMax proposed minimal id-only POSTs.
  **Resolved:** minimal id-only (verified `applySection` sets only optional
  fields with no gate). Explicit builders documented for future field-level
  coverage.
- **Divergence — `TestSection01AllFieldsPresentSucceeds`:** Kimi/MiniMax
  suggested rewriting its body to include sec-03/04/05; GLM/DeepSeek said leave
  it. **Resolved:** leave the body (it is a focused happy-path test; the
  full-flow test is Edit C), optionally refresh the comment.
- **No failed/missing sources.** All 5 source agents completed (`status: done`).
  Manifest path: `/tmp/fusion-harness-baft1z/source-manifest.json`.