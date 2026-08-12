package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	mcpmod "r3-intake/internal/mcp"

	"github.com/pocketbase/pocketbase/core"
)

// hst is Hawaii Standard Time (UTC-10, no DST). Used for all timestamp display and
// "now" derivations so the app is correct regardless of the server's OS timezone.
var hst = time.FixedZone("HST", -10*60*60)

// IntakeRow is the admin list view of an intake record. SSN is masked to last-4.
type IntakeRow struct {
	ID           string
	Name         string
	SSNMasked    string
	SiteName     string
	Status       string
	AssignedName string
	Created      string
	CreatedByID  string
	AssignedToID string
}

// AdminView is the view model for the admin dashboard template.
type AdminView struct {
	UserName     string
	Role         string
	IsAdmin      bool
	Rows         []IntakeRow
	Sites        []Site
	Users        []UserRow
	Query        string
	StatusFilter string
	SiteFilter   string
	Total        int
}

// UserOption is a minimal user label/value for dropdowns.
type UserOption struct {
	ID   string
	Name string
}

// UserRow is the admin users list.
type UserRow struct {
	ID    string
	Email string
	Name  string
	Role  string
}

// handleList renders the intake records list (the main landing screen).
// Unauthenticated users are redirected to the public intake form.
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	u := s.currentSession(r)
	if u == nil {
		http.Redirect(w, r, "/public/intake", http.StatusSeeOther)
		return
	}
	view := &AdminView{UserName: u.Name, Role: u.Role, IsAdmin: u.Role == "admin"}

	col, err := s.intakeCollection()
	if err == nil {
		parts := []string{}
		// Status filter from ?status=
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
		if statusFilter == "unassigned" || statusFilter == "claimed" || statusFilter == "completed" {
			parts = append(parts, fmt.Sprintf("status='%s'", mcpmod.EscapeFilter(statusFilter)))
			view.StatusFilter = statusFilter
		}
		// Site filter from ?site= (value is a site record ID)
		siteFilter := strings.TrimSpace(r.URL.Query().Get("site"))
		if siteFilter != "" {
			parts = append(parts, fmt.Sprintf("site='%s'", mcpmod.EscapeFilter(siteFilter)))
			view.SiteFilter = siteFilter
		}
		// Free-text search from ?q= (min 2 chars, matching mcp.go behavior)
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(query) >= 2 {
			escaped := mcpmod.EscapeFilter(query)
			parts = append(parts, fmt.Sprintf(`(name ~ "%s" || email ~ "%s" || contact ~ "%s")`, escaped, escaped, escaped))
			view.Query = query
		}
		filter := "1=1"
		if len(parts) > 0 {
			filter = strings.Join(parts, " && ")
		}
		recs, err := s.pb.FindRecordsByFilter(col.Id, filter, "-created", 500, 0)
		if err == nil {
			siteMap := s.siteNameMap()
			userMap := s.userNameMap()
			for _, rec := range recs {
				row := IntakeRow{
					ID:           rec.Id,
					Name:         rec.GetString("name"),
					SSNMasked:    maskSSN(rec.GetString("ssn")),
					Status:       rec.GetString("status"),
					Created:      rec.GetString("created"),
					CreatedByID:  rec.GetString("created_by"),
					AssignedToID: rec.GetString("assigned_to"),
				}
				row.SiteName = siteMap[rec.GetString("site")]
				if row.SiteName == "" {
					row.SiteName = "—"
				}
				row.AssignedName = userMap[row.AssignedToID]
				if row.AssignedName == "" {
					if row.Status == "unassigned" {
						row.AssignedName = "Unassigned"
					} else {
						row.AssignedName = "—"
					}
				}
				view.Rows = append(view.Rows, row)
			}
		}
		view.Total = len(view.Rows)
	}

	view.Sites = must(s.loadSites(false))
	_ = s.tpl.ExecuteTemplate(w, "list", view)
}

// handleAdminSettings renders the admin settings page (sites + users).
// It is wrapped with requireRole("admin") in the mux.
func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	view := &AdminView{UserName: u.Name, Role: u.Role, IsAdmin: true}
	view.Sites = must(s.loadSites(true))
	view.Users = s.loadUsers()
	_ = s.tpl.ExecuteTemplate(w, "admin", view)
}

// handleAdminSub routes /admin/* mutations: site add/toggle, claim, complete,
// delete, user add. Site and user mutations are admin-only.
func (s *Server) handleAdminSub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	u := s.currentSession(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	path := strings.TrimPrefix(r.URL.Path, "/admin/")

	switch {
	case path == "sites" && u.Role == "admin":
		s.adminSiteAdd(w, r)
	case strings.HasPrefix(path, "sites/") && strings.HasSuffix(path, "/default") && u.Role == "admin":
		s.adminSiteSetDefault(w, r, path)
	case strings.HasPrefix(path, "sites/") && strings.HasSuffix(path, "/toggle") && u.Role == "admin":
		s.adminSiteToggle(w, r, path)
	case strings.HasPrefix(path, "intake/") && strings.HasSuffix(path, "/claim"):
		s.adminClaim(w, r, u)
	case strings.HasPrefix(path, "intake/") && strings.HasSuffix(path, "/complete"):
		s.adminComplete(w, r, u)
	case path == "intake/bulk-delete" && u.Role == "admin":
		s.adminBulkDelete(w, r)
	case strings.HasPrefix(path, "intake/") && strings.HasSuffix(path, "/delete") && u.Role == "admin":
		s.adminDelete(w, r)
	case path == "users" && u.Role == "admin":
		s.adminUserAdd(w, r)
	case strings.HasPrefix(path, "users/") && strings.HasSuffix(path, "/update") && u.Role == "admin":
		s.adminUserUpdate(w, r, path)
	default:
		http.NotFound(w, r)
	}
}

// adminSiteAdd creates a new site.
func (s *Server) adminSiteAdd(w http.ResponseWriter, r *http.Request) {
	col, err := s.sitesCollection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rec := core.NewRecord(col)
	rec.Set("name", strings.TrimSpace(r.FormValue("name")))
	rec.Set("address", strings.TrimSpace(r.FormValue("address")))
	rec.Set("active", true)
	rec.Set("sort_order", 99)
	_ = s.pb.Save(rec)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// adminSiteSetDefault sets one site as the default and unsets all others.
func (s *Server) adminSiteSetDefault(w http.ResponseWriter, r *http.Request, path string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "sites/"), "/default")
	col, err := s.sitesCollection()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	recs, err := s.pb.FindRecordsByFilter(col.Id, "1=1", "", 1000, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, rec := range recs {
		rec.Set("is_default", rec.Id == id)
		_ = s.pb.Save(rec)
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// adminSiteToggle flips the active flag.
func (s *Server) adminSiteToggle(w http.ResponseWriter, r *http.Request, path string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "sites/"), "/toggle")
	rec, err := s.pb.FindRecordById("sites", id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rec.Set("active", !rec.GetBool("active"))
	_ = s.pb.Save(rec)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// adminClaim assigns the intake to the current user and sets status=claimed.
func (s *Server) adminClaim(w http.ResponseWriter, r *http.Request, u *sessionUser) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/intake/"), "/claim")
	rec, err := s.findIntake(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Only claim unassigned intakes; admins can claim anything.
	if rec.GetString("status") != "unassigned" && u.Role != "admin" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	rec.Set("assigned_to", u.ID)
	rec.Set("status", "claimed")
	_ = s.saveIntake(rec)
	http.Redirect(w, r, "/intake/"+rec.Id, http.StatusSeeOther)
}

// adminComplete marks the intake completed.
func (s *Server) adminComplete(w http.ResponseWriter, r *http.Request, u *sessionUser) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/intake/"), "/complete")
	rec, err := s.findIntake(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if u.Role != "admin" && rec.GetString("assigned_to") != u.ID && rec.GetString("created_by") != u.ID {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	rec.Set("status", "completed")
	_ = s.saveIntake(rec)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// adminDelete removes an intake (admin only).
func (s *Server) adminDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/intake/"), "/delete")
	rec, err := s.findIntake(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.pb.Delete(rec)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// adminBulkDelete removes multiple intake records (admin only).
func (s *Server) adminBulkDelete(w http.ResponseWriter, r *http.Request) {
	for _, id := range r.Form["ids"] {
		if rec, err := s.findIntake(id); err == nil {
			_ = s.pb.Delete(rec)
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// adminUserAdd creates a new case_manager, intake, or admin user (admin only).
func (s *Server) adminUserAdd(w http.ResponseWriter, r *http.Request) {
	role := r.FormValue("role")
	if role != "admin" && role != "case_manager" && role != "intake" {
		role = "case_manager"
	}
	col, err := s.pb.FindCollectionByNameOrId("users")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rec := core.NewRecord(col)
	rec.SetEmail(strings.TrimSpace(r.FormValue("email")))
	rec.Set("name", strings.TrimSpace(r.FormValue("name")))
	rec.Set("role", role)
	rec.SetPassword(r.FormValue("password"))
	if err := s.pb.Save(rec); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// adminUserUpdate edits an existing user (admin only).
func (s *Server) adminUserUpdate(w http.ResponseWriter, r *http.Request, path string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "users/"), "/update")
	rec, err := s.pb.FindRecordById("users", id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if name := strings.TrimSpace(r.FormValue("name")); name != "" {
		rec.Set("name", name)
	}
	if email := strings.TrimSpace(r.FormValue("email")); email != "" {
		rec.SetEmail(email)
	}
	if role := r.FormValue("role"); role == "admin" || role == "case_manager" || role == "intake" {
		rec.Set("role", role)
	}
	if pw := r.FormValue("password"); pw != "" {
		rec.SetPassword(pw)
	}
	_ = s.pb.Save(rec)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// loadUsers returns all users for the admin users list.
func (s *Server) loadUsers() []UserRow {
	col, err := s.pb.FindCollectionByNameOrId("users")
	if err != nil {
		return nil
	}
	recs, err := s.pb.FindRecordsByFilter(col.Id, "1=1", "name", 500, 0)
	if err != nil {
		return nil
	}
	out := make([]UserRow, 0, len(recs))
	for _, r := range recs {
		out = append(out, UserRow{
			ID:    r.Id,
			Email: r.GetString("email"),
			Name:  r.GetString("name"),
			Role:  r.GetString("role"),
		})
	}
	return out
}

// siteNameMap returns {siteID: name} for all sites.
func (s *Server) siteNameMap() map[string]string {
	m := map[string]string{}
	recs, err := s.loadSites(true)
	if err != nil {
		return m
	}
	for _, st := range recs {
		m[st.ID] = st.Name
	}
	return m
}

// userNameMap returns {userID: name} for all users.
func (s *Server) userNameMap() map[string]string {
	m := map[string]string{}
	col, err := s.pb.FindCollectionByNameOrId("users")
	if err != nil {
		return m
	}
	recs, err := s.pb.FindRecordsByFilter(col.Id, "1=1", "name", 1000, 0)
	if err != nil {
		return m
	}
	for _, r := range recs {
		m[r.Id] = r.GetString("name")
		if m[r.Id] == "" {
			m[r.Id] = r.GetString("email")
		}
	}
	return m
}

// formatTime pretty-prints a PocketBase created/updated timestamp (UTC) in Hawaii time.
func formatTime(s string) string {
	t, err := time.Parse("2006-01-02 15:04:05.000Z", s)
	if err != nil {
		return s
	}
	return t.In(hst).Format("Jan 2, 2006 3:04 PM")
}
