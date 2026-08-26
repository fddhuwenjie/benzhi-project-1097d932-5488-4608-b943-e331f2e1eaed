package store

import (
	"context"
	"encoding/json"
	"fmt"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func (r *SQLiteRepository) VerifyGraph(ctx context.Context, a accessibility.CaseAggregate) error {
	if err := r.verifyGraphCounts(ctx, a); err != nil {
		return err
	}
	caseID := a.Case.CaseID
	match := func(label, query string, args ...any) error {
		var count int
		if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return workflow.NewError("PERSISTENCE_INTEGRITY_ERROR", "个案 %s 的%s记录与聚合不一致", caseID, label)
		}
		return nil
	}
	if a.Profile != nil {
		rules, _ := json.Marshal(a.Profile.RuleCodes)
		severities, _ := json.Marshal(a.Profile.BlockingSeverities)
		if err := match("冻结基线", `SELECT COUNT(*) FROM requirement_profiles WHERE case_id=? AND profile_id=? AND ruleset_version=? AND rule_codes_json=? AND blocking_severities_json=? AND profile_digest=? AND frozen_by=? AND frozen_at=?`, caseID, a.Profile.ProfileID, a.Profile.RulesetVersion, rules, severities, a.Profile.ProfileDigest, a.Profile.FrozenBy, a.Profile.FrozenAt.Format("2006-01-02T15:04:05.999999999Z07:00")); err != nil {
			return err
		}
	}
	for _, f := range a.Findings {
		if err := match("发现项", `SELECT COUNT(*) FROM accessibility_findings WHERE case_id=? AND finding_id=? AND rule_code=? AND content_locator=? AND severity=? AND impact=? AND status=? AND reported_by=? AND reported_at=?`, caseID, f.FindingID, f.RuleCode, f.ContentLocator, f.Severity, f.Impact, f.Status, f.ReportedBy, f.ReportedAt.Format("2006-01-02T15:04:05.999999999Z07:00")); err != nil {
			return err
		}
	}
	for _, e := range a.Evidences {
		if err := match("修复证据", `SELECT COUNT(*) FROM remediation_evidence WHERE evidence_id=? AND finding_id=? AND round=? AND supersedes_evidence_id=? AND change_summary=? AND before_digest=? AND after_digest=? AND verification_result=? AND failure_reason=? AND submitted_by=? AND submitted_at=?`, e.EvidenceID, e.FindingID, e.Round, e.SupersedesEvidenceID, e.ChangeSummary, e.BeforeDigest, e.AfterDigest, e.VerificationResult, e.FailureReason, e.SubmittedBy, e.SubmittedAt.Format("2006-01-02T15:04:05.999999999Z07:00")); err != nil {
			return err
		}
	}
	for _, d := range a.Decisions {
		if err := match("复核决定", `SELECT COUNT(*) FROM review_decisions WHERE case_id=? AND decision_id=? AND reviewer_id=? AND outcome=? AND reason=? AND separation_check=? AND reviewed_revision=? AND previous_return_decision_id=? AND decided_at=?`, caseID, d.DecisionID, d.ReviewerID, d.Outcome, d.Reason, d.SeparationCheck, d.ReviewedRevision, d.PreviousReturnDecisionID, d.DecidedAt.Format("2006-01-02T15:04:05.999999999Z07:00")); err != nil {
			return err
		}
	}
	for _, i := range a.RemediationItems {
		closed := ""
		if i.ClosedAt != nil {
			closed = i.ClosedAt.Format("2006-01-02T15:04:05.999999999Z07:00")
		}
		if err := match("整改项", `SELECT COUNT(*) FROM review_remediation_items WHERE case_id=? AND item_id=? AND decision_id=? AND finding_id=? AND requirement=? AND priority=? AND status=? AND closed_by_evidence_id=? AND created_at=? AND closed_at=?`, caseID, i.ItemID, i.DecisionID, i.FindingID, i.Requirement, i.Priority, i.Status, i.ClosedByEvidenceID, i.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), closed); err != nil {
			return err
		}
	}
	if a.Release != nil {
		if err := match("发布凭证", `SELECT COUNT(*) FROM release_credentials WHERE case_id=? AND release_token=? AND baseline_digest=? AND content_digest=? AND authorized_by=? AND issued_at=? AND expires_at=?`, caseID, a.Release.Token, a.Release.BaselineDigest, a.Release.ContentDigest, a.Release.AuthorizedBy, a.Release.IssuedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), a.Release.ExpiresAt.Format("2006-01-02T15:04:05.999999999Z07:00")); err != nil {
			return err
		}
	}
	if a.Manifest != nil {
		if err := match("归档清单", `SELECT COUNT(*) FROM archive_manifests WHERE case_id=? AND manifest_id=? AND release_token=? AND baseline_digest=? AND content_digest=? AND event_chain_head=? AND manifest_digest=? AND verification_status=? AND archived_at=?`, caseID, a.Manifest.ManifestID, a.Manifest.ReleaseToken, a.Manifest.BaselineDigest, a.Manifest.ContentDigest, a.Manifest.EventChainHead, a.Manifest.ManifestDigest, a.Manifest.VerificationStatus, a.Manifest.ArchivedAt.Format("2006-01-02T15:04:05.999999999Z07:00")); err != nil {
			return err
		}
	}
	return nil
}

type graphCounts struct {
	profiles         int
	findings         int
	evidences        int
	decisions        int
	releases         int
	manifests        int
	remediationItems int
}

func (r *SQLiteRepository) verifyGraphCounts(ctx context.Context, a accessibility.CaseAggregate) error {
	actual, err := r.countEvidenceGraph(ctx, a.Case.CaseID)
	if err != nil {
		return err
	}
	expected := graphCounts{
		findings:         len(a.Findings),
		evidences:        len(a.Evidences),
		decisions:        len(a.Decisions),
		remediationItems: len(a.RemediationItems),
	}
	if a.Profile != nil {
		expected.profiles = 1
	}
	if a.Release != nil {
		expected.releases = 1
	}
	if a.Manifest != nil {
		expected.manifests = 1
	}
	if actual != expected {
		return workflow.NewError("PERSISTENCE_INTEGRITY_ERROR", "个案 %s 的证据图计数不一致：期望 %s，实际 %s", a.Case.CaseID, expected, actual)
	}
	return nil
}

func (r *SQLiteRepository) countEvidenceGraph(ctx context.Context, caseID string) (graphCounts, error) {
	var counts graphCounts
	queries := []struct {
		statement string
		target    *int
	}{
		{`SELECT COUNT(*) FROM requirement_profiles WHERE case_id=?`, &counts.profiles},
		{`SELECT COUNT(*) FROM accessibility_findings WHERE case_id=?`, &counts.findings},
		{`SELECT COUNT(*) FROM remediation_evidence WHERE finding_id IN (SELECT finding_id FROM accessibility_findings WHERE case_id=?)`, &counts.evidences},
		{`SELECT COUNT(*) FROM review_decisions WHERE case_id=?`, &counts.decisions},
		{`SELECT COUNT(*) FROM review_remediation_items WHERE case_id=?`, &counts.remediationItems},
		{`SELECT COUNT(*) FROM release_credentials WHERE case_id=?`, &counts.releases},
		{`SELECT COUNT(*) FROM archive_manifests WHERE case_id=?`, &counts.manifests},
	}
	for _, query := range queries {
		if err := r.db.QueryRowContext(ctx, query.statement, caseID).Scan(query.target); err != nil {
			return graphCounts{}, fmt.Errorf("读取证据图计数: %w", err)
		}
	}
	return counts, nil
}

func (c graphCounts) String() string {
	return fmt.Sprintf("profile=%d findings=%d evidences=%d decisions=%d remediation_items=%d releases=%d manifests=%d", c.profiles, c.findings, c.evidences, c.decisions, c.remediationItems, c.releases, c.manifests)
}
