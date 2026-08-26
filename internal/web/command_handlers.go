package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func (s *Server) PreflightProfileHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.PreflightProfileCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, e := s.service.PreflightProfile(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.CreateCaseCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	result, replay, err := s.service.CreateCase(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	if replay {
		w.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(w, http.StatusCreated, result)
}
func (s *Server) FreezeProfileHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.FreezeProfileCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, p, e := s.service.FreezeProfile(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeMutation(w, v, p)
}
func (s *Server) AddFindingHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.AddFindingCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, p, e := s.service.AddFinding(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeMutation(w, v, p)
}
func (s *Server) AddFindingsHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.AddFindingsCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, p, e := s.service.AddFindings(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeMutation(w, v, p)
}
func (s *Server) CompleteAuditHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.CompleteAuditCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, p, e := s.service.CompleteAudit(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeMutation(w, v, p)
}
func (s *Server) SubmitEvidenceHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.SubmitEvidenceCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, p, e := s.service.SubmitEvidence(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeMutation(w, v, p)
}
func (s *Server) SubmitReviewHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.SubmitReviewCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, p, e := s.service.SubmitReview(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeMutation(w, v, p)
}
func (s *Server) DecideReviewHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.DecideReviewCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, p, e := s.service.DecideReview(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeMutation(w, v, p)
}
func (s *Server) IssueReleaseHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.IssueReleaseCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, p, e := s.service.IssueRelease(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeMutation(w, v, p)
}
func (s *Server) ArchiveHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.ArchiveCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, p, e := s.service.Archive(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeMutation(w, v, p)
}
func (s *Server) VerifyReleaseHandler(w http.ResponseWriter, r *http.Request, id string) {
	var c workflow.VerifyReleaseCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, e := s.service.VerifyRelease(r.Context(), id, c)
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) ExportArchiveHandler(w http.ResponseWriter, r *http.Request, id string) {
	v, e := s.service.ExportArchive(r.Context(), id)
	if e != nil {
		writeError(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if v.Verified {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", id+"_"+v.Evidence.ArchiveManifest.ManifestID+".json"))
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}
