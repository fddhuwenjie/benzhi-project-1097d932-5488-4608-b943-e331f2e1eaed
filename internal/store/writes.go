package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/audit"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func (r *SQLiteRepository) Create(ctx context.Context, requestID, payloadHash string, record workflow.CreateRecord) ([]byte, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	if raw, found, err := replay(tx, ctx, requestID, payloadHash); err != nil || found {
		return raw, found, err
	}
	if err := record.Aggregate.Validate(); err != nil {
		return nil, false, err
	}
	response, err := json.Marshal(record.Result)
	if err != nil {
		return nil, false, err
	}
	if err := insertCase(ctx, tx, &record.Aggregate); err != nil {
		return nil, false, translateConstraint(err)
	}
	if err := syncEvidenceGraph(ctx, tx, &record.Aggregate); err != nil {
		return nil, false, err
	}
	if err := insertEvent(ctx, tx, record.Event); err != nil {
		return nil, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO idempotency_results(request_id,case_id,payload_hash,response_json,created_at) VALUES(?,?,?,?,?)`, requestID, record.Aggregate.Case.CaseID, payloadHash, response, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return response, false, nil
}

func (r *SQLiteRepository) Update(ctx context.Context, caseID, requestID, payloadHash string, expectedRevision int64, mutate func(*accessibility.CaseAggregate, []audit.Event) (workflow.Mutation, error)) ([]byte, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	if raw, found, err := replay(tx, ctx, requestID, payloadHash, caseID); err != nil || found {
		return raw, found, err
	}
	a, err := loadAggregate(ctx, tx, caseID)
	if err != nil {
		return nil, false, err
	}
	if a.Case.Revision != expectedRevision {
		return nil, false, workflow.NewError("REVISION_CONFLICT", "修订号冲突：当前为 %d", a.Case.Revision)
	}
	events, err := readEvents(ctx, tx, caseID)
	if err != nil {
		return nil, false, err
	}
	mutation, err := mutate(&a, events)
	if err != nil {
		return nil, false, err
	}
	if a.Case.Revision != expectedRevision+1 {
		return nil, false, fmt.Errorf("工作流未按约定递增修订号")
	}
	if err := a.Validate(); err != nil {
		return nil, false, err
	}
	if mutation.Event.Sequence != int64(len(events)+1) || mutation.Event.Revision != a.Case.Revision {
		return nil, false, fmt.Errorf("审计事件序号或修订号无效")
	}
	response, err := json.Marshal(mutation.Result)
	if err != nil {
		return nil, false, err
	}
	aggregateJSON, err := json.Marshal(a)
	if err != nil {
		return nil, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE publication_cases SET title=?,edition=?,media_format=?,owner_id=?,content_digest=?,status=?,revision=?,updated_at=?,aggregate_json=? WHERE case_id=? AND revision=?`, a.Case.Title, a.Case.Edition, a.Case.MediaFormat, a.Case.OwnerID, a.Case.ContentDigest, a.Case.Status, a.Case.Revision, a.Case.UpdatedAt.Format(time.RFC3339Nano), aggregateJSON, caseID, expectedRevision)
	if err != nil {
		return nil, false, err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return nil, false, workflow.NewError("REVISION_CONFLICT", "个案被并发修改")
	}
	if err := syncEvidenceGraph(ctx, tx, &a); err != nil {
		return nil, false, err
	}
	if err := insertEvent(ctx, tx, mutation.Event); err != nil {
		return nil, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO idempotency_results(request_id,case_id,payload_hash,response_json,created_at) VALUES(?,?,?,?,?)`, requestID, caseID, payloadHash, response, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return response, false, nil
}

func insertCase(ctx context.Context, tx *sql.Tx, a *accessibility.CaseAggregate) error {
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO publication_cases(case_id,title,edition,media_format,owner_id,content_digest,status,revision,created_at,updated_at,aggregate_json) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.Case.CaseID, a.Case.Title, a.Case.Edition, a.Case.MediaFormat, a.Case.OwnerID, a.Case.ContentDigest, a.Case.Status, a.Case.Revision, a.Case.CreatedAt.Format(time.RFC3339Nano), a.Case.UpdatedAt.Format(time.RFC3339Nano), raw)
	return err
}

func insertEvent(ctx context.Context, tx *sql.Tx, e audit.Event) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events(event_id,case_id,sequence,event_type,actor_id,revision,payload,payload_digest,previous_digest,event_digest,occurred_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, e.EventID, e.CaseID, e.Sequence, e.EventType, e.ActorID, e.Revision, e.Payload, e.PayloadDigest, e.PreviousDigest, e.EventDigest, e.OccurredAt.Format(time.RFC3339Nano))
	return err
}

func translateConstraint(err error) error {
	return workflow.NewError("CONFLICT", "保存个案失败: %v", err)
}
