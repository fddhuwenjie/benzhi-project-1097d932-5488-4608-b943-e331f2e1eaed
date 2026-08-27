package archive_row_reuse_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func digest(ch string) string { return strings.Repeat(ch, 64) }

func TestArchiveExportDoesNotReuseConsumedRow(t *testing.T) {
	ctx := context.Background()
	repo, err := store.Open(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	service := workflow.NewService(repo)

	aggregate, _, err := service.CreateCase(ctx, workflow.CreateCaseCommand{
		RequestID: "archive-row-create", Title: "可重复导出的出版物", Edition: "1.0",
		MediaFormat: "EPUB 3", OwnerID: "owner", ContentDigest: digest("a"),
	})
	if err != nil {
		t.Fatal(err)
	}
	caseID := aggregate.Case.CaseID
	aggregate, _, err = service.FreezeProfile(ctx, caseID, workflow.FreezeProfileCommand{
		CommandMeta:    workflow.CommandMeta{RequestID: "archive-row-profile", ActorID: "owner", ExpectedRevision: aggregate.Case.Revision},
		RulesetVersion: "WCAG-2.2", RuleCodes: []string{"1.1.1"}, BlockingSeverities: []string{"CRITICAL"},
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregate, _, err = service.CompleteAudit(ctx, caseID, workflow.CompleteAuditCommand{CommandMeta: workflow.CommandMeta{
		RequestID: "archive-row-audit", ActorID: "auditor", ExpectedRevision: aggregate.Case.Revision,
	}})
	if err != nil {
		t.Fatal(err)
	}
	aggregate, _, err = service.SubmitReview(ctx, caseID, workflow.SubmitReviewCommand{CommandMeta: workflow.CommandMeta{
		RequestID: "archive-row-submit", ActorID: "owner", ExpectedRevision: aggregate.Case.Revision,
	}})
	if err != nil {
		t.Fatal(err)
	}
	aggregate, _, err = service.DecideReview(ctx, caseID, workflow.DecideReviewCommand{
		CommandMeta: workflow.CommandMeta{RequestID: "archive-row-decision", ActorID: "reviewer", ExpectedRevision: aggregate.Case.Revision},
		Outcome:     "APPROVE", Reason: "无阻断发现项，批准发布",
	})
	if err != nil {
		t.Fatal(err)
	}
	aggregate, _, err = service.IssueRelease(ctx, caseID, workflow.IssueReleaseCommand{
		CommandMeta: workflow.CommandMeta{RequestID: "archive-row-release", ActorID: "publisher", ExpectedRevision: aggregate.Case.Revision},
		ValidHours:  24,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = service.Archive(ctx, caseID, workflow.ArchiveCommand{CommandMeta: workflow.CommandMeta{
		RequestID: "archive-row-finalize", ActorID: "archivist", ExpectedRevision: aggregate.Case.Revision,
	}})
	if err != nil {
		t.Fatal(err)
	}

	firstCtx, cancelFirst := context.WithCancel(ctx)
	first, err := service.ExportArchive(firstCtx, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Verified || first.Components["manifest"].Status != "VERIFIED" {
		t.Fatalf("首次导出应通过清单校验: %+v", first.Components["manifest"])
	}
	cancelFirst()
	secondCtx, cancelSecond := context.WithCancel(ctx)
	defer cancelSecond()
	second, err := service.ExportArchive(secondCtx, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Verified || second.Components["manifest"].Status != "VERIFIED" {
		t.Fatalf("重复导出复用了已消费的查询行: verified=%v manifest=%+v", second.Verified, second.Components["manifest"])
	}
}
