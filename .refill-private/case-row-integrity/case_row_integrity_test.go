package caserowintegrity_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
	_ "modernc.org/sqlite"
)

func TestStartupRejectsDivergentCaseRowAndAggregate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case-row-divergence.db")
	repo, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.NewService(repo)
	aggregate, _, err := service.CreateCase(context.Background(), workflow.CreateCaseCommand{
		RequestID: "request-case-row", Title: "完整性测试", Edition: "第一版", MediaFormat: "EPUB",
		OwnerID: "owner", ContentDigest: accessibility.DigestBytes([]byte("content")),
	})
	if err != nil {
		repo.Close()
		t.Fatal(err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE publication_cases SET revision=revision+7, status='ARCHIVED' WHERE case_id=?`, aggregate.Case.CaseID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(path)
	if err == nil {
		reopened.Close()
		t.Fatalf("启动完整性检查接受了 publication_cases 列与 aggregate_json 不一致的数据")
	}
}
