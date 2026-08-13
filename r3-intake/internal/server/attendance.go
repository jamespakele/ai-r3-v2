package server

import "net/http"

// AttendanceView is the view model for the attendance matrix placeholder.
// Later cards extend this with site/event filters, date range, matrix rows,
// and summary stats.
type AttendanceView struct {
	UserName string
	Role     string
	IsAdmin  bool
}

// handleMatrix renders the attendance matrix screen. It is wrapped with
// requireAuth in the mux, so an authenticated session is guaranteed here.
func (s *Server) handleMatrix(w http.ResponseWriter, r *http.Request) {
	u := s.currentSession(r)
	view := AttendanceView{
		UserName: u.Name,
		Role:     u.Role,
		IsAdmin:  u.Role == "admin",
	}
	_ = s.tpl.ExecuteTemplate(w, "matrix", view)
}
