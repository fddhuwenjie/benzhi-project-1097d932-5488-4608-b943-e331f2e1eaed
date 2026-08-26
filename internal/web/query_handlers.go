package web

import "net/http"

func (s *Server) ListCasesHandler(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.ListCases(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) GetCaseHandler(w http.ResponseWriter, r *http.Request, caseID string) {
	result, err := s.service.GetCase(r.Context(), caseID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) EventsHandler(w http.ResponseWriter, r *http.Request, caseID string) {
	result, err := s.service.Events(r.Context(), caseID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
