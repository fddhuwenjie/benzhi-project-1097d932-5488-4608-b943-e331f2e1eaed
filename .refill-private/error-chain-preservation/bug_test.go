package errorchainpreservation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/web"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func TestGetMissingCasePreservesNotFoundCode(t *testing.T) {
	repo, err := store.Open(t.TempDir() + "/cases.db")
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	service := workflow.NewService(repo)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/cases/case_missing_for_error_chain", nil).WithContext(context.Background())
	recorder := httptest.NewRecorder()
	web.New(service, nil).Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Error == nil || envelope.Error.Code != "NOT_FOUND" {
		got := "<nil>"
		if envelope.Error != nil {
			got = envelope.Error.Code
		}
		t.Fatalf("error.code = %s, want NOT_FOUND", got)
	}
}
