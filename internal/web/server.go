package web

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

type Server struct {
	service *workflow.Service
	logger  *slog.Logger
	handler http.Handler
}

func New(service *workflow.Service, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{service: service, logger: logger}
	s.handler = s.security(s.accessLog(http.HandlerFunc(s.route)))
	return s
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	if r.Method == http.MethodGet && path == "/" {
		s.IndexHandler(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(path, "/assets/") {
		s.AssetHandler(w, r)
		return
	}
	if path == "/api/v1/health" && r.Method == http.MethodGet {
		s.HealthHandler(w, r)
		return
	}
	if path == "/api/v1/cases" {
		if r.Method == http.MethodGet {
			s.ListCasesHandler(w, r)
			return
		}
		if r.Method == http.MethodPost {
			s.CreateCaseHandler(w, r)
			return
		}
	}
	prefix := "/api/v1/cases/"
	if strings.HasPrefix(path, prefix) {
		remainder := strings.TrimPrefix(path, prefix)
		parts := strings.Split(remainder, "/")
		if len(parts) >= 1 && parts[0] != "" {
			caseID := parts[0]
			if len(parts) == 1 && r.Method == http.MethodGet {
				s.GetCaseHandler(w, r, caseID)
				return
			}
			if len(parts) == 2 && parts[1] == "events" && r.Method == http.MethodGet {
				s.EventsHandler(w, r, caseID)
				return
			}
			if len(parts) == 3 && parts[1] == "archive" && parts[2] == "export" && r.Method == http.MethodGet {
				s.ExportArchiveHandler(w, r, caseID)
				return
			}
			if r.Method == http.MethodPost {
				s.dispatchCommand(w, r, caseID, parts[1:])
				return
			}
		}
	}
	writeError(w, workflow.NewError("NOT_FOUND", "路由不存在"))
}

func (s *Server) dispatchCommand(w http.ResponseWriter, r *http.Request, caseID string, suffix []string) {
	key := strings.Join(suffix, "/")
	switch key {
	case "profile/preflight":
		s.PreflightProfileHandler(w, r, caseID)
	case "profile":
		s.FreezeProfileHandler(w, r, caseID)
	case "findings":
		s.AddFindingHandler(w, r, caseID)
	case "findings/batch":
		s.AddFindingsHandler(w, r, caseID)
	case "audit/complete":
		s.CompleteAuditHandler(w, r, caseID)
	case "remediations":
		s.SubmitEvidenceHandler(w, r, caseID)
	case "review/submit":
		s.SubmitReviewHandler(w, r, caseID)
	case "review/decision":
		s.DecideReviewHandler(w, r, caseID)
	case "release":
		s.IssueReleaseHandler(w, r, caseID)
	case "release/verify":
		s.VerifyReleaseHandler(w, r, caseID)
	case "archive":
		s.ArchiveHandler(w, r, caseID)
	default:
		writeError(w, workflow.NewError("NOT_FOUND", "命令路由不存在"))
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		s.logger.Info("http_request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "bytes", sw.bytes, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
