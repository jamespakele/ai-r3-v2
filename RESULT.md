# RESULT — t_081e1391: Update MCP tool definitions to remove claim vocabulary

Story 3 (MCP cleanup) of "Remove the Claim Feature Completely".

## What was built
Removed all claim vocabulary from the MCP server tool definitions and
implementations in `r3-intake/internal/mcp/mcp.go`:

- `list_intakes` status enum: `["unassigned","claimed","completed"]` → `["unassigned","completed"]`
- `search_intakes` status enum: same change
- Removed the `assigned_to` filter parameter from the `list_intakes` schema,
  its `listIntakesIn.AssignedTo` struct field, and the filter-building branch
  that appended `assigned_to=<id>`.
- Removed `AssignedName` from `intakeSummary` and the assigned-name population
  in `handleListIntakes` / `handleSearchIntakes`.
- Removed the `Claimed` counter from `statusCounts` (by_status) and its
  `case "claimed"` branch in `handleIntakeStats`.
- Updated `list_users` description that referenced "assigned_to resolution context".
- Removed now-dead helpers: `loadUserMap`, `resolveUser`, `userName`, `userInfo`.

Only `r3-intake/internal/mcp/mcp.go` changed (31 insertions, 105 deletions).

## Verification
- `gofmt -l internal/mcp/` → clean
- `go vet ./internal/mcp/` → OK
- `go build ./...` → OK (entire module compiles)
- `go test ./internal/mcp/` → ok (no test files)
- grep gate `claimed|assigned_to|assigned_name` across `r3-intake/internal/mcp/` → ZERO hits
