package workflow_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func digest(ch string) string { return strings.Repeat(ch, 64) }

type fixture struct {
	t *testing.T
	s *workflow.Service
	r *store.SQLiteRepository
	a accessibility.CaseAggregate
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	repo, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	return &fixture{t: t, s: workflow.NewService(repo), r: repo}
}

func (f *fixture) create() {
	f.t.Helper()
	a, replay, err := f.s.CreateCase(context.Background(), workflow.CreateCaseCommand{
		RequestID: "request-create-01", Title: "测试出版物", Edition: "1.0", MediaFormat: "EPUB 3", OwnerID: "owner", ContentDigest: digest("a"),
	})
	if err != nil || replay {
		f.t.Fatalf("建档失败 replay=%v err=%v", replay, err)
	}
	f.a = a
}

func (f *fixture) freeze() {
	f.t.Helper()
	a, _, err := f.s.FreezeProfile(context.Background(), f.a.Case.CaseID, workflow.FreezeProfileCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-profile-01", ActorID: "owner", ExpectedRevision: f.a.Case.Revision}, RulesetVersion: "WCAG-2.2", RuleCodes: []string{"1.1.1"}, BlockingSeverities: []string{"CRITICAL"}})
	if err != nil {
		f.t.Fatal(err)
	}
	f.a = a
}

func (f *fixture) finding() string {
	f.t.Helper()
	a, _, err := f.s.AddFinding(context.Background(), f.a.Case.CaseID, workflow.AddFindingCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-finding-01", ActorID: "auditor", ExpectedRevision: f.a.Case.Revision}, RuleCode: "1.1.1", ContentLocator: "chapter.xhtml#image", Severity: "CRITICAL", Impact: "读屏无法感知图片"})
	if err != nil {
		f.t.Fatal(err)
	}
	f.a = a
	return a.Findings[0].FindingID
}

func (f *fixture) completeAudit() {
	f.t.Helper()
	a, _, err := f.s.CompleteAudit(context.Background(), f.a.Case.CaseID, workflow.CompleteAuditCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-audit-0001", ActorID: "auditor", ExpectedRevision: f.a.Case.Revision}})
	if err != nil {
		f.t.Fatal(err)
	}
	f.a = a
}

func TestCompleteReleaseWorkflow(t *testing.T) {
	f := newFixture(t)
	f.create()
	f.freeze()
	findingID := f.finding()
	f.completeAudit()
	a, _, err := f.s.SubmitEvidence(context.Background(), f.a.Case.CaseID, workflow.SubmitEvidenceCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-evidence-01", ActorID: "editor", ExpectedRevision: f.a.Case.Revision}, FindingID: findingID, ChangeSummary: "增加有意义的替代文本", BeforeDigest: digest("b"), AfterDigest: digest("c"), VerificationResult: "PASS"})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	a, _, err = f.s.SubmitReview(context.Background(), f.a.Case.CaseID, workflow.SubmitReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-submit-review", ActorID: "owner", ExpectedRevision: f.a.Case.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	a, _, err = f.s.DecideReview(context.Background(), f.a.Case.CaseID, workflow.DecideReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-review-decision", ActorID: "reviewer", ExpectedRevision: f.a.Case.Revision}, Outcome: "APPROVE", Reason: "证据满足基线"})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	a, _, err = f.s.IssueRelease(context.Background(), f.a.Case.CaseID, workflow.IssueReleaseCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-release-01", ActorID: "publisher", ExpectedRevision: f.a.Case.Revision}, ValidHours: 24})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	a, _, err = f.s.Archive(context.Background(), f.a.Case.CaseID, workflow.ArchiveCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-archive-01", ActorID: "archivist", ExpectedRevision: f.a.Case.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	if a.Case.Status != accessibility.StatusArchived || a.Manifest == nil || a.Manifest.VerificationStatus != "VERIFIED" {
		t.Fatalf("归档结果错误: %+v", a)
	}
	if err := f.r.VerifyAll(context.Background()); err != nil {
		t.Fatalf("数据库完整性检查失败: %v", err)
	}
	view, err := f.s.GetCase(context.Background(), a.Case.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Audit.Valid || view.Audit.EventCount != 9 {
		t.Fatalf("审计链错误: %+v", view.Audit)
	}
}

func TestIdempotencyAndRevisionConflict(t *testing.T) {
	f := newFixture(t)
	f.create()
	cmd := workflow.CreateCaseCommand{RequestID: "request-create-01", Title: "测试出版物", Edition: "1.0", MediaFormat: "EPUB 3", OwnerID: "owner", ContentDigest: digest("a")}
	replayed, replay, err := f.s.CreateCase(context.Background(), cmd)
	if err != nil || !replay {
		t.Fatalf("应幂等重放: %v %v", replay, err)
	}
	if replayed.Case.CaseID != f.a.Case.CaseID {
		t.Fatal("幂等重放改变了 case_id")
	}
	cmd.Title = "不同出版物"
	_, _, err = f.s.CreateCase(context.Background(), cmd)
	assertCode(t, err, "IDEMPOTENCY_CONFLICT")
	f.freeze()
	_, _, err = f.s.AddFinding(context.Background(), f.a.Case.CaseID, workflow.AddFindingCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-stale-0001", ActorID: "auditor", ExpectedRevision: 1}, RuleCode: "1.1.1", ContentLocator: "x", Severity: "CRITICAL", Impact: "x"})
	assertCode(t, err, "REVISION_CONFLICT")
}

func TestBlockingAndSeparationPolicies(t *testing.T) {
	f := newFixture(t)
	f.create()
	f.freeze()
	findingID := f.finding()
	f.completeAudit()
	_, _, err := f.s.SubmitReview(context.Background(), f.a.Case.CaseID, workflow.SubmitReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-too-early", ActorID: "owner", ExpectedRevision: f.a.Case.Revision}})
	assertCode(t, err, "BLOCKERS_REMAIN")
	a, _, err := f.s.SubmitEvidence(context.Background(), f.a.Case.CaseID, workflow.SubmitEvidenceCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-evidence-02", ActorID: "editor", ExpectedRevision: f.a.Case.Revision}, FindingID: findingID, ChangeSummary: "修复", BeforeDigest: digest("b"), AfterDigest: digest("c"), VerificationResult: "PASS"})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	a, _, err = f.s.SubmitReview(context.Background(), f.a.Case.CaseID, workflow.SubmitReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-submit-0002", ActorID: "owner", ExpectedRevision: f.a.Case.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	_, _, err = f.s.DecideReview(context.Background(), f.a.Case.CaseID, workflow.DecideReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-owner-review", ActorID: "owner", ExpectedRevision: f.a.Case.Revision}, Outcome: "APPROVE", Reason: "不允许"})
	assertCode(t, err, "SEPARATION_CONFLICT")
	_, _, err = f.s.DecideReview(context.Background(), f.a.Case.CaseID, workflow.DecideReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-editor-review", ActorID: "editor", ExpectedRevision: f.a.Case.Revision}, Outcome: "APPROVE", Reason: "不允许"})
	assertCode(t, err, "SEPARATION_CONFLICT")
}

func assertCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望错误 %s", want)
	}
	var app *workflow.Error
	if !errors.As(err, &app) || app.Code != want {
		t.Fatalf("期望 %s，得到 %T %v", want, err, err)
	}
}
