package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
)

type SQLiteRepository struct {
	db *sql.DB

	manifestMu   sync.Mutex
	manifestRows map[string]*sql.Row
}

func Open(path string) (*SQLiteRepository, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("数据库路径不能为空")
	}
	dsn := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		dsn = "file:" + path
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	repo := &SQLiteRepository{db: db, manifestRows: make(map[string]*sql.Row)}
	if err := repo.initialize(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	if err := repo.VerifyAll(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *SQLiteRepository) initialize(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("初始化数据库模式: %w", err)
	}
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_meta`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO schema_meta(version) VALUES(?)`, schemaVersion); err != nil {
			return err
		}
	}
	var version int
	if err := r.db.QueryRowContext(ctx, `SELECT version FROM schema_meta LIMIT 1`).Scan(&version); err != nil {
		return err
	}
	if version == 1 {
		migration := []string{
			`ALTER TABLE remediation_evidence ADD COLUMN round INTEGER NOT NULL DEFAULT 1`,
			`ALTER TABLE remediation_evidence ADD COLUMN supersedes_evidence_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE remediation_evidence ADD COLUMN failure_reason TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE review_decisions ADD COLUMN previous_return_decision_id TEXT NOT NULL DEFAULT ''`,
		}
		for _, statement := range migration {
			if _, err := r.db.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("迁移数据库到版本 2: %w", err)
			}
		}
		if err := r.migrateV1Aggregates(ctx); err != nil {
			return err
		}
		if _, err := r.db.ExecContext(ctx, `UPDATE schema_meta SET version=2`); err != nil {
			return err
		}
		version = 2
	}
	if version != schemaVersion {
		return fmt.Errorf("不支持的数据库版本 %d，期望 %d", version, schemaVersion)
	}
	return nil
}

func (r *SQLiteRepository) migrateV1Aggregates(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `SELECT case_id,aggregate_json FROM publication_cases ORDER BY case_id`)
	if err != nil {
		return err
	}
	type record struct {
		id  string
		raw []byte
	}
	records := []record{}
	for rows.Next() {
		var item record
		if err := rows.Scan(&item.id, &item.raw); err != nil {
			rows.Close()
			return err
		}
		records = append(records, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range records {
		var a accessibility.CaseAggregate
		if err := json.Unmarshal(item.raw, &a); err != nil {
			return fmt.Errorf("迁移个案 %s: %w", item.id, err)
		}
		rounds := map[string]int{}
		latest := map[string]string{}
		for i := range a.Evidences {
			e := &a.Evidences[i]
			rounds[e.FindingID]++
			e.Round = rounds[e.FindingID]
			e.SupersedesEvidenceID = latest[e.FindingID]
			if e.VerificationResult == "FAIL" && strings.TrimSpace(e.FailureReason) == "" {
				e.FailureReason = "历史 FAIL 证据未记录失败原因"
			}
			latest[e.FindingID] = e.EvidenceID
			if _, err := tx.ExecContext(ctx, `UPDATE remediation_evidence SET round=?,supersedes_evidence_id=?,failure_reason=? WHERE evidence_id=?`, e.Round, e.SupersedesEvidenceID, e.FailureReason, e.EvidenceID); err != nil {
				return err
			}
		}
		raw, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE publication_cases SET aggregate_json=? WHERE case_id=?`, raw, item.id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) Close() error { return r.db.Close() }
