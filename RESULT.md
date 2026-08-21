# RESULT — Epic 34: Remove per-section Save buttons + equal-size Save/New

## Goal

Remove the redundant per-section "Save section 0X" buttons from the R3 intake form,
make the single top-level Save button persist all five sections via `R3F.saveAll()`,
and ensure the bottom Save and New buttons render at identical sizes.

## What was built

### Remove per-section Save buttons + saveAll (t_2e1c5fdd)

- Removed the 5 `<button class="section-save-btn">Save section 0N</button>` elements
  from `sec-01`..`sec-05`.
- Each form keeps its `hx-post="/section/0N" hx-trigger="submit"` attributes.
- Added `R3F.saveAll()` — a sequential `await fetch` loop over all 5 sections that:
  - sends `HX-Request: true` so the server's 202/HX-Redirect path is exercised,
  - on a new record reads `HX-Redirect`, extracts the new record id, and patches
    `input[name=id]` across all 5 forms + `#finish-form` to avoid duplicate records,
  - replicates the section-01 First/Last validation before posting and aborts on 400.
- Both top-level Save buttons now call `R3F.saveAll()`.
- Updated topbar and intro help text so they no longer reference per-section saving.
- Removed the dead `.section-save-btn` CSS block.
- Bumped all 9 `/static/app.css?v=8` cache-buster links to `?v=9`.

### Equal-size Save/New buttons (t_22fbf8d0)

- Replaced the single `.finish-actions .btn-primary` sizing rule with a combined
  selector that also sizes `.btn-ghost`:
  ```css
  .finish-actions .btn-primary,
  .finish-actions .btn-ghost { box-sizing: border-box; padding: 13px 26px; font-size: 15px; line-height: 1; }
  ```
- `box-sizing: border-box` is required because there is no global border-box reset;
  without it the ghost button's 1px border creates a 2px height gap versus the
  primary button's `border: none`, even when padding matches.
- Scoped strictly to `.finish-actions`; no other buttons are affected.

## Files changed

- `r3-intake/internal/assets/public/index.html`
- `r3-intake/internal/assets/public/app.css`
- `RESULT.md`
- `docs/plans/omp-plan-remove-per-section-save-buttons.md`
- `docs/plans/omp-plan-save-new-buttons-same-size.md`
- `docs/plans/omp-plan-epic34-integration.md`

## Verification

- `grep "Save section" r3-intake/internal/assets/public/index.html` → no matches.
- `grep "section-save-btn" r3-intake/internal/assets/public/index.html r3-intake/internal/assets/public/app.css` → no matches.
- `grep "R3F.saveAll" r3-intake/internal/assets/public/index.html` → 2 matches (topbar Save + finish-form Save).
- `grep -c "app.css?v=9" r3-intake/internal/assets/public/index.html` → 9.
- `grep "app.css?v=8" r3-intake/internal/assets/public/index.html` → no matches.
- `grep -A2 "finish-actions .btn-primary" r3-intake/internal/assets/public/app.css` shows both `.btn-primary` and `.btn-ghost` in the combined selector.
- `cd r3-intake && go build ./...` → exit 0.
- `cd r3-intake && go vet ./...` → exit 0.
- `cd r3-intake && go test ./...` → PASS.
- `git diff --check` → clean.

## Acceptance criteria

- [x] No "Save section" buttons remain.
- [x] Each section form still has `hx-post`/`hx-trigger` attributes.
- [x] Both Save buttons call `R3F.saveAll()`.
- [x] `R3F.saveAll()` submits all 5 sections and patches the record id on first save.
- [x] Save and New buttons render at identical size.
- [x] No regression to buttons outside `.finish-actions`.
- [x] Build/vet/test green and `git diff --check` clean.
