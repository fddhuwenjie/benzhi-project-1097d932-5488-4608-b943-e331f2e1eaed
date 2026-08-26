package store

import (
	"context"
	"database/sql"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// replay returns a previously persisted response when request_id and payload
// match. For update operations an optional caseID can be supplied; reusing the
// same request_id against a different case must not replay the first case's
// response.
func replay(q rowQuerier, ctx context.Context, requestID, payloadHash string, caseIDs ...string) ([]byte, bool, error) {
	var storedHash string
	var storedCaseID string
	var response []byte
	err := q.QueryRowContext(ctx, `SELECT case_id, payload_hash, response_json FROM idempotency_results WHERE request_id=?`, requestID).Scan(&storedCaseID, &storedHash, &response)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if storedHash != payloadHash {
		return nil, false, workflow.NewError("IDEMPOTENCY_CONFLICT", "request_id 已用于不同载荷")
	}
	if len(caseIDs) > 0 && caseIDs[0] != "" && storedCaseID != caseIDs[0] {
		return nil, false, workflow.NewError("IDEMPOTENCY_CONFLICT", "request_id 已用于其他个案")
	}
	return append([]byte(nil), response...), true, nil
}
