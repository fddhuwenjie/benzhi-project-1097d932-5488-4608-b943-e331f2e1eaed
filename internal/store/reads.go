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
		if err := r.verifyCaseColumns(ctx, id, aggregate.Case); err != nil {
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

// verifyCaseColumns confirms that the normalized business columns of a
// publication_cases row agree with the case object embedded in its
// aggregate_json. A divergence means the persisted view (used by optimistic
// concurrency and list queries) no longer reflects the authoritative aggregate,
// which would let the startup integrity check pass while reads serve stale
// state and subsequent legitimate writes fail on a phantom revision conflict.
func (r *SQLiteRepository) verifyCaseColumns(ctx context.Context, caseID string, c accessibility.PublicationCase) error {
	var (
		title        string
		edition      string
		mediaFormat  string
		ownerID      string
		contentDigest string
		status       string
		revision     int64
		createdAt    string
		updatedAt    string
	)
	err := r.db.QueryRowContext(ctx, `SELECT title,edition,media_format,owner_id,content_digest,status,revision,created_at,updated_at FROM publication_cases WHERE case_id=?`, caseID).
		Scan(&title, &edition, &mediaFormat, &ownerID, &contentDigest, &status, &revision, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return workflow.NewError("PERSISTENCE_INTEGRITY_ERROR", "个案 %s 的规范化行不存在", caseID)
	}
	if err != nil {
		return err
	}
	discrepancies := []string{}
	if title != c.Title {
		discrepancies = append(discrepancies, fmt.Sprintf("标题：列为 %q 聚合为 %q", title, c.Title))
	}
	if edition != c.Edition {
		discrepancies = append(discrepancies, fmt.Sprintf("版本：列为 %q 聚合为 %q", edition, c.Edition))
	}
	if mediaFormat != c.MediaFormat {
		discrepancies = append(discrepancies, fmt.Sprintf("介质格式：列为 %q 聚合为 %q", mediaFormat, c.MediaFormat))
	}
	if ownerID != c.OwnerID {
		discrepancies = append(discrepancies, fmt.Sprintf("归属：列为 %q 聚合为 %q", ownerID, c.OwnerID))
	}
	if contentDigest != c.ContentDigest {
		discrepancies = append(discrepancies, fmt.Sprintf("摘要：列为 %q 聚合为 %q", contentDigest, c.ContentDigest))
	}
	if status != string(c.Status) {
		discrepancies = append(discrepancies, fmt.Sprintf("状态：列为 %q 聚合为 %q", status, c.Status))
	}
	if revision != c.Revision {
		discrepancies = append(discrepancies, fmt.Sprintf("修订号：列为 %d 聚合为 %d", revision, c.Revision))
	}
	if createdAt != c.CreatedAt.Format(time.RFC3339Nano) {
		discrepancies = append(discrepancies, fmt.Sprintf("创建时间：列为 %q 聚合为 %q", createdAt, c.CreatedAt.Format(time.RFC3339Nano)))
	}
	if updatedAt != c.UpdatedAt.Format(time.RFC3339Nano) {
		discrepancies = append(discrepancies, fmt.Sprintf("更新时间：列为 %q 聚合为 %q", updatedAt, c.UpdatedAt.Format(time.RFC3339Nano)))
	}
	if len(discrepancies) > 0 {
		return workflow.NewError("PERSISTENCE_INTEGRITY_ERROR", "个案 %s 的规范化列与聚合不一致：%v", caseID, discrepancies)
	}
	return nil
}
