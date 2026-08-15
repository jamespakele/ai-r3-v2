// Package mcp implements the Model Context Protocol server for the R3 Intake app.
// It exposes read-only tools for intake records, sites, and users, applying the
// privacy mask defined in the plan (ssn → ssn_last4, dob → age, signature data
// URLs omitted). Both stdio and Streamable HTTP transports consume the server
// returned by NewServer.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"r3-intake/internal/config"
	"r3-intake/internal/crypto"
)

// hst is Hawaii Standard Time (UTC-10, no DST). Used for timestamp display and
// "now" derivations so the app is correct regardless of the server's OS timezone.
var hst = time.FixedZone("HST", -10*60*60)

const version = "0.1.0"

// Deps holds the runtime dependencies needed by the MCP server.
type Deps struct {
	PB     *pocketbase.PocketBase
	Cipher crypto.Cipher
	Cfg    config.Config
}

// NewServer builds an MCP server with all R3 Intake tools registered.
// The caller is responsible for attaching a transport (stdio or Streamable HTTP).
func NewServer(d Deps) (*mcp.Server, error) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "r3-intake",
		Version: version,
	}, nil)

	// list_intakes
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_intakes",
		Title:       "List intakes",
		Description: "List intake records with optional filters. Returns summaries with an edit_url for each record.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":      map[string]any{"type": "string", "enum": []string{"unassigned", "claimed", "completed"}, "description": "Filter by intake status"},
				"site":        map[string]any{"type": "string", "description": "Filter by event id or event name (the intake's home event; intake.site was renamed to intake.event)"},
				"assigned_to": map[string]any{"type": "string", "description": "Filter by assigned user id or email"},
				"limit":       map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "description": "Maximum records to return (default 50, max 200)"},
				"offset":      map[string]any{"type": "integer", "minimum": 0, "description": "Offset for pagination (default 0)"},
			},
		},
	}, d.handleListIntakes)

	// get_intake
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_intake",
		Title:       "Get intake",
		Description: "Return a single masked intake record by id. Sensitive fields are masked (ssn → ssn_last4, dob → age; signature data URLs are omitted).",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Intake record id"},
			},
		},
	}, d.handleGetIntake)

	// search_intakes
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_intakes",
		Title:       "Search intakes",
		Description: "Free-text search across participant name, email, and contact fields.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"query"},
			"properties": map[string]any{
				"query":  map[string]any{"type": "string", "minLength": 2, "description": "Search query (min 2 characters)"},
				"status": map[string]any{"type": "string", "enum": []string{"unassigned", "claimed", "completed"}, "description": "Optional status filter"},
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "description": "Maximum records to return (default 50, max 200)"},
			},
		},
	}, d.handleSearchIntakes)

	// intake_stats
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "intake_stats",
		Title:       "Intake stats",
		Description: "Aggregate counts of intake records by status and site, plus completions this month.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, d.handleIntakeStats)

	// list_sites
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_sites",
		Title:       "List sites",
		Description: "Return the list of intake sites for reference and filter resolution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"include_inactive": map[string]any{"type": "boolean", "description": "Include inactive sites (default false)"},
			},
		},
	}, d.handleListSites)

	// list_users
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_users",
		Title:       "List users",
		Description: "Return case managers and admins for assigned_to resolution context.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"role": map[string]any{"type": "string", "enum": []string{"admin", "case_manager"}, "description": "Filter by role"},
			},
		},
	}, d.handleListUsers)

	return srv, nil
}

// decryptSensitive mirrors the server-side decryption path so this package can
// read encrypted fields without importing internal/server and its template/proxy deps.
func (d Deps) decryptSensitive(rec *core.Record) {
	for _, f := range d.Cfg.Encryption.SensitiveFields {
		if v := rec.GetString(f); v != "" {
			if dec, err := d.Cipher.Decrypt([]byte(v)); err == nil {
				rec.Set(f, string(dec))
			}
		}
	}
}

func (d Deps) editURL(rec *core.Record) string {
	return fmt.Sprintf("%s/intake/%s", d.Cfg.MCP.BaseURL, rec.Id)
}

func ssnLast4(ssn string) string {
	ssn = strings.ReplaceAll(ssn, "-", "")
	ssn = strings.ReplaceAll(ssn, " ", "")
	if ssn == "" {
		return ""
	}
	// Keep only digits so we always return the true last-4 digits.
	var digits []rune
	for _, r := range ssn {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) == 0 {
		return ""
	}
	if len(digits) <= 4 {
		return string(digits)
	}
	return string(digits[len(digits)-4:])
}

// hstCreated converts a PocketBase UTC "created" timestamp to an ISO-8601
// string in Hawaii Standard Time (e.g. "2026-08-10 12:33:50.527 -10:00").
// Returns the input unchanged on parse failure.
func hstCreated(s string) string {
	t, err := time.Parse("2006-01-02 15:04:05.000Z", s)
	if err != nil {
		return s
	}
	return t.In(hst).Format("2006-01-02 15:04:05.000 -07:00")
}

func ageFromDOB(dob string) *int {
	dob = strings.TrimSpace(dob)
	if dob == "" {
		return nil
	}
	var t time.Time
	var err error
	for _, layout := range []string{"2006-01-02", "01/02/2006", "1/2/2006"} {
		t, err = time.Parse(layout, dob)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil
	}
	now := time.Now().In(hst)
	years := now.Year() - t.Year()
	if now.Month() < t.Month() || (now.Month() == t.Month() && now.Day() < t.Day()) {
		years--
	}
	if years < 0 {
		return nil
	}
	return &years
}

// maskRecord returns a copy of the record data with sensitive fields masked.
// The input record must already be decrypted. The returned map contains the
// record id, all non-sensitive collection fields, ssn_last4, age (or null), and
// edit_url. participantSigDataUrl and casemanagerSigDataUrl are omitted.
func (d Deps) maskRecord(rec *core.Record) map[string]any {
	out := map[string]any{"id": rec.Id}
	for k, v := range rec.FieldsData() {
		switch k {
		case "ssn":
			out["ssn_last4"] = ssnLast4(rec.GetString("ssn"))
		case "dob":
			age := ageFromDOB(rec.GetString("dob"))
			if age == nil {
				out["age"] = nil
			} else {
				out["age"] = *age
			}
		case "participantSigDataUrl", "casemanagerSigDataUrl":
			// omitted entirely
		default:
			out[k] = v
		}
	}
	out["edit_url"] = d.editURL(rec)
	return out
}

type intakeSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	SiteName     string `json:"site_name"`
	AssignedName string `json:"assigned_name"`
	Created      string `json:"created"`
	SSNLast4     string `json:"ssn_last4"`
	EditURL      string `json:"edit_url"`
}

type listIntakesIn struct {
	Status     string `json:"status"`
	Site       string `json:"site"`
	AssignedTo string `json:"assigned_to"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}

type listIntakesOut struct {
	Intakes  []intakeSummary `json:"intakes"`
	Total    int             `json:"total"`
	Returned int             `json:"returned"`
}

func (d Deps) handleListIntakes(ctx context.Context, req *mcp.CallToolRequest, in listIntakesIn) (*mcp.CallToolResult, listIntakesOut, error) {
	col, err := d.PB.FindCollectionByNameOrId("intake")
	if err != nil {
		return nil, listIntakesOut{}, fmt.Errorf("load intake collection: %w", err)
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}

	parts := []string{}
	if in.Status != "" {
		parts = append(parts, fmt.Sprintf("status='%s'", EscapeFilter(in.Status)))
	}
	if in.Site != "" {
		// intake.site was renamed to intake.event (migration 016); the filter
		// now scopes by the intake's home event.
		id, err := d.resolveEvent(in.Site)
		if err != nil {
			return nil, listIntakesOut{}, err
		}
		parts = append(parts, fmt.Sprintf("event='%s'", EscapeFilter(id)))
	}
	if in.AssignedTo != "" {
		id, err := d.resolveUser(in.AssignedTo)
		if err != nil {
			return nil, listIntakesOut{}, err
		}
		parts = append(parts, fmt.Sprintf("assigned_to='%s'", EscapeFilter(id)))
	}
	filter := "1=1"
	if len(parts) > 0 {
		filter = strings.Join(parts, " && ")
	}

	recs, err := d.PB.FindRecordsByFilter(col.Id, filter, "-created", limit, offset)
	if err != nil {
		return nil, listIntakesOut{}, fmt.Errorf("list intakes: %w", err)
	}

	sites, err := d.loadEventMap()
	if err != nil {
		return nil, listIntakesOut{}, err
	}
	users, err := d.loadUserMap()
	if err != nil {
		return nil, listIntakesOut{}, err
	}

	sums := make([]intakeSummary, 0, len(recs))
	for _, r := range recs {
		d.decryptSensitive(r)
		eventID := r.GetString("event")
		assignedID := r.GetString("assigned_to")
		sums = append(sums, intakeSummary{
			ID:           r.Id,
			Name:         r.GetString("name"),
			Status:       r.GetString("status"),
			SiteName:     sites[eventID],
			AssignedName: userName(users[assignedID]),
			Created:      hstCreated(r.GetString("created")),
			SSNLast4:     ssnLast4(r.GetString("ssn")),
			EditURL:      d.editURL(r),
		})
	}

	out := listIntakesOut{
		Intakes:  sums,
		Total:    len(sums),
		Returned: len(sums),
	}
	return nil, out, nil
}

func (d Deps) handleGetIntake(ctx context.Context, req *mcp.CallToolRequest, in struct {
	ID string `json:"id"`
}) (*mcp.CallToolResult, map[string]any, error) {
	col, err := d.PB.FindCollectionByNameOrId("intake")
	if err != nil {
		return nil, nil, fmt.Errorf("load intake collection: %w", err)
	}
	rec, err := d.PB.FindRecordById(col.Id, in.ID)
	if err != nil {
		return nil, nil, errors.New("intake not found")
	}
	d.decryptSensitive(rec)
	return nil, d.maskRecord(rec), nil
}

type searchIntakesIn struct {
	Query  string `json:"query"`
	Status string `json:"status"`
	Limit  int    `json:"limit"`
}

type searchIntakesOut struct {
	Intakes []intakeSummary `json:"intakes"`
}

func (d Deps) handleSearchIntakes(ctx context.Context, req *mcp.CallToolRequest, in searchIntakesIn) (*mcp.CallToolResult, searchIntakesOut, error) {
	q := strings.TrimSpace(in.Query)
	if len(q) < 2 {
		return nil, searchIntakesOut{}, errors.New("query must be at least 2 characters")
	}
	col, err := d.PB.FindCollectionByNameOrId("intake")
	if err != nil {
		return nil, searchIntakesOut{}, fmt.Errorf("load intake collection: %w", err)
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	escaped := EscapeFilter(q)
	filter := fmt.Sprintf(`name ~ "%s" || email ~ "%s" || contact ~ "%s"`, escaped, escaped, escaped)
	if in.Status != "" {
		filter = fmt.Sprintf("(%s) && status='%s'", filter, EscapeFilter(in.Status))
	}

	recs, err := d.PB.FindRecordsByFilter(col.Id, filter, "-created", limit, 0)
	if err != nil {
		return nil, searchIntakesOut{}, fmt.Errorf("search intakes: %w", err)
	}

	events, err := d.loadEventMap()
	if err != nil {
		return nil, searchIntakesOut{}, err
	}
	users, err := d.loadUserMap()
	if err != nil {
		return nil, searchIntakesOut{}, err
	}

	sums := make([]intakeSummary, 0, len(recs))
	for _, r := range recs {
		d.decryptSensitive(r)
		eventID := r.GetString("event")
		assignedID := r.GetString("assigned_to")
		sums = append(sums, intakeSummary{
			ID:           r.Id,
			Name:         r.GetString("name"),
			Status:       r.GetString("status"),
			SiteName:     events[eventID],
			AssignedName: userName(users[assignedID]),
			Created:      hstCreated(r.GetString("created")),
			SSNLast4:     ssnLast4(r.GetString("ssn")),
			EditURL:      d.editURL(r),
		})
	}
	return nil, searchIntakesOut{Intakes: sums}, nil
}

type statusCounts struct {
	Unassigned int `json:"unassigned"`
	Claimed    int `json:"claimed"`
	Completed  int `json:"completed"`
}

type siteCount struct {
	SiteName string `json:"site_name"`
	Count    int    `json:"count"`
}

type intakeStatsOut struct {
	Total              int          `json:"total"`
	ByStatus           statusCounts `json:"by_status"`
	BySite             []siteCount  `json:"by_site"`
	CompletedThisMonth int          `json:"completed_this_month"`
}

func (d Deps) handleIntakeStats(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, intakeStatsOut, error) {
	col, err := d.PB.FindCollectionByNameOrId("intake")
	if err != nil {
		return nil, intakeStatsOut{}, fmt.Errorf("load intake collection: %w", err)
	}
	recs, err := d.PB.FindRecordsByFilter(col.Id, "1=1", "-created", 1000, 0)
	if err != nil {
		return nil, intakeStatsOut{}, fmt.Errorf("load intakes: %w", err)
	}

	events, err := d.loadEventMap()
	if err != nil {
		return nil, intakeStatsOut{}, err
	}

	out := intakeStatsOut{
		BySite: make([]siteCount, 0),
	}
	now := time.Now().In(hst)
	bySite := map[string]int{}
	for _, r := range recs {
		out.Total++
		status := r.GetString("status")
		switch status {
		case "unassigned":
			out.ByStatus.Unassigned++
		case "claimed":
			out.ByStatus.Claimed++
		case "completed":
			out.ByStatus.Completed++
			updated := r.GetDateTime("updated").Time()
			if !updated.IsZero() && updated.In(hst).Year() == now.Year() && updated.In(hst).Month() == now.Month() {
				out.CompletedThisMonth++
			}
		}
		// Group by the intake's home event (intake.site was renamed to
		// intake.event in migration 016).
		if eid := r.GetString("event"); eid != "" {
			bySite[eid]++
		}
	}
	for eid, count := range bySite {
		out.BySite = append(out.BySite, siteCount{SiteName: events[eid], Count: count})
	}
	return nil, out, nil
}

type siteOut struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	SortOrder int    `json:"sort_order"`
}

func (d Deps) handleListSites(ctx context.Context, req *mcp.CallToolRequest, in struct {
	IncludeInactive bool `json:"include_inactive"`
}) (*mcp.CallToolResult, struct {
	Sites []siteOut `json:"sites"`
}, error) {
	col, err := d.PB.FindCollectionByNameOrId("sites")
	if err != nil {
		return nil, struct {
			Sites []siteOut `json:"sites"`
		}{}, fmt.Errorf("load sites collection: %w", err)
	}
	filter := "active = true"
	if in.IncludeInactive {
		filter = "1=1"
	}
	recs, err := d.PB.FindRecordsByFilter(col.Id, filter, "sort_order,name", 1000, 0)
	if err != nil {
		return nil, struct {
			Sites []siteOut `json:"sites"`
		}{}, fmt.Errorf("list sites: %w", err)
	}
	out := make([]siteOut, 0, len(recs))
	for _, r := range recs {
		out = append(out, siteOut{
			ID:        r.Id,
			Name:      r.GetString("name"),
			Active:    r.GetBool("active"),
			SortOrder: r.GetInt("sort_order"),
		})
	}
	return nil, struct {
		Sites []siteOut `json:"sites"`
	}{Sites: out}, nil
}

type userOut struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (d Deps) handleListUsers(ctx context.Context, req *mcp.CallToolRequest, in struct {
	Role string `json:"role"`
}) (*mcp.CallToolResult, struct {
	Users []userOut `json:"users"`
}, error) {
	col, err := d.PB.FindCollectionByNameOrId("users")
	if err != nil {
		return nil, struct {
			Users []userOut `json:"users"`
		}{}, fmt.Errorf("load users collection: %w", err)
	}
	filter := "1=1"
	if in.Role != "" {
		filter = fmt.Sprintf("role='%s'", EscapeFilter(in.Role))
	}
	recs, err := d.PB.FindRecordsByFilter(col.Id, filter, "name", 1000, 0)
	if err != nil {
		return nil, struct {
			Users []userOut `json:"users"`
		}{}, fmt.Errorf("list users: %w", err)
	}
	out := make([]userOut, 0, len(recs))
	for _, r := range recs {
		out = append(out, userOut{
			ID:    r.Id,
			Name:  r.GetString("name"),
			Email: r.GetString("email"),
			Role:  r.GetString("role"),
		})
	}
	return nil, struct {
		Users []userOut `json:"users"`
	}{Users: out}, nil
}

// --- resolution helpers ---------------------------------------------------

// loadEventMap returns {eventID: name} for all non-deleted events. Intake
// records reference their home event (migration 016 renamed intake.site to
// intake.event), so event names replace site names in intake summaries.
func (d Deps) loadEventMap() (map[string]string, error) {
	col, err := d.PB.FindCollectionByNameOrId("events")
	if err != nil {
		return nil, err
	}
	recs, err := d.PB.FindRecordsByFilter(col.Id, "(deleted = false || deleted = null)", "name", 1000, 0)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(recs))
	for _, r := range recs {
		m[r.Id] = r.GetString("name")
	}
	return m, nil
}

type userInfo struct {
	Name  string
	Email string
	Role  string
}

func (d Deps) loadUserMap() (map[string]userInfo, error) {
	col, err := d.PB.FindCollectionByNameOrId("users")
	if err != nil {
		return nil, err
	}
	recs, err := d.PB.FindRecordsByFilter(col.Id, "1=1", "name", 1000, 0)
	if err != nil {
		return nil, err
	}
	m := make(map[string]userInfo, len(recs))
	for _, r := range recs {
		m[r.Id] = userInfo{
			Name:  r.GetString("name"),
			Email: r.GetString("email"),
			Role:  r.GetString("role"),
		}
	}
	return m, nil
}

// resolveEvent resolves an event name or id to an event id. The intake list
// tools' site filter now scopes by the intake's home event (migration 016
// renamed intake.site to intake.event), so the input resolves against events.
func (d Deps) resolveEvent(nameOrID string) (string, error) {
	// Fast path: already an id.
	if r, err := d.PB.FindRecordById("events", nameOrID); err == nil {
		return r.Id, nil
	}
	// Resolve by name.
	r, err := d.PB.FindFirstRecordByData("events", "name", nameOrID)
	if err != nil {
		return "", fmt.Errorf("event not found: %s", nameOrID)
	}
	return r.Id, nil
}

func (d Deps) resolveUser(emailOrID string) (string, error) {
	if r, err := d.PB.FindRecordById("users", emailOrID); err == nil {
		return r.Id, nil
	}
	r, err := d.PB.FindFirstRecordByData("users", "email", emailOrID)
	if err != nil {
		return "", fmt.Errorf("user not found: %s", emailOrID)
	}
	return r.Id, nil
}

func userName(u userInfo) string {
	if u.Name != "" {
		return u.Name
	}
	if u.Email != "" {
		return u.Email
	}
	return ""
}

func EscapeFilter(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
