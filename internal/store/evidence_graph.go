package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
)

func syncEvidenceGraph(ctx context.Context, tx *sql.Tx, a *accessibility.CaseAggregate) error {
	caseID := a.Case.CaseID
	for _, statement := range []string{
		`DELETE FROM review_remediation_items WHERE case_id=?`,
		`DELETE FROM remediation_evidence WHERE finding_id IN (SELECT finding_id FROM accessibility_findings WHERE case_id=?)`,
		`DELETE FROM accessibility_findings WHERE case_id=?`, `DELETE FROM review_decisions WHERE case_id=?`,
		`DELETE FROM requirement_profiles WHERE case_id=?`, `DELETE FROM release_credentials WHERE case_id=?`,
		`DELETE FROM archive_manifests WHERE case_id=?`,
	} {
		if _, err := tx.ExecContext(ctx, statement, caseID); err != nil {
			return err
		}
	}
	if a.Profile != nil {
		rules, _ := json.Marshal(a.Profile.RuleCodes)
		severity, _ := json.Marshal(a.Profile.BlockingSeverities)
		if _, err := tx.ExecContext(ctx, `INSERT INTO requirement_profiles(profile_id,case_id,ruleset_version,rule_codes_json,blocking_severities_json,profile_digest,frozen_by,frozen_at) VALUES(?,?,?,?,?,?,?,?)`, a.Profile.ProfileID, caseID, a.Profile.RulesetVersion, rules, severity, a.Profile.ProfileDigest, a.Profile.FrozenBy, a.Profile.FrozenAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	for _, f := range a.Findings {
		if _, err := tx.ExecContext(ctx, `INSERT INTO accessibility_findings(finding_id,case_id,rule_code,content_locator,severity,impact,status,reported_by,reported_at) VALUES(?,?,?,?,?,?,?,?,?)`, f.FindingID, caseID, f.RuleCode, f.ContentLocator, f.Severity, f.Impact, f.Status, f.ReportedBy, f.ReportedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	for _, e := range a.Evidences {
		if _, err := tx.ExecContext(ctx, `INSERT INTO remediation_evidence(evidence_id,finding_id,round,supersedes_evidence_id,change_summary,before_digest,after_digest,verification_result,failure_reason,submitted_by,submitted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, e.EvidenceID, e.FindingID, e.Round, e.SupersedesEvidenceID, e.ChangeSummary, e.BeforeDigest, e.AfterDigest, e.VerificationResult, e.FailureReason, e.SubmittedBy, e.SubmittedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	for _, d := range a.Decisions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_decisions(decision_id,case_id,reviewer_id,outcome,reason,separation_check,reviewed_revision,previous_return_decision_id,decided_at) VALUES(?,?,?,?,?,?,?,?,?)`, d.DecisionID, caseID, d.ReviewerID, d.Outcome, d.Reason, d.SeparationCheck, d.ReviewedRevision, d.PreviousReturnDecisionID, d.DecidedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	for _, item := range a.RemediationItems {
		closedAt := ""
		if item.ClosedAt != nil {
			closedAt = item.ClosedAt.Format(time.RFC3339Nano)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO review_remediation_items(item_id,case_id,decision_id,finding_id,requirement,priority,status,closed_by_evidence_id,created_at,closed_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, item.ItemID, caseID, item.DecisionID, item.FindingID, item.Requirement, item.Priority, item.Status, item.ClosedByEvidenceID, item.CreatedAt.Format(time.RFC3339Nano), closedAt); err != nil {
			return err
		}
	}
	if a.Release != nil {
		if _, err := tx.ExecContext(ctx, `INSERT INTO release_credentials(release_token,case_id,baseline_digest,content_digest,authorized_by,issued_at,expires_at) VALUES(?,?,?,?,?,?,?)`, a.Release.Token, caseID, a.Release.BaselineDigest, a.Release.ContentDigest, a.Release.AuthorizedBy, a.Release.IssuedAt.Format(time.RFC3339Nano), a.Release.ExpiresAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if a.Manifest != nil {
		if _, err := tx.ExecContext(ctx, `INSERT INTO archive_manifests(manifest_id,case_id,release_token,baseline_digest,content_digest,event_chain_head,manifest_digest,verification_status,archived_at) VALUES(?,?,?,?,?,?,?,?,?)`, a.Manifest.ManifestID, caseID, a.Manifest.ReleaseToken, a.Manifest.BaselineDigest, a.Manifest.ContentDigest, a.Manifest.EventChainHead, a.Manifest.ManifestDigest, a.Manifest.VerificationStatus, a.Manifest.ArchivedAt.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return nil
}
