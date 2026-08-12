package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"

	mcpmod "r3-intake/internal/mcp"
)

// NoteUser is one option for the note-author dropdown: label is "Name - role".
type NoteUser struct {
	ID    string
	Label string
}

// NoteRow is one existing note in the list view.
type NoteRow struct {
	ID          string
	AuthorLabel string // "Name - role" or "Unknown"
	Date        string // ISO YYYY-MM-DD, display as-is
	Body        string
	Created     string // pretty-printed via formatTime
}

// NotesView is the view model for the "notes" template.
type NotesView struct {
	UserName   string
	Role       string
	IsAdmin    bool
	IntakeID   string
	IntakeName string
	Notes      []NoteRow
	Authors    []NoteUser
	AuthorSel  string // default-selected author = logged-in user ID, or note author in edit mode
	Today      string // YYYY-MM-DD
	Error      string // non-empty when the add/edit form was submitted blank
	EditNoteID string // non-empty when editing an existing note
	EditAuthor string // note's current author ID (for <select> pre-select)
	EditDate   string // note's current date (for date input value)
	EditBody   string // note's current body (for <textarea> content)
}

// NoteChangeRow is one entry in a note's audit history.
type NoteChangeRow struct {
	Action     string // "create", "update", or "delete"
	UserLabel  string // "Name - role" or "Unknown"
	ChangeFrom string
	ChangeTo   string
	Created    string // pretty-printed via formatTime
}

// NoteHistoryView is the view model for the "note-history" template.
type NoteHistoryView struct {
	UserName   string
	Role       string
	IsAdmin    bool
	IntakeID   string
	IntakeName string
	NoteBody   string
	Entries    []NoteChangeRow
}

// noteChangesCollection returns the note_changes collection.
func (s *Server) noteChangesCollection() (*core.Collection, error) {
	return s.pb.FindCollectionByNameOrId("note_changes")
}

// logNoteChange writes an audit record to note_changes.
// action is "create", "update", or "delete". fromBody/toBody are the old/new note body.
func (s *Server) logNoteChange(noteRec *core.Record, userID, action, fromBody, toBody string) {
	col, err := s.noteChangesCollection()
	if err != nil {
		return
	}
	entry := core.NewRecord(col)
	entry.Set("note_id", noteRec.Id)
	entry.Set("user_id", userID)
	entry.Set("action", action)
	entry.Set("change_from", fromBody)
	entry.Set("change_to", toBody)
	_ = s.pb.Save(entry)
}

// handleNotes renders the notes screen (GET) or adds/edits/deletes a note (POST) for one intake.
// Wrapped in requireAuth in the mux, so s.currentSession(r) is non-nil.
func (s *Server) handleNotes(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r) // non-nil (requireAuth)
	path := strings.TrimPrefix(r.URL.Path, "/notes/")
	parts := strings.SplitN(path, "/", 3)
	intakeID := strings.TrimSpace(parts[0])
	if intakeID == "" {
		http.NotFound(w, r)
		return
	}
	intake, err := s.findIntake(intakeID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// No noteID → list + add form.
	if len(parts) == 1 || parts[1] == "" {
		if r.Method == http.MethodGet {
			s.renderNotes(w, r, u, intake, "")
			return
		}
		if r.Method == http.MethodPost {
			s.handleNotesAdd(w, r, u, intake)
			return
		}
		http.NotFound(w, r)
		return
	}

	// Sub-action: {noteID}/edit or {noteID}/delete
	noteID := strings.TrimSpace(parts[1])
	action := ""
	if len(parts) > 2 {
		action = strings.TrimSpace(parts[2])
	}

	switch {
	case action == "edit" && r.Method == http.MethodGet:
		s.handleNotesEdit(w, r, u, intake, noteID)
	case action == "edit" && r.Method == http.MethodPost:
		s.handleNotesUpdate(w, r, u, intake, noteID)
	case action == "history" && r.Method == http.MethodGet:
		s.handleNotesHistory(w, r, u, intake, noteID)
	case action == "delete" && r.Method == http.MethodPost:
		s.handleNotesDelete(w, r, u, intake, noteID)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleNotesAdd(w http.ResponseWriter, r *http.Request, u *sessionUser, intake *core.Record) {
	_ = r.ParseForm()
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		s.renderNotes(w, r, u, intake, "Note cannot be empty.")
		return
	}

	author := strings.TrimSpace(r.FormValue("author"))
	if !s.validNoteAuthor(author) { // not a real system user → fall back to logged-in
		author = u.ID
	}
	date := strings.TrimSpace(r.FormValue("date"))
	if !isValidISODate(date) { // empty or malformed → today
		date = time.Now().In(hst).Format("2006-01-02")
	}

	col, err := s.notesCollection()
	if err != nil {
		http.Error(w, "notes collection missing", http.StatusInternalServerError)
		return
	}
	rec := core.NewRecord(col)
	rec.Set("intake", intake.Id)
	rec.Set("author", author)
	rec.Set("date", date)
	rec.Set("body", body)
	rec.Set("deleted", false)
	if err := s.pb.Save(rec); err != nil {
		http.Error(w, "save note failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.logNoteChange(rec, u.ID, "create", "", body)
	http.Redirect(w, r, "/notes/"+intake.Id, http.StatusSeeOther) // PRG
}

func (s *Server) handleNotesEdit(w http.ResponseWriter, r *http.Request, u *sessionUser, intake *core.Record, noteID string) {
	rec, err := s.findNote(intake, noteID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.renderNotesEdit(w, r, u, intake, rec, "")
}

func (s *Server) handleNotesUpdate(w http.ResponseWriter, r *http.Request, u *sessionUser, intake *core.Record, noteID string) {
	rec, err := s.findNote(intake, noteID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_ = r.ParseForm()
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		s.renderNotesEdit(w, r, u, intake, rec, "Note cannot be empty.")
		return
	}

	author := strings.TrimSpace(r.FormValue("author"))
	if !s.validNoteAuthor(author) { // not a real system user → fall back to logged-in
		author = u.ID
	}
	date := strings.TrimSpace(r.FormValue("date"))
	if !isValidISODate(date) { // empty or malformed → today
		date = time.Now().In(hst).Format("2006-01-02")
	}

	oldBody := rec.GetString("body")
	rec.Set("author", author)
	rec.Set("date", date)
	rec.Set("body", body)
	if err := s.pb.Save(rec); err != nil {
		http.Error(w, "save note failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.logNoteChange(rec, u.ID, "update", oldBody, body)
	http.Redirect(w, r, "/notes/"+intake.Id, http.StatusSeeOther)
}

func (s *Server) handleNotesDelete(w http.ResponseWriter, r *http.Request, u *sessionUser, intake *core.Record, noteID string) {
	rec, err := s.findNote(intake, noteID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	oldBody := rec.GetString("body")
	rec.Set("deleted", true)
	if err := s.pb.Save(rec); err != nil {
		http.Error(w, "delete note failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.logNoteChange(rec, u.ID, "delete", oldBody, "")
	http.Redirect(w, r, "/notes/"+intake.Id, http.StatusSeeOther)
}

// findNote returns a note record belonging to the given intake, or an error if not found/mismatched.
func (s *Server) findNote(intake *core.Record, noteID string) (*core.Record, error) {
	col, err := s.notesCollection()
	if err != nil {
		return nil, err
	}
	rec, err := s.pb.FindRecordById(col.Id, noteID)
	if err != nil || rec.GetString("intake") != intake.Id || rec.GetBool("deleted") {
		return nil, fmt.Errorf("note not found")
	}
	return rec, nil
}

func (s *Server) handleNotesHistory(w http.ResponseWriter, r *http.Request, u *sessionUser, intake *core.Record, noteID string) {
	noteRec, err := s.findNote(intake, noteID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	col, err := s.noteChangesCollection()
	if err != nil {
		http.Error(w, "audit collection missing", http.StatusInternalServerError)
		return
	}
	filter := fmt.Sprintf("note_id='%s'", mcpmod.EscapeFilter(noteRec.Id))
	recs, err := s.pb.FindRecordsByFilter(col.Id, filter, "-created", 500, 0)
	labelMap := s.noteUserLabelMap()
	entries := []NoteChangeRow{}
	if err == nil {
		for _, rec := range recs {
			ul := labelMap[rec.GetString("user_id")]
			if ul == "" {
				ul = "Unknown"
			}
			entries = append(entries, NoteChangeRow{
				Action:     rec.GetString("action"),
				UserLabel:  ul,
				ChangeFrom: rec.GetString("change_from"),
				ChangeTo:   rec.GetString("change_to"),
				Created:    formatTime(rec.GetString("created")),
			})
		}
	}
	view := &NoteHistoryView{
		UserName: u.Name, Role: u.Role, IsAdmin: u.Role == "admin",
		IntakeID: intake.Id, IntakeName: intake.GetString("name"),
		NoteBody: noteRec.GetString("body"),
		Entries:  entries,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tpl.ExecuteTemplate(w, "note-history", view)
}

func (s *Server) renderNotes(w http.ResponseWriter, r *http.Request, u *sessionUser, intake *core.Record, errMsg string) {
	view := &NotesView{
		UserName: u.Name, Role: u.Role, IsAdmin: u.Role == "admin",
		IntakeID: intake.Id, IntakeName: intake.GetString("name"),
		Notes: s.loadNoteRows(intake.Id), Authors: s.loadNoteUsers(), AuthorSel: u.ID,
		Today: time.Now().In(hst).Format("2006-01-02"), Error: errMsg,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tpl.ExecuteTemplate(w, "notes", view)
}

func (s *Server) renderNotesEdit(w http.ResponseWriter, r *http.Request, u *sessionUser, intake *core.Record, noteRec *core.Record, errMsg string) {
	view := &NotesView{
		UserName: u.Name, Role: u.Role, IsAdmin: u.Role == "admin",
		IntakeID: intake.Id, IntakeName: intake.GetString("name"),
		Notes: s.loadNoteRows(intake.Id), Authors: s.loadNoteUsers(), AuthorSel: noteRec.GetString("author"),
		Today: time.Now().In(hst).Format("2006-01-02"), Error: errMsg,
		EditNoteID: noteRec.Id,
		EditAuthor: noteRec.GetString("author"),
		EditDate:   noteRec.GetString("date"),
		EditBody:   noteRec.GetString("body"),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tpl.ExecuteTemplate(w, "notes", view)
}

// loadNoteRows returns all notes for an intake, newest first.
func (s *Server) loadNoteRows(intakeID string) []NoteRow {
	col, err := s.notesCollection()
	if err != nil {
		return []NoteRow{}
	}
	filter := fmt.Sprintf("intake='%s' && (deleted = false || deleted = null)", mcpmod.EscapeFilter(intakeID))
	recs, err := s.pb.FindRecordsByFilter(col.Id, filter, "-created", 500, 0)
	labelMap := s.noteUserLabelMap()
	rows := []NoteRow{}
	if err != nil {
		return rows
	}
	for _, rec := range recs {
		al := labelMap[rec.GetString("author")]
		if al == "" {
			al = "Unknown"
		}
		rows = append(rows, NoteRow{
			ID: rec.Id, AuthorLabel: al,
			Date: rec.GetString("date"), Body: rec.GetString("body"),
			Created: formatTime(rec.GetString("created")),
		})
	}
	return rows
}

// loadNoteUsers returns every system user as "Name - role", sorted by name.
func (s *Server) loadNoteUsers() []NoteUser {
	col, err := s.pb.FindCollectionByNameOrId("users")
	if err != nil {
		return nil
	}
	recs, err := s.pb.FindRecordsByFilter(col.Id, "1=1", "name", 1000, 0)
	if err != nil {
		return nil
	}
	out := make([]NoteUser, 0, len(recs))
	for _, r := range recs {
		name := r.GetString("name")
		if name == "" {
			name = r.GetString("email")
		}
		out = append(out, NoteUser{ID: r.Id, Label: name + " - " + r.GetString("role")})
	}
	return out
}

// noteUserLabelMap returns {userID: "Name - role"} for resolving note authors.
func (s *Server) noteUserLabelMap() map[string]string {
	m := map[string]string{}
	for _, u := range s.loadNoteUsers() {
		m[u.ID] = u.Label
	}
	return m
}

// validNoteAuthor reports whether id is an existing system user.
func (s *Server) validNoteAuthor(id string) bool {
	if id == "" {
		return false
	}
	_, err := s.pb.FindRecordById("users", id)
	return err == nil
}

// isValidISODate reports whether s parses as YYYY-MM-DD.
func isValidISODate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
