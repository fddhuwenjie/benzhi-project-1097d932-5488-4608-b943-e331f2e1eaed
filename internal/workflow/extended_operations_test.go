package workflow_test

import (
	"context"
	"encoding/json"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func TestProfilePreflightIsReadOnlyAndUsesFreezeNormalization(t *testing.T) {
	f := newFixture(t)
	f.create()
	ctx := context.Background()
	cmd := workflow.PreflightProfileCommand{ExpectedRevision: 1, RulesetVersion: "WCAG-2.2", RuleCodes: []string{" 2.1.1 ", "1.1.1", "2.1.1"}, BlockingSeverities: []string{"major", "critical"}}
	first, err := f.s.PreflightProfile(ctx, f.a.Case.CaseID, cmd)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.s.PreflightProfile(ctx, f.a.Case.CaseID, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !first.CanFreeze || first.ProfileDigest != second.ProfileDigest || len(first.RuleCodes) != 2 || first.RuleCodes[0] != "1.1.1" {
		t.Fatalf("预检规范化错误: %+v", first)
	}
	if first.RuleResults[2].Code != "DUPLICATE_RULE_CODE" || first.RuleResults[2].DuplicateOf == nil || *first.RuleResults[2].DuplicateOf != 0 {
		t.Fatalf("未定位重复输入: %+v", first.RuleResults)
	}
	view, err := f.s.GetCase(ctx, f.a.Case.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Aggregate.Case.Revision != 1 || view.Aggregate.Case.Status != accessibility.StatusDraft {
		t.Fatal("预检改变了个案")
	}
	f.freeze()
	_, err = f.s.PreflightProfile(ctx, f.a.Case.CaseID, cmd)
	assertCode(t, err, "REVISION_CONFLICT")
}

func TestFindingBatchAtomicAndIdempotent(t *testing.T) {
	f := newFixture(t)
	f.create()
	f.freeze()
	ctx := context.Background()
	cmd := workflow.AddFindingsCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-batch-findings", ActorID: "auditor", ExpectedRevision: f.a.Case.Revision}, Findings: []workflow.FindingInput{{RuleCode: "1.1.1", ContentLocator: "chapter.xhtml#a", Severity: "CRITICAL", Impact: "缺少说明"}, {RuleCode: "1.1.1", ContentLocator: "chapter.xhtml#b", Severity: "MAJOR", Impact: "结构不清"}, {RuleCode: "1.1.1", ContentLocator: "chapter.xhtml#c", Severity: "MINOR", Impact: "提示不足"}}}
	result, replay, err := f.s.AddFindings(ctx, f.a.Case.CaseID, cmd)
	if err != nil || replay {
		t.Fatalf("批量登记失败: %v", err)
	}
	if result.CreatedCount != 3 || result.Aggregate.Case.Revision != 3 || result.Aggregate.Case.Status != accessibility.StatusAuditing {
		t.Fatalf("批量结果错误: %+v", result)
	}
	replayed, replay, err := f.s.AddFindings(ctx, f.a.Case.CaseID, cmd)
	if err != nil || !replay || replayed.CreatedCount != 3 {
		t.Fatalf("批量幂等重放失败: %v", err)
	}
	cmd.Findings[1].Impact = "不同载荷"
	_, _, err = f.s.AddFindings(ctx, f.a.Case.CaseID, cmd)
	assertCode(t, err, "IDEMPOTENCY_CONFLICT")
	bad := workflow.AddFindingsCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-invalid-batch", ActorID: "auditor", ExpectedRevision: result.Aggregate.Case.Revision}, Findings: []workflow.FindingInput{{RuleCode: "1.1.1", ContentLocator: "new#a", Severity: "CRITICAL", Impact: "有效"}, {RuleCode: "9.9.9", ContentLocator: "new#b", Severity: "MAJOR", Impact: "无效"}}}
	_, _, err = f.s.AddFindings(ctx, f.a.Case.CaseID, bad)
	assertCode(t, err, "FINDING_BATCH_INVALID")
	view, _ := f.s.GetCase(ctx, f.a.Case.CaseID)
	if len(view.Aggregate.Findings) != 3 || view.Aggregate.Case.Revision != 3 {
		t.Fatal("无效批次发生了部分写入")
	}
}

func TestEvidenceVersionsAndReturnChecklistClosure(t *testing.T) {
	f := newFixture(t)
	f.create()
	f.freeze()
	findingID := f.finding()
	f.completeAudit()
	ctx := context.Background()
	fail, _, err := f.s.SubmitEvidence(ctx, f.a.Case.CaseID, workflow.SubmitEvidenceCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-fail-evidence", ActorID: "editor", ExpectedRevision: f.a.Case.Revision}, FindingID: findingID, ChangeSummary: "首次尝试", BeforeDigest: digest("b"), AfterDigest: digest("c"), VerificationResult: "FAIL", FailureReason: "自动检查仍失败"})
	if err != nil {
		t.Fatal(err)
	}
	f.a = fail
	first := f.a.Evidences[0]
	if first.Round != 1 || f.a.Findings[0].Status != accessibility.FindingOpen {
		t.Fatal("FAIL 证据状态错误")
	}
	_, _, err = f.s.SubmitEvidence(ctx, f.a.Case.CaseID, workflow.SubmitEvidenceCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-broken-evidence", ActorID: "editor", ExpectedRevision: f.a.Case.Revision}, FindingID: findingID, ChangeSummary: "断链", BeforeDigest: digest("d"), AfterDigest: digest("e"), VerificationResult: "PASS", SupersedesEvidenceID: first.EvidenceID})
	assertCode(t, err, "EVIDENCE_CHAIN_BROKEN")
	pass, _, err := f.s.SubmitEvidence(ctx, f.a.Case.CaseID, workflow.SubmitEvidenceCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-pass-evidence", ActorID: "editor", ExpectedRevision: f.a.Case.Revision}, FindingID: findingID, ChangeSummary: "继续修复", BeforeDigest: digest("c"), AfterDigest: digest("d"), VerificationResult: "PASS", SupersedesEvidenceID: first.EvidenceID})
	if err != nil {
		t.Fatal(err)
	}
	f.a = pass
	if len(f.a.Evidences) != 2 || f.a.Evidences[1].Round != 2 || f.a.Findings[0].Status != accessibility.FindingResolved {
		t.Fatal("PASS 证据未形成连续版本")
	}
	a, _, err := f.s.SubmitReview(ctx, f.a.Case.CaseID, workflow.SubmitReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-first-review", ActorID: "owner", ExpectedRevision: f.a.Case.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	a, _, err = f.s.DecideReview(ctx, f.a.Case.CaseID, workflow.DecideReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-return-review", ActorID: "reviewer", ExpectedRevision: f.a.Case.Revision}, Outcome: "RETURN", Reason: "仍需调整", RemediationItems: []workflow.RemediationItemInput{{FindingID: findingID, Requirement: "补充人工复测", Priority: "HIGH"}}})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	if f.a.Findings[0].Status != accessibility.FindingOpen || len(f.a.PendingRemediationItems()) != 1 {
		t.Fatal("退回未重新打开指定发现项")
	}
	_, _, err = f.s.SubmitReview(ctx, f.a.Case.CaseID, workflow.SubmitReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-review-too-soon", ActorID: "owner", ExpectedRevision: f.a.Case.Revision}})
	assertCode(t, err, "REMEDIATION_ITEMS_PENDING")
	latest := f.a.Evidences[1]
	a, _, err = f.s.SubmitEvidence(ctx, f.a.Case.CaseID, workflow.SubmitEvidenceCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-close-return", ActorID: "editor", ExpectedRevision: f.a.Case.Revision}, FindingID: findingID, ChangeSummary: "完成人工复测", BeforeDigest: latest.AfterDigest, AfterDigest: digest("e"), VerificationResult: "PASS", SupersedesEvidenceID: latest.EvidenceID})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	if len(f.a.PendingRemediationItems()) != 0 || f.a.RemediationItems[0].ClosedByEvidenceID == "" {
		t.Fatal("新 PASS 证据未关闭整改项")
	}
}

func TestReleaseVerificationAndDeterministicArchiveExport(t *testing.T) {
	f := newFixture(t)
	f.create()
	f.freeze()
	findingID := f.finding()
	f.completeAudit()
	ctx := context.Background()
	a, _, err := f.s.SubmitEvidence(ctx, f.a.Case.CaseID, workflow.SubmitEvidenceCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-export-proof", ActorID: "editor", ExpectedRevision: f.a.Case.Revision}, FindingID: findingID, ChangeSummary: "完成修复", BeforeDigest: digest("b"), AfterDigest: digest("c"), VerificationResult: "PASS"})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	a, _, err = f.s.SubmitReview(ctx, f.a.Case.CaseID, workflow.SubmitReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-export-submit", ActorID: "owner", ExpectedRevision: f.a.Case.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	a, _, err = f.s.DecideReview(ctx, f.a.Case.CaseID, workflow.DecideReviewCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-export-approve", ActorID: "reviewer", ExpectedRevision: f.a.Case.Revision}, Outcome: "APPROVE", Reason: "通过"})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	a, _, err = f.s.IssueRelease(ctx, f.a.Case.CaseID, workflow.IssueReleaseCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-export-release", ActorID: "publisher", ExpectedRevision: f.a.Case.Revision}, ValidHours: 24})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	eventsBefore, _ := f.s.Events(ctx, f.a.Case.CaseID)
	verification, err := f.s.VerifyRelease(ctx, f.a.Case.CaseID, workflow.VerifyReleaseCommand{ReleaseToken: f.a.Release.Token})
	if err != nil || verification.Status != "VALID" || verification.RemainingSeconds <= 0 {
		t.Fatalf("凭证核验失败: %+v %v", verification, err)
	}
	mismatch, _ := f.s.VerifyRelease(ctx, f.a.Case.CaseID, workflow.VerifyReleaseCommand{ReleaseToken: "rel_other"})
	if mismatch.Status != "MISMATCH" || mismatch.AuthorizedBy != "" {
		t.Fatalf("错用令牌泄露了凭证详情: %+v", mismatch)
	}
	eventsAfter, _ := f.s.Events(ctx, f.a.Case.CaseID)
	if len(eventsBefore) != len(eventsAfter) {
		t.Fatal("凭证核验改变了审计链")
	}
	a, _, err = f.s.Archive(ctx, f.a.Case.CaseID, workflow.ArchiveCommand{CommandMeta: workflow.CommandMeta{RequestID: "request-export-archive", ActorID: "archivist", ExpectedRevision: f.a.Case.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	f.a = a
	one, err := f.s.ExportArchive(ctx, f.a.Case.CaseID)
	if err != nil || !one.Verified {
		t.Fatalf("归档导出未验证: %+v %v", one.Components, err)
	}
	two, err := f.s.ExportArchive(ctx, f.a.Case.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	rawOne, _ := json.Marshal(one)
	rawTwo, _ := json.Marshal(two)
	if string(rawOne) != string(rawTwo) {
		t.Fatal("相同归档的导出字节不稳定")
	}
}
