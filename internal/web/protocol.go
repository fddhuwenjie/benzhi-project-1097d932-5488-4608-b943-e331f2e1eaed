package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

const maxRequestBody = 1 << 20

type envelope struct {
	Data  any        `json:"data,omitempty"`
	Error *errorBody `json:"error,omitempty"`
}
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return workflow.NewError("VALIDATION_ERROR", "Content-Type 必须为 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return workflow.NewError("VALIDATION_ERROR", "JSON 请求无效: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return workflow.NewError("VALIDATION_ERROR", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Data: data})
}

func writeMutation(w http.ResponseWriter, data any, replay bool) {
	if replay {
		w.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(w, http.StatusOK, data)
}

func writeError(w http.ResponseWriter, err error) {
	code := workflow.Code(err)
	status := http.StatusInternalServerError
	switch code {
	case "VALIDATION_ERROR":
		status = http.StatusBadRequest
	case "NOT_FOUND":
		status = http.StatusNotFound
	case "REVISION_CONFLICT", "IDEMPOTENCY_CONFLICT", "CONFLICT", "INVALID_STATE", "BLOCKERS_REMAIN", "INVALID_EVIDENCE", "DIGEST_MISMATCH", "AUDIT_CHAIN_INVALID", "AGGREGATE_INVALID", "PERSISTENCE_INTEGRITY_ERROR", "RELEASE_NOT_READY", "RELEASE_EXPIRED", "DUPLICATE_FINDING", "STALE_EVIDENCE", "EVIDENCE_CHAIN_BROKEN", "REMEDIATION_ITEMS_PENDING":
		status = http.StatusConflict
	case "ROLE_CONFLICT", "SEPARATION_CONFLICT":
		status = http.StatusForbidden
	case "RULE_NOT_IN_PROFILE", "FINDING_BATCH_INVALID", "REMEDIATION_ITEMS_INVALID":
		status = http.StatusUnprocessableEntity
	}
	message := "内部服务错误"
	var details any
	var app *workflow.Error
	if errors.As(err, &app) {
		message = app.Message
		details = app.Details
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Error: &errorBody{Code: code, Message: message, Details: details}})
}
