package workflow_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func archiveDigest(ch string) string { return strings.Repeat(ch, 64) }

func buildArchivedCase(t *testing.T, s *workflow.Service, suffix string) string {
	t.Helper()
	ctx := context.Background()
	a, _, err := s.CreateCase(ctx, workflow.CreateCaseCommand{RequestID: "create-" + suffix, Title: "归档个案 " + suffix, Edition: "1.0", MediaFormat: "EPUB 3", OwnerID: "owner-" + suffix, ContentDigest: archiveDigest("a")})
	if err != nil {
		t.Fatal(err)
	}
	caseID := a.Case.CaseID
	a, _, err = s.FreezeProfile(ctx, caseID, workflow.FreezeProfileCommand{CommandMeta: workflow.CommandMeta{RequestID: "profile-" + suffix, ActorID: "owner-" + suffix, ExpectedRevision: a.Case.Revision}, RulesetVersion: "WCAG-2.2", RuleCodes: []string{"1.1.1"}, BlockingSeverities: []string{"CRITICAL"}})
	if err != nil {
		t.Fatal(err)
	}
	a, _, err = s.AddFinding(ctx, caseID, workflow.AddFindingCommand{CommandMeta: workflow.CommandMeta{RequestID: "finding-" + suffix, ActorID: "auditor-" + suffix, ExpectedRevision: a.Case.Revision}, RuleCode: "1.1.1", ContentLocator: "chapter.xhtml#image", Severity: "CRITICAL", Impact: "读屏无法感知图片"})
	if err != nil {
		t.Fatal(err)
	}
	findingID := a.Findings[0].FindingID
	a, _, err = s.CompleteAudit(ctx, caseID, workflow.CompleteAuditCommand{CommandMeta: workflow.CommandMeta{RequestID: "audit-" + suffix, ActorID: "auditor-" + suffix, ExpectedRevision: a.Case.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	a, _, err = s.SubmitEvidence(ctx, caseID, workflow.SubmitEvidenceCommand{CommandMeta: workflow.CommandMeta{RequestID: "evidence-" + suffix, ActorID: "editor-" + suffix, ExpectedRevision: a.Case.Revision}, FindingID: findingID, ChangeSummary: "补充替代文本", BeforeDigest: archiveDigest("b"), AfterDigest: archiveDigest("c"), VerificationResult: "PASS"})
	if err != nil {
		t.Fatal(err)
	}
	a, _, err = s.SubmitReview(ctx, caseID, workflow.SubmitReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "submit-review-" + suffix, ActorID: "owner-" + suffix, ExpectedRevision: a.Case.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	a, _, err = s.DecideReview(ctx, caseID, workflow.DecideReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "decide-review-" + suffix, ActorID: "reviewer-" + suffix, ExpectedRevision: a.Case.Revision}, Outcome: "APPROVE", Reason: "证据满足要求"})
	if err != nil {
		t.Fatal(err)
	}
	a, _, err = s.IssueRelease(ctx, caseID, workflow.IssueReleaseCommand{CommandMeta: workflow.CommandMeta{RequestID: "release-" + suffix, ActorID: "publisher-" + suffix, ExpectedRevision: a.Case.Revision}, ValidHours: 24})
	if err != nil {
		t.Fatal(err)
	}
	a, _, err = s.Archive(ctx, caseID, workflow.ArchiveCommand{CommandMeta: workflow.CommandMeta{RequestID: "archive-" + suffix, ActorID: "archivist-" + suffix, ExpectedRevision: a.Case.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	return a.Case.CaseID
}

func TestArchiveExportIsolatedPerCase(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "archive-cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := workflow.NewService(repo)
	firstID := buildArchivedCase(t, service, "one")
	secondID := buildArchivedCase(t, service, "two")
	first, err := service.ExportArchive(context.Background(), firstID)
	if err != nil || !first.Verified {
		t.Fatalf("首个归档导出失败: verified=%v err=%v", first.Verified, err)
	}
	second, err := service.ExportArchive(context.Background(), secondID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Evidence.Case.CaseID != secondID {
		t.Fatalf("跨个案复用了归档导出结果: got=%s want=%s", second.Evidence.Case.CaseID, secondID)
	}
}
