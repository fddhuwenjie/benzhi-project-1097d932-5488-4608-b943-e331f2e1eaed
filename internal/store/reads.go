package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/audit"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadAggregate(ctx context.Context, q queryer, caseID string) (accessibility.CaseAggregate, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT aggregate_json FROM publication_cases WHERE case_id=?`, caseID).Scan(&raw)
	if err == sql.ErrNoRows {
		return accessibility.CaseAggregate{}, workflow.NewError("NOT_FOUND", "个案不存在")
	}
	if err != nil {
		return accessibility.CaseAggregate{}, err
	}
	var a accessibility.CaseAggregate
	if err := json.Unmarshal(raw, &a); err != nil {
		return a, err
	}
	return a, nil
}

func (r *SQLiteRepository) Get(ctx context.Context, caseID string) (accessibility.CaseAggregate, error) {
	return loadAggregate(ctx, r.db, caseID)
}

func (r *SQLiteRepository) List(ctx context.Context) ([]accessibility.PublicationCase, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT aggregate_json FROM publication_cases ORDER BY updated_at DESC, case_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []accessibility.PublicationCase{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var a accessibility.CaseAggregate
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		result = append(result, a.Case)
	}
	return result, rows.Err()
}

func readEvents(ctx context.Context, q queryer, caseID string) ([]audit.Event, error) {
	rows, err := q.QueryContext(ctx, `SELECT event_id,case_id,sequence,event_type,actor_id,revision,payload,payload_digest,previous_digest,event_digest,occurred_at FROM audit_events WHERE case_id=? ORDER BY sequence`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []audit.Event{}
	for rows.Next() {
		var e audit.Event
		var occurred string
		if err := rows.Scan(&e.EventID, &e.CaseID, &e.Sequence, &e.EventType, &e.ActorID, &e.Revision, &e.Payload, &e.PayloadDigest, &e.PreviousDigest, &e.EventDigest, &occurred); err != nil {
			return nil, err
		}
		e.OccurredAt, err = time.Parse(time.RFC3339Nano, occurred)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) Events(ctx context.Context, caseID string) ([]audit.Event, error) {
	return readEvents(ctx, r.db, caseID)
}

func (r *SQLiteRepository) StoredManifestDigest(ctx context.Context, caseID string) (string, error) {
	var digest string
	err := r.db.QueryRowContext(ctx, `SELECT manifest_digest FROM archive_manifests WHERE case_id=?`, caseID).Scan(&digest)
	if err == sql.ErrNoRows {
		return "", workflow.NewError("NOT_FOUND", "归档清单不存在")
	}
	return digest, err
}

func (r *SQLiteRepository) VerifyAll(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `SELECT case_id FROM publication_cases ORDER BY case_id`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		aggregate, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
		if err := aggregate.Validate(); err != nil {
			return workflow.NewError("AGGREGATE_INVALID", "个案 %s: %v", id, err)
		}
		events, err := r.Events(ctx, id)
		if err != nil {
			return err
		}
		if v := audit.Verify(events); !v.Valid {
			return workflow.NewError("AUDIT_CHAIN_INVALID", "个案 %s: %s", id, v.Diagnostic)
		}
		if len(events) == 0 || events[len(events)-1].Revision != aggregate.Case.Revision {
			return workflow.NewError("AUDIT_CHAIN_INVALID", "个案 %s 的链头修订号与聚合不一致", id)
		}
		if err := r.VerifyGraph(ctx, aggregate); err != nil {
			return err
		}
	}
	return nil
}
