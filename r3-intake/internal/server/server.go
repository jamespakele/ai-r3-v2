// Package server wires the R3 Intake HTTP layer: static assets, Go templates,
// the /api/* and (under --admin) /_/ reverse proxies to the in-process
// PocketBase, and the custom handlers for the form, sections, admin, and auth.
//
// All PocketBase data access goes through pb.Dao() in-process — the browser
// never talks to PocketBase directly. The /api/* and /_/ proxies exist so the
// PB admin UI (dev-only, under --admin) and any direct PB API use still work.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"r3-intake/internal/assets"
	"r3-intake/internal/config"
	"r3-intake/internal/crypto"
	mcpmod "r3-intake/internal/mcp"
)

// Server holds runtime dependencies shared by all handlers.
type Server struct {
	cfg        config.Config
	pb         *pocketbase.PocketBase
	tpl        *template.Template
	cipher     crypto.Cipher
	pbProxy    *httputil.ReverseProxy
	assets     http.FileSystem
	mcpHandler http.Handler
}

// New builds the Server: parses embedded templates, sets up the PB reverse
// proxy, and prepares the static-asset FS from the embedded public/ files.
func New(cfg config.Config, pb *pocketbase.PocketBase) (*Server, error) {
	html, err := assets.TemplateString()
	if err != nil {
		return nil, err
	}
	tpl, err := template.New("index.html").
		Funcs(templateFuncs()).
		Parse(html)
	if err != nil {
		return nil, err
	}

	pbURL, err := url.Parse("http://" + strings.TrimPrefix(cfg.PBInternalAddr, "http://"))
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(pbURL)

	// Static assets served from the embedded FS.
	assetsFS := assets.HTTPFileSystem()

	s := &Server{
		cfg:     cfg,
		pb:      pb,
		tpl:     tpl,
		cipher:  crypto.NewPlainCipher(), // config.Encryption.Enabled=false by default
		pbProxy: proxy,
		assets:  assetsFS,
	}

	if cfg.MCP.Token != "" {
		mcpSrv, err := mcpmod.NewServer(mcpmod.Deps{
			PB:     pb,
			Cipher: s.cipher,
			Cfg:    cfg,
		})
		if err != nil {
			return nil, err
		}
		s.mcpHandler = mcp.NewStreamableHTTPHandler(
			func(r *http.Request) *mcp.Server { return mcpSrv },
			&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
		)
	}

	return s, nil
}

// Mux assembles the full HTTP router.
func (s *Server) Mux() http.Handler {
	mux := http.NewServeMux()

	// Main screen: List (authed) or redirect to public form
	mux.HandleFunc("/", s.handleList)
	// Public intake form (no auth)
	mux.HandleFunc("/public/intake", s.handlePublicIntake)
	// Intake form view/edit + finish
	mux.HandleFunc("/intake/", s.handleIntakeCmd)
	// Section autosave
	mux.HandleFunc("/section/", s.handleSection)
	// Site fragment (htmx)
	mux.HandleFunc("/sites", s.handleSites)

	// Auth
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)

	// Admin settings (sites + users) — admin only
	mux.HandleFunc("/admin", s.requireRole("admin", s.handleAdminSettings))
	// Admin mutations (per-action role checks inside)
	mux.HandleFunc("/admin/", s.requireAuth(s.handleAdminSub))
	// Event enrollment-management placeholder — admin only
	mux.HandleFunc("/admin/events/", s.requireRole("admin", s.handleAdminEventManage))
	// Story 2.2 enrollment routes
	mux.HandleFunc("/admin/events/{id}/enroll", s.requireRole("admin", s.handleEventEnroll))
	mux.HandleFunc("/admin/events/{id}/unenroll", s.requireRole("admin", s.handleEventUnenroll))
	mux.HandleFunc("/admin/events/{id}/enroll-search", s.requireRole("admin", s.handleEnrollSearch))
	// Story 2.3 lifecycle routes
	mux.HandleFunc("POST /admin/events/{id}/status", s.requireRole("admin", s.handleEventStatus))
	// Report placeholder — admin only
	mux.HandleFunc("GET /admin/events/{id}/report", s.requireRole("admin", s.handleEventReport))

	// Duplicate search (auth-only)
	mux.HandleFunc("/search/duplicates", s.requireAuth(s.handleDuplicateSearch))

	// Per-participant notes screen + add (auth-only)
	mux.HandleFunc("/notes/", s.requireAuth(s.handleNotes))

	// Attendance matrix (auth-only)
	mux.HandleFunc("/attendance", s.requireAuth(s.handleMatrix))
	// CSV export (FR14) — admin only. PRD §10 marks Export CSV as admin-only
	// (✓ admin, ✗ case_manager), overriding the generic auth note in §09.
	mux.HandleFunc("GET /attendance/export", s.requireRole("admin", s.handleExportCSV))
	mux.HandleFunc("/attendance/toggle", s.requireAuth(s.handleToggle))
	mux.HandleFunc("/attendance/walkin-search", s.requireAuth(s.handleWalkinSearch))
	mux.HandleFunc("/attendance/walkin", s.requireAuth(s.handleWalkin))

	// PB admin UI — only when --admin / R3_ADMIN=1
	mux.HandleFunc("/_/", s.handlePBAdmin)

	// PB API proxy (always on; collection rules lock it to admin/superuser)
	mux.HandleFunc("/api/", s.proxyPB)
	mux.Handle("/pb_files/", http.StripPrefix("/pb_files/", http.FileServer(http.Dir(s.cfg.PBRootDir+"/pb_data"))))

	// MCP read-only endpoint (only when R3_MCP_TOKEN is set)
	mux.HandleFunc("/mcp", s.handleMCP)

	// Static assets (everything in public/ except index.html, which / renders)
	mux.HandleFunc("/static/", s.serveStatic)

	return mux
}

// proxyPB forwards /api/* to the in-process PocketBase on :8091.
func (s *Server) proxyPB(w http.ResponseWriter, r *http.Request) {
	s.pbProxy.ServeHTTP(w, r)
}

// handleMCP serves the MCP Streamable HTTP endpoint when MCP is enabled and
// the Authorization header carries the configured bearer token.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if s.cfg.MCP.Token == "" || s.mcpHandler == nil {
		http.NotFound(w, r)
		return
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.MCP.Token)) != 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.mcpHandler.ServeHTTP(w, r)
}

// handlePBAdmin forwards /_/... to PocketBase's admin UI, or 404 when not in
// --admin mode.
func (s *Server) handlePBAdmin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.ExposePBAdmin {
		http.NotFound(w, r)
		return
	}
	s.pbProxy.ServeHTTP(w, r)
}

// serveStatic serves a file from the embedded public/ FS by its basename,
// skipping index.html. Embedded files have no real modtime, so a stable time
// is used for cache-friendly Last-Modified headers.
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/static/")
	if p == "" || p == "index.html" {
		http.NotFound(w, r)
		return
	}
	f, info, err := assets.OpenReadSeeker(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, info.Name(), assets.StaticModTime(), f)
}

// --- PocketBase repository helpers (in-process via the embedded core.App) ---

// intakeCollection returns the intake collection, creating a blank record shell.
func (s *Server) intakeCollection() (*core.Collection, error) {
	return s.pb.FindCollectionByNameOrId("intake")
}

// sitesCollection returns the sites collection.
func (s *Server) sitesCollection() (*core.Collection, error) {
	return s.pb.FindCollectionByNameOrId("sites")
}

// notesCollection returns the notes collection.
func (s *Server) notesCollection() (*core.Collection, error) {
	return s.pb.FindCollectionByNameOrId("notes")
}

// eventEnrollmentCollection returns the event_enrollment junction collection.
func (s *Server) eventEnrollmentCollection() (*core.Collection, error) {
	return s.pb.FindCollectionByNameOrId("event_enrollment")
}

// Site is a flat view of a sites record for templates.
type Site struct {
	ID        string
	Name      string
	Address   string
	Active    bool
	SortOrder int
	Default   bool
}

// loadSites returns active sites sorted by sort_order then name. When
// includeInactive is true, all sites are returned (admin views).
func (s *Server) loadSites(includeInactive bool) ([]Site, error) {
	col, err := s.sitesCollection()
	if err != nil {
		return nil, err
	}
	filter := "active = true"
	if includeInactive {
		filter = "1=1"
	}
	recs, err := s.pb.FindRecordsByFilter(col.Id, filter, "sort_order,name", 1000, 0)
	if err != nil {
		return nil, err
	}
	out := make([]Site, 0, len(recs))
	for _, r := range recs {
		out = append(out, Site{
			ID:        r.Id,
			Name:      r.GetString("name"),
			Address:   r.GetString("address"),
			Active:    r.GetBool("active"),
			SortOrder: r.GetInt("sort_order"),
			Default:   r.GetBool("is_default"),
		})
	}
	return out, nil
}

// findIntake loads an intake record by id and decrypts sensitive fields in
// place via the configured Cipher (PlainCipher passes through when encryption
// is disabled).
func (s *Server) findIntake(id string) (*core.Record, error) {
	col, err := s.intakeCollection()
	if err != nil {
		return nil, err
	}
	rec, err := s.pb.FindRecordById(col.Id, id)
	if err != nil {
		return nil, err
	}
	s.decryptSensitive(rec)
	return rec, nil
}

// newIntakeRecord creates a fresh intake record (not yet saved).
func (s *Server) newIntakeRecord() (*core.Record, error) {
	col, err := s.intakeCollection()
	if err != nil {
		return nil, err
	}
	rec := core.NewRecord(col)
	rec.Set("status", "unassigned")
	rec.Set("household", []map[string]string{{"name": "", "relationship": ""}})
	rec.Set("race", map[string]bool{})
	rec.Set("documents", map[string]bool{})
	rec.Set("housing", map[string]bool{})
	rec.Set("income", map[string]bool{})
	rec.Set("personal", []string{"", "", "", "", "", "", "", ""})
	rec.Set("servicePlan", []string{"", "", "", "", "", "", "", ""})
	return rec, nil
}

// saveIntake persists an intake record. Sensitive fields are encrypted via the
// configured Cipher before save and restored to plaintext in memory after
// (PlainCipher passes through, so this is a no-op until encryption is enabled).
func (s *Server) saveIntake(rec *core.Record) error {
	s.encryptSensitive(rec)
	err := s.pb.Save(rec)
	s.decryptSensitive(rec)
	return err
}

// encryptSensitive transforms each configured SensitiveField in place.
func (s *Server) encryptSensitive(rec *core.Record) {
	for _, f := range s.cfg.Encryption.SensitiveFields {
		v := rec.GetString(f)
		if v == "" {
			continue
		}
		if enc, err := s.cipher.Encrypt([]byte(v)); err == nil {
			rec.Set(f, string(enc))
		}
	}
}

// decryptSensitive reverses encryptSensitive in place.
func (s *Server) decryptSensitive(rec *core.Record) {
	for _, f := range s.cfg.Encryption.SensitiveFields {
		v := rec.GetString(f)
		if v == "" {
			continue
		}
		if dec, err := s.cipher.Decrypt([]byte(v)); err == nil {
			rec.Set(f, string(dec))
		}
	}
}

// maskSSN returns the last-4 mask for admin list views.
func maskSSN(ssn string) string {
	ssn = strings.ReplaceAll(ssn, "-", "")
	if len(ssn) <= 4 {
		return ssn
	}
	return "***-**-" + ssn[len(ssn)-4:]
}

// jsonBytes normalizes a PocketBase JSON field value into raw JSON bytes.
// PB's JsonField hands back types.JSONRaw (a named []byte) from rec.Get; it
// can also be a parsed map/slice or a string. Marshal handles JSONRaw (its
// MarshalJSON returns the raw bytes) and parsed Go values; string/[]byte are
// taken as-is.
func jsonBytes(v any) ([]byte, bool) {
	if v == nil {
		return nil, false
	}
	switch t := v.(type) {
	case string:
		return []byte(t), true
	case []byte:
		return t, true
	case types.JSONRaw:
		return []byte(t), true
	}
	b, err := json.Marshal(v)
	return b, err == nil
}

// jsonFieldString returns a JSON field as a JSON string (for Alpine x-data).
func jsonFieldString(rec *core.Record, key, fallback string) string {
	b, ok := jsonBytes(rec.Get(key))
	if !ok || len(b) == 0 {
		return fallback
	}
	return string(b)
}

// asStringSlice coerces a JSON field to a fixed-length []string.
func asStringSlice(rec *core.Record, key string, n int) []string {
	out := make([]string, n)
	b, ok := jsonBytes(rec.Get(key))
	if !ok || len(b) == 0 {
		return out
	}
	var arr []string
	if json.Unmarshal(b, &arr) == nil {
		for i := 0; i < n && i < len(arr); i++ {
			out[i] = arr[i]
		}
	}
	return out
}

// asBoolMap coerces a JSON field to map[string]bool. Always returns a
// non-nil map (a "null" JSON field unmarshals to nil, which the template's
// `index $.Race` and anyBool would mishandle).
func asBoolMap(rec *core.Record, key string) map[string]bool {
	out := map[string]bool{}
	b, ok := jsonBytes(rec.Get(key))
	if !ok || len(b) == 0 {
		return out
	}
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = map[string]bool{}
	}
	return out
}
