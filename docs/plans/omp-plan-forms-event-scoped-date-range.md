# Working Plan: Ensure matrix, stat cards, and forms use event-scoped date range

## Objective

Lock in, via a template RENDER TEST, that the attendance FORMS carry and use the
event-scoped date range consistently. The matrix already auto-scopes its date
range to the selected event's start_date->end_date (implemented by sibling card
t_8863d450 in `parseMatrixFilters`, with the UI "Event dates" label by sibling
t_6cf29cb9). Because the forms already derive their from/to from the view model
(`MatrixViewData.DateFrom`/`DateTo` and `MatrixCell.From`/`To`, which are
event-scoped after the parent's change), this card adds NO production code. The
concrete deliverable is a render test that proves the walk-in panel's hidden
from/to inputs and the matrix-cell toggle forms' hidden from/to inputs reflect
the event-scoped date range.

This card is complementary to sibling t_024eea03, which unit-tests
`parseMatrixFilters` logic directly. This card tests the TEMPLATE OUTPUT that
consumes that logic.

## Constraints

- Go server + embedded PocketBase, server-rendered Go templates (single embedded
  `index.html` with `{{define}}` blocks), HTMX + Alpine.js, vanilla CSS.
- All timestamps HST (`var hst = time.FixedZone("HST", -10*60*60)` in
  attendance.go).
- White-box tests: `package server` (same package), so unexported funcs and
  structs are directly accessible.
- Tests parse the embedded template via `assets.TemplateString()`,
  `template.New("index.html").Funcs(templateFuncs()).Parse(html)`,
  `ExecuteTemplate`, then assert on output substrings. No DB/PB harness needed
  for a pure render test.
- Do NOT hardcode wall-clock "today" dates. The event-scoped dates in the view
  model are test FIXTURES (not "today"), so fixed dates are fine and expected.
- Do NOT re-implement `parseMatrixFilters` auto-scoping or the "Event dates"
  label — those belong to sibling cards and are NOT in this worktree. This card
  only adds a render test.
- No production source changes. Only a new test function in
  `internal/server/attendance_test.go`.

## File Structure

- MODIFY: `r3-intake/internal/server/attendance_test.go` — add one new render
  test function (e.g. `TestMatrixContentRenderEventScopedFormDates`) alongside
  the existing `TestMatrixContentRender`, `TestMatrixContentRenderEventRequired`,
  and `TestMatrixContentRenderDefaultsToFirstEvent`.
- No other files change. No new files.

## Implementation Notes

### What the test must assert

Build a `MatrixViewData` whose `DateFrom`/`DateTo` represent an event-scoped
span (e.g. `"2026-08-01"` / `"2026-08-31"`), with `MatrixRow` cells whose
`From`/`To` are set to the SAME event-scoped range. Render the `matrix-content`
template and assert the output contains:

1. The walk-in panel hidden inputs:
   - `<input type="hidden" name="from" value="2026-08-01">`
   - `<input type="hidden" name="to" value="2026-08-31">`
   These come from `{{.DateFrom}}` / `{{.DateTo}}` on the walk-in search and
   walk-in create forms (index.html ~L1192-1193, ~L1203-1204).
2. The matrix-cell toggle forms' hidden inputs:
   - `name="from" value="2026-08-01"` and `name="to" value="2026-08-31"`
   These come from `{{.From}}` / `{{.To}}` on each `matrix-cell` form
   (index.html ~L1269-1270).

Because the walk-in panel and the matrix-cell forms both render the same
event-scoped values, a single `matrix-content` render exercises both. Assert on
the full hidden-input substrings (including `name=` and `value=`) so the test
proves the VALUE is event-scoped, not merely that a `from`/`to` input exists.

### Test structure (mirror the existing render tests)

```go
func TestMatrixContentRenderEventScopedFormDates(t *testing.T) {
	html, err := assets.TemplateString()
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("index.html").Funcs(templateFuncs()).Parse(html)
	if err != nil {
		t.Fatal(err)
	}

	view := MatrixViewData{
		UserName: "Admin",
		Role:     "admin",
		IsAdmin:  true,
		SiteID:   "site1",
		SiteName: "Kona",
		DateFrom: "2026-08-01", // event-scoped start
		DateTo:   "2026-08-31", // event-scoped end
		Dates:    []string{"2026-08-01", "2026-08-31"},
		Rows: []MatrixRow{
			{IntakeID: "i1", Name: "Alice", TotalDays: 2,
				Cells: []MatrixCell{
					{IntakeID: "i1", Date: "2026-08-01", Status: "present",
						SiteID: "site1", From: "2026-08-01", To: "2026-08-31"},
					{IntakeID: "i1", Date: "2026-08-31", Status: "absent",
						SiteID: "site1", From: "2026-08-01", To: "2026-08-31",
						Disabled: true},
				},
				PresentCount: 1, WalkInCount: 0},
		},
		Sites:   []Site{{ID: "site1", Name: "Kona"}},
		Events:  []Event{{ID: "ev1", Name: "Morning Program"}},
		Summary: MatrixSummary{TotalCheckIns: 1, ActiveParticipants: 1, Stopped: 0, AvgRate: 50},
		EventID: "ev1",
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "matrix-content", view); err != nil {
		t.Fatalf("render matrix-content: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		`<input type="hidden" name="from" value="2026-08-01">`,
		`<input type="hidden" name="to" value="2026-08-31">`,
		`name="from" value="2026-08-01"`,
		`name="to" value="2026-08-31"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("matrix-content output missing event-scoped form date %q", want)
		}
	}
}
```

Notes on the assertions:
- The first two `want` strings (full `<input ...>` tags) match the walk-in panel
  hidden inputs, which render `{{.DateFrom}}`/`{{.DateTo}}`.
- The last two `want` strings (attribute substrings) match the matrix-cell
  forms' hidden inputs, which render `{{.From}}`/`{{.To}}`. Using the attribute
  substring (not the full tag) keeps the assertion robust to the surrounding
  `matrix-cell` form markup while still proving the value is event-scoped.
- The `Disabled: true` cell still renders the hidden from/to inputs (the
  disabled branch only swaps the dot for a span), so it exercises the same
  event-scoped values on a non-toggleable cell.

### Why no production change

`handleMatrix` already sets `view.DateFrom`/`view.DateTo` from the
`parseMatrixFilters` result (attendance.go ~L118-119), and `loadMatrixRows`
populates each `MatrixCell.From`/`To` from the same event-scoped `dates`
(attendance.go ~L388-393). The templates already bind these to the hidden
inputs. The parent's auto-scope change makes those values event-scoped; this
test locks that contract in at the template-output level.

### Edge cases / pitfalls

- The walk-in panel and matrix-cell forms are all rendered by the single
  `matrix-content` template, so one render covers all of them. Do not add a
  separate test per form.
- Do not assert on wall-clock dates. The fixture dates `2026-08-01`/`2026-08-31`
  are event-scoped test fixtures, not "today".
- Keep the test in `package server` (white-box) so `MatrixViewData`,
  `MatrixRow`, `MatrixCell`, `MatrixSummary`, `Site`, and `Event` are directly
  constructible.
- The existing render tests already prove the template parses and renders; this
  new test is additive and does not disturb them.

## Verification Criteria

- `cd r3-intake && go build ./...` passes (no production code changed, so this
  is a sanity check).
- `cd r3-intake && go vet ./...` passes.
- `cd r3-intake && go test ./internal/server/ -run TestMatrixContentRender -v`
  passes, including the new `TestMatrixContentRenderEventScopedFormDates`.
- The new test FAILS if the walk-in panel or matrix-cell hidden from/to inputs
  do not carry the event-scoped values (e.g. if a future change hardcodes a
  different range or drops the hidden inputs).
- Full suite: `cd r3-intake && go test ./...` passes.
