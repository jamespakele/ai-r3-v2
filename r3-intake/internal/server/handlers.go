package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"

	mcpmod "r3-intake/internal/mcp"
)

// Canonical option lists — mirror the reference exactly.
var (
	RACE_LIST = [][2]string{
		{"chinese", "Chinese"}, {"nativeHawaiian", "Native Hawaiian"}, {"white", "White"},
		{"japanese", "Japanese"}, {"tongan", "Tongan"}, {"blackAA", "Black or African American"},
		{"korean", "Korean"}, {"samoan", "Samoan"}, {"americanIndian", "American Indian or Alaska Native"},
		{"filipino", "Filipino"}, {"marshallese", "Marshallese"}, {"hispanic", "Hispanic"},
		{"vietnamese", "Vietnamese"}, {"micronesian", "Micronesian"}, {"other", "Other"},
	}
	DOC_LIST = [][2]string{
		{"birthCert", "Birth Certificate"}, {"stateId", "State ID or Driver's License"},
		{"ssnCard", "Social Security Card"}, {"background", "Background Clearance"},
		{"courtOutreach", "Community Outreach Court"}, {"medical", "Medical"},
		{"healthInsurance", "Health Insurance"},
	}
	HOUSING_LIST = [][2]string{
		{"shelter", "Willing to go to shelter"}, {"ehv", "Emergency Housing Voucher (EHV)"},
		{"section8", "Section 8"}, {"housingApp", "Application to housing"},
	}
	INCOME_LIST = [][2]string{
		{"work", "Work / Employment"}, {"foodStamps", "Food Stamps / EBT / SNAP / Welfare"},
		{"ssi", "Supplemental Security Income (SSI)"}, {"ssdi", "SSDI (Social Security Disability Insurance)"},
	}
	PERSONAL_QUESTIONS = []string{
		"Is there anything specific you need right now?",
		"Is there someone you trust or rely on for support?",
		"Would you like to share your story?",
		"Are there any resources or services you are aware of that can be helpful?",
		"Have you experienced issues with any services?",
		"How can the community better support individuals facing homelessness in Wai\u02bbanae?",
		"Do you have any specific skills or talents?",
		"How can we help you right now?",
	}
	REQUIRED_FIELDS = []string{
		"event", "name", "dob", "contact", "race", "sexAtBirth",
		"servedMilitary", "hasPets", "employment", "mentalHealth",
		"substanceUse", "fleeingViolence",
	}
)

// FormState is the view model consumed by public/index.html.
type FormState struct {
	ID        string
	HasRecord bool
	Sites     []Site
	Events    []Event // active events for the intake dropdown
	EventSel  string
	UserName  string
	Role      string
	IsAuthed  bool

	// 01
	Name, DOB, SSN, Contact, Email                               string
	LivingWith                                                   string
	HouseholdJSON                                                string
	Household                                                    []HouseholdRow
	SleptWhere                                                   string
	Race                                                         map[string]bool
	RaceOther                                                    string
	SexAtBirth                                                   string
	ServedMilitary, MilitaryDetail                               string
	HasPets, PetSupport, PetPrevented                            string
	Employment, UnemployedDuration, InterestedEmployed, JobTypes string

	// 02
	MentalHealth, SubstanceUse, FleeingViolence, HomelessFactors string

	// 03
	HMIS                  bool
	HMISProvider          string
	Documents             map[string]bool
	HealthInsuranceDetail string
	Housing               map[string]bool
	Income                map[string]bool
	CasemanagerName       string
	CaseManagers          []UserOption

	// 04 / 05
	Personal    []string
	ServicePlan []string

	Status string

	// view
	Errors         map[string]bool
	Validated      bool
	ShowSuccess    bool
	ShowIncomplete bool
}

// sessionUser is the authenticated user (if any) attached to a request.
type sessionUser struct {
	ID     string
	Email  string
	Name   string
	Role   string
	Issued int64 // unix seconds when the session was created
}

// canAccessIntake returns true when u is allowed to view or mutate rec.
// Admins may access any intake; other users may only touch records they
// created or are assigned to.
func canAccessIntake(rec *core.Record, u *sessionUser) bool {
	if u == nil {
		return false
	}
	if u.Role == "admin" {
		return true
	}
	return rec.GetString("created_by") == u.ID || rec.GetString("assigned_to") == u.ID
}

// templateFuncs exposes the canonical lists + helpers to the template.
func templateFuncs() map[string]any {
	return map[string]any{
		"raceList":       func() [][2]string { return RACE_LIST },
		"docList":        func() [][2]string { return DOC_LIST },
		"housingList":    func() [][2]string { return HOUSING_LIST },
		"incomeList":     func() [][2]string { return INCOME_LIST },
		"personalQ":      func() []string { return PERSONAL_QUESTIONS },
		"requiredFields": func() []string { return REQUIRED_FIELDS },
		"add1":           func(i int) int { return i + 1 },
		"formatTime":     formatTime,
		"fmtPhone":       fmtPhone,
		"fmtDob":         fmtDob,
		"ssnLast4":       ssnLast4,
		"now":            func() string { return time.Now().In(hst).Format("3:04 PM") },
	}
}

// Progress returns the percent of required fields filled (0–100), for the
// header progress bar.
func (st *FormState) Progress() int {
	filled := 0
	if st.EventSel != "" {
		filled++
	}
	if st.Name != "" {
		filled++
	}
	if st.DOB != "" {
		filled++
	}
	if st.Contact != "" {
		filled++
	}
	if anyBool(st.Race) {
		filled++
	}
	if st.SexAtBirth != "" {
		filled++
	}
	if st.ServedMilitary != "" {
		filled++
	}
	if st.HasPets != "" {
		filled++
	}
	if st.Employment != "" {
		filled++
	}
	if st.MentalHealth != "" {
		filled++
	}
	if st.SubstanceUse != "" {
		filled++
	}
	if st.FleeingViolence != "" {
		filled++
	}
	return filled * 100 / len(REQUIRED_FIELDS)
}

// loadCaseManagers returns case_manager users sorted by name for the dropdown.
func (s *Server) loadCaseManagers() []UserOption {
	col, err := s.pb.FindCollectionByNameOrId("users")
	if err != nil {
		return nil
	}
	recs, err := s.pb.FindRecordsByFilter(col.Id, "role='case_manager' && deleted=false", "name", 1000, 0)
	if err != nil {
		return nil
	}
	out := make([]UserOption, 0, len(recs))
	for _, r := range recs {
		out = append(out, UserOption{
			ID:   r.Id,
			Name: r.GetString("name"),
		})
	}
	return out
}

// firstEventID returns the first active event's ID, or "" when there are no
// active events. Used to default the intake's home event on both new and
// edited forms.
func firstEventID(events []Event) string {
	if len(events) == 0 {
		return ""
	}
	return events[0].ID
}

// blankState builds a fresh, unsaved FormState.
func (s *Server) blankState(user *sessionUser) *FormState {
	sites := must(s.loadSites(false))
	events := must(s.loadEvents())
	st := &FormState{
		Sites:         sites,
		Events:        events,
		CaseManagers:  s.loadCaseManagers(),
		Race:          map[string]bool{},
		Documents:     map[string]bool{},
		Housing:       map[string]bool{},
		Income:        map[string]bool{},
		Personal:      []string{"", "", "", "", "", "", "", ""},
		ServicePlan:   []string{"", "", "", "", "", "", "", ""},
		HouseholdJSON: `[{"name":"","relationship":""}]`,
		Household:     []HouseholdRow{{Name: "", Relationship: ""}},
		Status:        "unassigned",
		Errors:        map[string]bool{},
	}
	// Default the intake's home event to the first active event. There is no
	// "default event" concept (unlike the old default site), so pick the
	// first active, non-deleted event.
	st.EventSel = firstEventID(events)
	if user != nil {
		st.IsAuthed = true
		st.Role = user.Role
		st.UserName = user.Name
		st.CasemanagerName = user.Name
	}
	return st
}

// stateFromRecord builds a FormState from a saved intake record.
func (s *Server) stateFromRecord(rec *core.Record, user *sessionUser, errors map[string]bool, validated bool) *FormState {
	st := &FormState{
		ID:                    rec.Id,
		HasRecord:             true,
		Sites:                 must(s.loadSites(false)),
		Events:                must(s.loadEvents()),
		CaseManagers:          s.loadCaseManagers(),
		EventSel:              rec.GetString("event"),
		Name:                  rec.GetString("name"),
		DOB:                   rec.GetString("dob"),
		SSN:                   rec.GetString("ssn"),
		Contact:               rec.GetString("contact"),
		Email:                 rec.GetString("email"),
		LivingWith:            rec.GetString("livingWith"),
		HouseholdJSON:         jsonFieldString(rec, "household", `[{"name":"","relationship":""}]`),
		Household:             parseHouseholdRows(rec),
		SleptWhere:            rec.GetString("sleptWhere"),
		Race:                  asBoolMap(rec, "race"),
		RaceOther:             rec.GetString("raceOther"),
		SexAtBirth:            rec.GetString("sexAtBirth"),
		ServedMilitary:        rec.GetString("servedMilitary"),
		MilitaryDetail:        rec.GetString("militaryDetail"),
		HasPets:               rec.GetString("hasPets"),
		PetSupport:            rec.GetString("petSupport"),
		PetPrevented:          rec.GetString("petPrevented"),
		Employment:            rec.GetString("employment"),
		UnemployedDuration:    rec.GetString("unemployedDuration"),
		InterestedEmployed:    rec.GetString("interestedEmployed"),
		JobTypes:              rec.GetString("jobTypes"),
		MentalHealth:          rec.GetString("mentalHealth"),
		SubstanceUse:          rec.GetString("substanceUse"),
		FleeingViolence:       rec.GetString("fleeingViolence"),
		HomelessFactors:       rec.GetString("homelessFactors"),
		HMIS:                  rec.GetBool("hmis"),
		HMISProvider:          rec.GetString("hmisProvider"),
		Documents:             asBoolMap(rec, "documents"),
		HealthInsuranceDetail: rec.GetString("healthInsuranceDetail"),
		Housing:               asBoolMap(rec, "housing"),
		Income:                asBoolMap(rec, "income"),
		Personal:              asStringSlice(rec, "personal", 8),
		ServicePlan:           asStringSlice(rec, "servicePlan", 8),
		CasemanagerName:       rec.GetString("casemanagerName"),
		Status:                rec.GetString("status"),
		Errors:                errors,
		Validated:             validated,
	}
	if st.Race == nil {
		st.Race = map[string]bool{}
	}
	if st.Documents == nil {
		st.Documents = map[string]bool{}
	}
	if st.Housing == nil {
		st.Housing = map[string]bool{}
	}
	if st.Income == nil {
		st.Income = map[string]bool{}
	}
	// Records without a stored event render the same default as a new form:
	// the first active event. Display-only; the stored record is untouched.
	if st.EventSel == "" {
		st.EventSel = firstEventID(st.Events)
	}
	if user != nil {
		st.IsAuthed = true
		st.Role = user.Role
		st.UserName = user.Name
	}
	if validated {
		if len(errors) == 0 {
			st.ShowSuccess = true
		} else {
			st.ShowIncomplete = true
		}
	}
	return st
}

func must[T any](v T, err error) T {
	if err != nil {
		return v // degrade gracefully; empty slice is fine for rendering
	}
	return v
}

// currentSession returns the authenticated user from the cookie, or nil.
func (s *Server) currentSession(r *http.Request) *sessionUser {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}
	return s.parseSession(c.Value)
}

// handleIndex renders the full form. GET /  or  GET /intake/{id}.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	user := s.currentSession(r)

	if r.URL.Path == "/" || r.URL.Path == "/public/intake" {
		_ = s.tpl.ExecuteTemplate(w, "page", s.blankState(user))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/intake/") {
		id := strings.TrimPrefix(r.URL.Path, "/intake/")
		rec, err := s.findIntake(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_ = s.tpl.ExecuteTemplate(w, "page", s.stateFromRecord(rec, user, map[string]bool{}, false))
		return
	}
	http.NotFound(w, r)
}

// handlePublicIntake renders the public form. When the URL carries an `id`
// query param (e.g. after first section save), it loads the saved record so
// the participant can resume an anonymous submission.
func (s *Server) handlePublicIntake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	user := s.currentSession(r)
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		_ = s.tpl.ExecuteTemplate(w, "page", s.blankState(user))
		return
	}
	rec, err := s.findIntake(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Public resume is only allowed for unassigned/unclaimed records created
	// anonymously (created_by empty). Once a case manager claims the record,
	// auth is required.
	if rec.GetString("created_by") != "" || rec.GetString("status") == "claimed" {
		http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
		return
	}
	_ = s.tpl.ExecuteTemplate(w, "page", s.stateFromRecord(rec, user, map[string]bool{}, false))
}

// handleSites returns just the site radio fragment (for optional htmx refresh).
func (s *Server) handleSites(w http.ResponseWriter, r *http.Request) {
	st := &FormState{Sites: must(s.loadSites(false))}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tpl.ExecuteTemplate(w, "site-fragment", st)
}

// handleSection persists one numbered section and returns its fragment (htmx)
// or a full-page redirect (no-JS fallback).
func (s *Server) handleSection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	section := strings.TrimPrefix(r.URL.Path, "/section/")
	user := s.currentSession(r)
	wasNew := strings.TrimSpace(r.FormValue("id")) == ""
	rec, err := s.getOrCreateIntake(r, user)
	if err != nil {
		http.Error(w, "intake load/create failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !wasNew && !canAccessIntake(rec, user) {
		http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
		return
	}
	if err := s.applySection(rec, r, section); err != nil {
		http.Error(w, "section apply failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	if section == "01" {
		missing := s.validateRecord(rec)
		if len(missing) > 0 {
			writeValidationErrors(w, requiredFieldMessages(missing))
			return
		}
	}
	if err := s.saveIntake(rec); err != nil {
		http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// First save on a brand-new form: every section form + the finish form
	// carry the (previously empty) id, so a fragment swap would leave them
	// stale and the next section save would create a duplicate record. Public
	// users must be redirected to the public resume path (/public/intake?id=...)
	// so they are not blocked by the auth gate on /intake/{id}. Authenticated
	// users continue to use /intake/{id}.
	resumeURL := "/public/intake?id=" + rec.Id
	if user != nil {
		resumeURL = "/intake/" + rec.Id
	}
	if wasNew {
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", resumeURL)
			w.WriteHeader(http.StatusAccepted)
			return
		}
		http.Redirect(w, r, resumeURL, http.StatusSeeOther)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// No-JS fallback: full page reload showing saved state.
	http.Redirect(w, r, resumeURL, http.StatusSeeOther)
}

// handleIntakeCmd handles POST /intake/{id}/finish — full validation.
func (s *Server) handleIntakeCmd(w http.ResponseWriter, r *http.Request) {
	// GET /intake/{id} → render the saved form (the /intake/ subtree outranks
	// the / handler in net/http, so view routes live here too).
	if r.Method == http.MethodGet {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/intake/"), "/")
		if id == "" {
			_ = s.tpl.ExecuteTemplate(w, "page", s.blankState(s.currentSession(r)))
			return
		}
		rec, err := s.findIntake(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		user := s.currentSession(r)
		if !canAccessIntake(rec, user) {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}
		_ = s.tpl.ExecuteTemplate(w, "page", s.stateFromRecord(rec, user, map[string]bool{}, false))
		return
	}
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/intake/")
	parts := strings.SplitN(p, "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	user := s.currentSession(r)
	var rec *core.Record
	var err error
	if parts[0] == "new" {
		// Brand-new form: no record exists yet. The "Save Form" button on a
		// fresh form posts to /intake/new/finish — create the record first
		// (reads the hidden empty id → new intake), then validate/render it
		// just like a saved record. Without this, a fresh form's Save Form
		// button 404s because findIntake("new") finds nothing.
		rec, err = s.getOrCreateIntake(r, user)
		if err != nil {
			http.Error(w, "intake load/create failed: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		rec, err = s.findIntake(parts[0])
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	switch parts[1] {
	case "finish":
		if !canAccessIntake(rec, user) {
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}
		st := s.stateFromRecord(rec, user, s.validateRecord(rec), true)
		_ = s.tpl.ExecuteTemplate(w, "page", st)
	case "cancel":
		if canAccessIntake(rec, user) {
			_ = s.pb.Delete(rec)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		http.NotFound(w, r)
	}
	return
}

// getOrCreateIntake loads the intake by the hidden "id" form field, or creates
// a new one (status unassigned, or claimed + assigned_to = authed user).
func (s *Server) getOrCreateIntake(r *http.Request, user *sessionUser) (*core.Record, error) {
	id := strings.TrimSpace(r.FormValue("id"))
	if id != "" {
		if rec, err := s.findIntake(id); err == nil {
			return rec, nil
		}
	}
	rec, err := s.newIntakeRecord()
	if err != nil {
		return nil, err
	}
	if user != nil {
		rec.Set("created_by", user.ID)
		rec.Set("assigned_to", user.ID)
		rec.Set("status", "claimed")
	}
	return rec, nil
}

// applySection writes a section's submitted form values onto the record.
func (s *Server) applySection(rec *core.Record, r *http.Request, section string) error {
	_ = r.ParseMultipartForm(10 << 20)
	switch section {
	case "01":
		rec.Set("event", r.FormValue("event"))
		first := strings.TrimSpace(r.FormValue("first_name"))
		last := strings.TrimSpace(r.FormValue("last_name"))
		if first != "" || last != "" {
			rec.Set("name", strings.TrimSpace(first+" "+last))
		} else {
			rec.Set("name", r.FormValue("name"))
		}
		rec.Set("dob", normalizeDob(r.FormValue("dob")))
		rec.Set("ssn", ssnLast4(r.FormValue("ssn")))
		rec.Set("contact", fmtPhone(r.FormValue("contact")))
		rec.Set("email", r.FormValue("email"))
		rec.Set("livingWith", r.FormValue("livingWith"))
		rec.Set("sleptWhere", r.FormValue("sleptWhere"))
		rec.Set("race", boolMapFromForm(r, "race"))
		rec.Set("raceOther", r.FormValue("raceOther"))
		rec.Set("sexAtBirth", r.FormValue("sexAtBirth"))
		rec.Set("servedMilitary", r.FormValue("servedMilitary"))
		rec.Set("militaryDetail", r.FormValue("militaryDetail"))
		rec.Set("hasPets", r.FormValue("hasPets"))
		rec.Set("petSupport", r.FormValue("petSupport"))
		rec.Set("petPrevented", r.FormValue("petPrevented"))
		rec.Set("employment", r.FormValue("employment"))
		rec.Set("unemployedDuration", r.FormValue("unemployedDuration"))
		rec.Set("interestedEmployed", r.FormValue("interestedEmployed"))
		rec.Set("jobTypes", r.FormValue("jobTypes"))
		rec.Set("household", householdFromForm(r))
	case "02":
		rec.Set("mentalHealth", r.FormValue("mentalHealth"))
		rec.Set("substanceUse", r.FormValue("substanceUse"))
		rec.Set("fleeingViolence", r.FormValue("fleeingViolence"))
		rec.Set("homelessFactors", r.FormValue("homelessFactors"))
	case "03":
		rec.Set("hmis", r.FormValue("hmis") == "on")
		rec.Set("hmisProvider", r.FormValue("hmisProvider"))
		rec.Set("documents", boolMapFromForm(r, "documents"))
		rec.Set("healthInsuranceDetail", r.FormValue("healthInsuranceDetail"))
		rec.Set("housing", boolMapFromForm(r, "housing"))
		rec.Set("income", boolMapFromForm(r, "income"))
		rec.Set("casemanagerName", r.FormValue("casemanagerName"))
		if cmName := strings.TrimSpace(r.FormValue("casemanagerName")); cmName != "" {
			s.ensureCaseManager(cmName)
		}
	case "04":
		rec.Set("personal", indexedFormValues(r, "personal_", 8))
	case "05":
		rec.Set("servicePlan", indexedFormValues(r, "servicePlan_", 8))
	default:
		return fmt.Errorf("unknown section %q", section)
	}
	return nil
}

// ensureCaseManager creates a case_manager user with no usable password if
// no case_manager with the given name (case-insensitive) exists. The user
// can't log in until an admin assigns a password.
func (s *Server) ensureCaseManager(name string) {
	col, err := s.pb.FindCollectionByNameOrId("users")
	if err != nil {
		return
	}
	recs, err := s.pb.FindRecordsByFilter(col.Id, "role='case_manager' && deleted=false", "name", 1000, 0)
	if err != nil {
		return
	}
	for _, r := range recs {
		if strings.EqualFold(r.GetString("name"), name) {
			return
		}
	}
	rec := core.NewRecord(col)
	rec.Set("name", name)
	rec.Set("role", "case_manager")
	rec.SetEmail(strings.ToLower(strings.ReplaceAll(name, " ", ".")) + "@pending.local")
	rec.SetPassword(cryptoRandHex(32))
	_ = s.pb.Save(rec)
}

// cryptoRandHex returns n random hex bytes from crypto/rand.
func cryptoRandHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// applyToState copies submitted fields onto a blank FormState (used when finish
// is posted before any section has been saved).
func (s *Server) applyToState(st *FormState, r *http.Request) {
	_ = r.ParseMultipartForm(10 << 20)
	st.EventSel = r.FormValue("event")
	st.Name = r.FormValue("name")
	st.DOB = normalizeDob(r.FormValue("dob"))
	st.SSN = ssnLast4(r.FormValue("ssn"))
	st.Contact = fmtPhone(r.FormValue("contact"))
	st.Email = r.FormValue("email")
	st.LivingWith = r.FormValue("livingWith")
	st.SleptWhere = r.FormValue("sleptWhere")
	st.Race = boolMapFromForm(r, "race")
	st.RaceOther = r.FormValue("raceOther")
	st.SexAtBirth = r.FormValue("sexAtBirth")
	st.ServedMilitary = r.FormValue("servedMilitary")
	st.MilitaryDetail = r.FormValue("militaryDetail")
	st.HasPets = r.FormValue("hasPets")
	st.PetSupport = r.FormValue("petSupport")
	st.PetPrevented = r.FormValue("petPrevented")
	st.Employment = r.FormValue("employment")
	st.UnemployedDuration = r.FormValue("unemployedDuration")
	st.InterestedEmployed = r.FormValue("interestedEmployed")
	st.JobTypes = r.FormValue("jobTypes")
	st.MentalHealth = r.FormValue("mentalHealth")
	st.SubstanceUse = r.FormValue("substanceUse")
	st.FleeingViolence = r.FormValue("fleeingViolence")
	st.HomelessFactors = r.FormValue("homelessFactors")
	st.HMIS = r.FormValue("hmis") == "on"
	st.HMISProvider = r.FormValue("hmisProvider")
	st.Documents = boolMapFromForm(r, "documents")
	st.HealthInsuranceDetail = r.FormValue("healthInsuranceDetail")
	st.Housing = boolMapFromForm(r, "housing")
	st.Income = boolMapFromForm(r, "income")
	st.Personal = indexedFormValues(r, "personal_", 8)
	st.ServicePlan = indexedFormValues(r, "servicePlan_", 8)
	st.CasemanagerName = r.FormValue("casemanagerName")
}

// validateRecord runs the 12 required-field checks against a saved record.
func (s *Server) validateRecord(rec *core.Record) map[string]bool {
	errs := map[string]bool{}
	for _, k := range REQUIRED_FIELDS {
		if k == "race" {
			if !anyBool(asBoolMap(rec, "race")) {
				errs["race"] = true
			}
			continue
		}
		if strings.TrimSpace(rec.GetString(k)) == "" {
			errs[k] = true
		}
	}
	if len(digitsOnly(rec.GetString("contact"))) < 10 {
		errs["contact"] = true
	}
	return errs
}

// validateState runs the 12 required-field checks against a FormState.
func (s *Server) validateState(st *FormState) map[string]bool {
	errs := map[string]bool{}
	check := func(k, v string) {
		if strings.TrimSpace(v) == "" {
			errs[k] = true
		}
	}
	check("event", st.EventSel)
	check("name", st.Name)
	check("dob", st.DOB)
	check("contact", st.Contact)
	if !anyBool(st.Race) {
		errs["race"] = true
	}
	check("sexAtBirth", st.SexAtBirth)
	check("servedMilitary", st.ServedMilitary)
	check("hasPets", st.HasPets)
	check("employment", st.Employment)
	check("mentalHealth", st.MentalHealth)
	check("substanceUse", st.SubstanceUse)
	check("fleeingViolence", st.FleeingViolence)
	if len(digitsOnly(st.Contact)) < 10 {
		errs["contact"] = true
	}
	return errs
}

// digitsOnly returns only the 0-9 characters of s.
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// fmtPhone formats a 10-digit number as (808) 555-0100; non-10-digit values are
// returned trimmed (left as-is) so partial/international entries are not mangled.
func fmtPhone(s string) string {
	d := digitsOnly(s)
	if len(d) != 10 {
		return strings.TrimSpace(s)
	}
	return "(" + d[:3] + ") " + d[3:6] + "-" + d[6:10]
}

// fmtDob converts ISO YYYY-MM-DD to display MM/DD/YYYY; anything else is returned as-is.
func fmtDob(s string) string {
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return s[5:7] + "/" + s[8:10] + "/" + s[0:4]
	}
	return s
}

// normalizeDob stores DOB as ISO YYYY-MM-DD. Accepts MM/DD/YYYY input (the
// mask output) and passes ISO through; anything else or an invalid calendar
// date is returned as an empty string.
func normalizeDob(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var iso string
	if len(s) == 10 && s[4] == '-' && s[7] == '-' { // already ISO
		iso = s
	} else if len(s) == 10 && s[2] == '/' && s[5] == '/' { // MM/DD/YYYY
		iso = s[6:10] + "-" + s[0:2] + "-" + s[3:5]
	} else {
		return ""
	}
	if _, err := time.Parse("2006-01-02", iso); err != nil {
		return ""
	}
	return iso
}

// ssnLast4 returns the last 4 digits of s (empty if none). Used to store and
// display only the last 4 of the SSN.
func ssnLast4(s string) string {
	d := digitsOnly(s)
	if len(d) <= 4 {
		return d
	}
	return d[len(d)-4:]
}

// validateNameFields checks the first_name/last_name form fields. If both keys
// are absent it falls back to the legacy single `name` field. Returns nil when
// valid, otherwise a map of field name -> message.
func validateNameFields(r *http.Request) map[string]string {
	first := strings.TrimSpace(r.FormValue("first_name"))
	last := strings.TrimSpace(r.FormValue("last_name"))
	hasFirst := r.Form.Has("first_name")
	hasLast := r.Form.Has("last_name")

	// Legacy single-name path (no first/last inputs submitted).
	if !hasFirst && !hasLast {
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

// requiredFieldMessages translates validateRecord's missing-field keys into
// the map[string]string the frontend expects. The form edits name as
// first_name + last_name, so the "name" requirement is surfaced as both
// keys. Messages mirror the per-field error text in index.html.
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

// requiredFieldMessage returns the human-readable message for a required
// field, matching the error text already rendered by index.html.
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

// writeValidationErrors writes a 400 with the JSON body the frontend expects:
// {"errors":{"field":"message"}}.
func writeValidationErrors(w http.ResponseWriter, errs map[string]string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{"errors": errs})
}

// --- form helpers ----------------------------------------------------------

func boolMapFromForm(r *http.Request, key string) map[string]bool {
	out := map[string]bool{}
	for _, v := range r.Form[key] {
		out[v] = true
	}
	return out
}

func indexedFormValues(r *http.Request, prefix string, n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = r.FormValue(fmt.Sprintf("%s%d", prefix, i))
	}
	return out
}

func anyBool(m map[string]bool) bool {
	for _, v := range m {
		if v {
			return true
		}
	}
	return false
}

// HouseholdRow is one household member row for the template.
type HouseholdRow struct {
	Name         string
	Relationship string
}

// householdFromForm reads the household array. With JS on, the hidden
// "household" input carries JSON.stringify(household). Without JS, the
// server-rendered fallback rows submit as household_name_N / household_rel_N
// indexed inputs — read those so no household data is lost when JS is off.
func householdFromForm(r *http.Request) []map[string]string {
	if hh := r.FormValue("household"); hh != "" {
		var rows []map[string]string
		if json.Unmarshal([]byte(hh), &rows) == nil && len(rows) > 0 {
			return rows
		}
	}
	rows := []map[string]string{}
	for i := 0; i < 50; i++ {
		n := r.FormValue(fmt.Sprintf("household_name_%d", i))
		rel := r.FormValue(fmt.Sprintf("household_rel_%d", i))
		if n == "" && rel == "" && i > 0 {
			break
		}
		rows = append(rows, map[string]string{"name": n, "relationship": rel})
	}
	if len(rows) == 0 {
		rows = []map[string]string{{"name": "", "relationship": ""}}
	}
	return rows
}

// handleDuplicateSearch returns an HTML fragment listing intake records
// whose name matches the ?name= query (auth-only, used by the intake form's
// live duplicate check). Min 2 chars, max 10 results, all records (no role
// scoping — duplicate detection is cross-caseload).
func (s *Server) handleDuplicateSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("name"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(q) < 2 {
		return // empty body — no results for short queries
	}
	col, err := s.intakeCollection()
	if err != nil {
		return
	}
	filter := fmt.Sprintf(`name ~ "%s"`, mcpmod.EscapeFilter(q))
	recs, err := s.pb.FindRecordsByFilter(col.Id, filter, "-created", 10, 0)
	if err != nil {
		return
	}
	type dupResult struct {
		ID       string
		Name     string
		SSNLast4 string
	}
	results := make([]dupResult, 0, len(recs))
	for _, rec := range recs {
		s.decryptSensitive(rec)
		name := rec.GetString("name")
		if name == "" {
			name = "(unnamed)"
		}
		ssn := rec.GetString("ssn")
		if len(ssn) > 4 {
			ssn = ssn[len(ssn)-4:]
		}
		results = append(results, dupResult{
			ID:       rec.Id,
			Name:     name,
			SSNLast4: ssn,
		})
	}
	_ = s.tpl.ExecuteTemplate(w, "dup-fragment", results)
}

// parseHouseholdRows reads the household JSON field into HouseholdRow values.
// PB hands back types.JSONRaw (named []byte) for JSON fields, so route through
// jsonBytes. Always returns at least one blank row (a "null"/empty field would
// otherwise yield a nil slice the template can't range safely).
func parseHouseholdRows(rec *core.Record) []HouseholdRow {
	out := []HouseholdRow{{Name: "", Relationship: ""}}
	b, ok := jsonBytes(rec.Get("household"))
	if !ok || len(b) == 0 {
		return out
	}
	var rows []HouseholdRow
	if json.Unmarshal(b, &rows) == nil && len(rows) > 0 {
		return rows
	}
	return out
}
