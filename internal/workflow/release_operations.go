package workflow

import (
	"context"
	"strings"
	"time"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/audit"
)

func (s *Service) SubmitEvidence(ctx context.Context, caseID string, cmd SubmitEvidenceCommand) (accessibility.CaseAggregate, bool, error) {
	raw, replay, err := s.update(ctx, caseID, cmd.CommandMeta, cmd, func(a *accessibility.CaseAggregate, events []audit.Event, now time.Time) (Mutation, error) {
		if a.Case.Status != accessibility.StatusRemediating {
			return Mutation{}, NewError("INVALID_STATE", "仅整改中可提交证据")
		}
		finding, _ := a.Finding(cmd.FindingID)
		if finding == nil {
			return Mutation{}, NewError("NOT_FOUND", "发现项不存在")
		}
		if strings.TrimSpace(cmd.ChangeSummary) == "" || !accessibility.ValidDigest(cmd.BeforeDigest) || !accessibility.ValidDigest(cmd.AfterDigest) {
			return Mutation{}, NewError("VALIDATION_ERROR", "修复说明及前后 SHA-256 摘要必须完整")
		}
		if strings.EqualFold(cmd.BeforeDigest, cmd.AfterDigest) {
			return Mutation{}, NewError("VALIDATION_ERROR", "修复前后摘要不能相同")
		}
		result := strings.ToUpper(cmd.VerificationResult)
		if result != "PASS" && result != "FAIL" {
			return Mutation{}, NewError("VALIDATION_ERROR", "verification_result 必须为 PASS 或 FAIL")
		}
		if result == "FAIL" && strings.TrimSpace(cmd.FailureReason) == "" {
			return Mutation{}, NewError("VALIDATION_ERROR", "FAIL 证据必须填写 failure_reason")
		}
		history := a.EvidenceHistory(finding.FindingID)
		round := len(history) + 1
		if len(history) == 0 {
			if strings.TrimSpace(cmd.SupersedesEvidenceID) != "" {
				return Mutation{}, NewError("EVIDENCE_CHAIN_BROKEN", "首轮证据不能指定 supersedes_evidence_id")
			}
		} else {
			latest := history[len(history)-1]
			if cmd.SupersedesEvidenceID != latest.EvidenceID {
				return Mutation{}, NewError("STALE_EVIDENCE", "supersedes_evidence_id 不是发现项当前最新证据")
			}
			if !strings.EqualFold(strings.TrimSpace(cmd.BeforeDigest), latest.AfterDigest) {
				return Mutation{}, NewError("EVIDENCE_CHAIN_BROKEN", "第 %d 轮 before_digest 必须等于上一轮 after_digest", round)
			}
		}
		evidenceID, err := newID("evidence")
		if err != nil {
			return Mutation{}, err
		}
		evidence := accessibility.RemediationEvidence{
			EvidenceID: evidenceID, FindingID: finding.FindingID, Round: round, SupersedesEvidenceID: strings.TrimSpace(cmd.SupersedesEvidenceID), ChangeSummary: strings.TrimSpace(cmd.ChangeSummary),
			BeforeDigest: strings.ToLower(cmd.BeforeDigest), AfterDigest: strings.ToLower(cmd.AfterDigest),
			VerificationResult: result, FailureReason: strings.TrimSpace(cmd.FailureReason), SubmittedBy: cmd.ActorID, SubmittedAt: now,
		}
		a.Evidences = append(a.Evidences, evidence)
		if result == "PASS" {
			finding.Status = accessibility.FindingResolved
			for i := range a.RemediationItems {
				item := &a.RemediationItems[i]
				if item.FindingID == finding.FindingID && item.Status == accessibility.RemediationItemPending {
					closed := now
					item.Status = accessibility.RemediationItemClosed
					item.ClosedByEvidenceID = evidence.EvidenceID
					item.ClosedAt = &closed
				}
			}
		} else {
			finding.Status = accessibility.FindingOpen
		}
		touch(a, now)
		event, err := eventFor(a, events, "REMEDIATION_EVIDENCE_VERSIONED", cmd.ActorID, evidence, now)
		return Mutation{Result: *a, Event: event}, err
	})
	if err != nil {
		return accessibility.CaseAggregate{}, replay, err
	}
	var result accessibility.CaseAggregate
	err = decodeResult(raw, &result)
	return result, replay, err
}

func (s *Service) SubmitReview(ctx context.Context, caseID string, cmd SubmitReviewCommand) (accessibility.CaseAggregate, bool, error) {
	raw, replay, err := s.update(ctx, caseID, cmd.CommandMeta, cmd, func(a *accessibility.CaseAggregate, events []audit.Event, now time.Time) (Mutation, error) {
		if cmd.ActorID != a.Case.OwnerID {
			return Mutation{}, NewError("ROLE_CONFLICT", "只有个案责任人可提交复核")
		}
		if err := a.CanSubmitReview(); err != nil {
			return Mutation{}, err
		}
		if err := a.Transition(accessibility.StatusReviewPending); err != nil {
			return Mutation{}, err
		}
		touch(a, now)
		event, err := eventFor(a, events, "REVIEW_SUBMITTED", cmd.ActorID, map[string]int{"evidence_count": len(a.Evidences)}, now)
		return Mutation{Result: *a, Event: event}, err
	})
	if err != nil {
		return accessibility.CaseAggregate{}, replay, err
	}
	var result accessibility.CaseAggregate
	err = decodeResult(raw, &result)
	return result, replay, err
}

func (s *Service) DecideReview(ctx context.Context, caseID string, cmd DecideReviewCommand) (accessibility.CaseAggregate, bool, error) {
	raw, replay, err := s.update(ctx, caseID, cmd.CommandMeta, cmd, func(a *accessibility.CaseAggregate, events []audit.Event, now time.Time) (Mutation, error) {
		if a.Case.Status != accessibility.StatusReviewPending {
			return Mutation{}, NewError("INVALID_STATE", "个案尚未等待复核")
		}
		if err := EnforceReviewer(a, cmd.ActorID); err != nil {
			return Mutation{}, err
		}
		outcome := strings.ToUpper(cmd.Outcome)
		if outcome != "APPROVE" && outcome != "RETURN" {
			return Mutation{}, NewError("VALIDATION_ERROR", "outcome 必须为 APPROVE 或 RETURN")
		}
		if strings.TrimSpace(cmd.Reason) == "" {
			return Mutation{}, NewError("VALIDATION_ERROR", "复核理由不能为空")
		}
		if outcome == "APPROVE" && len(cmd.RemediationItems) > 0 {
			return Mutation{}, NewError("VALIDATION_ERROR", "APPROVE 不能包含整改项")
		}
		if outcome == "RETURN" && len(cmd.RemediationItems) == 0 {
			return Mutation{}, NewError("VALIDATION_ERROR", "RETURN 至少需要一条整改项")
		}
		seenFindings := map[string]bool{}
		if outcome == "RETURN" {
			for i, item := range cmd.RemediationItems {
				finding, _ := a.Finding(item.FindingID)
				if finding == nil {
					return Mutation{}, NewDetailedError("REMEDIATION_ITEMS_INVALID", "整改清单校验失败", map[string]any{"index": i, "finding_id": item.FindingID, "code": "UNKNOWN_FINDING"})
				}
				if seenFindings[item.FindingID] {
					return Mutation{}, NewDetailedError("REMEDIATION_ITEMS_INVALID", "整改清单校验失败", map[string]any{"index": i, "finding_id": item.FindingID, "code": "DUPLICATE_FINDING"})
				}
				if strings.TrimSpace(item.Requirement) == "" || !accessibility.ValidateRemediationItemPriority(item.Priority) {
					return Mutation{}, NewDetailedError("REMEDIATION_ITEMS_INVALID", "整改清单校验失败", map[string]any{"index": i, "finding_id": item.FindingID, "code": "INVALID_ITEM"})
				}
				seenFindings[item.FindingID] = true
			}
		}
		previousReturnID := ""
		for i := len(a.Decisions) - 1; i >= 0; i-- {
			if a.Decisions[i].Outcome == "RETURN" {
				previousReturnID = a.Decisions[i].DecisionID
				break
			}
		}
		decisionID, err := newID("decision")
		if err != nil {
			return Mutation{}, err
		}
		decision := accessibility.ReviewDecision{
			DecisionID: decisionID, CaseID: caseID, ReviewerID: cmd.ActorID, Outcome: outcome,
			Reason: strings.TrimSpace(cmd.Reason), SeparationCheck: true, ReviewedRevision: a.Case.Revision, PreviousReturnDecisionID: previousReturnID, DecidedAt: now,
		}
		a.Decisions = append(a.Decisions, decision)
		if outcome == "APPROVE" {
			if err := a.CanSubmitReview(); err != nil && accessibility.ErrorCode(err) != "INVALID_STATE" {
				return Mutation{}, err
			}
			if err := a.Transition(accessibility.StatusApproved); err != nil {
				return Mutation{}, err
			}
		} else {
			for _, input := range cmd.RemediationItems {
				finding, _ := a.Finding(input.FindingID)
				finding.Status = accessibility.FindingOpen
				itemID, err := newID("remediation_item")
				if err != nil {
					return Mutation{}, err
				}
				a.RemediationItems = append(a.RemediationItems, accessibility.ReviewRemediationItem{ItemID: itemID, CaseID: caseID, DecisionID: decision.DecisionID, FindingID: input.FindingID, Requirement: strings.TrimSpace(input.Requirement), Priority: strings.ToUpper(strings.TrimSpace(input.Priority)), Status: accessibility.RemediationItemPending, CreatedAt: now})
			}
			if err := a.Transition(accessibility.StatusRemediating); err != nil {
				return Mutation{}, err
			}
		}
		touch(a, now)
		eventResult := map[string]any{"decision": decision, "remediation_items": a.RemediationItems}
		event, err := eventFor(a, events, "REVIEW_DECIDED", cmd.ActorID, eventResult, now)
		return Mutation{Result: *a, Event: event}, err
	})
	if err != nil {
		return accessibility.CaseAggregate{}, replay, err
	}
	var result accessibility.CaseAggregate
	err = decodeResult(raw, &result)
	return result, replay, err
}

func (s *Service) IssueRelease(ctx context.Context, caseID string, cmd IssueReleaseCommand) (accessibility.CaseAggregate, bool, error) {
	raw, replay, err := s.update(ctx, caseID, cmd.CommandMeta, cmd, func(a *accessibility.CaseAggregate, events []audit.Event, now time.Time) (Mutation, error) {
		if err := EnforceRelease(a); err != nil {
			return Mutation{}, err
		}
		if cmd.ValidHours < 1 || cmd.ValidHours > 24*30 {
			return Mutation{}, NewError("VALIDATION_ERROR", "有效期须在 1 到 720 小时之间")
		}
		credential := accessibility.ReleaseCredential{
			Token:  accessibility.CalculateReleaseToken(caseID, a.Profile.ProfileDigest, a.Case.ContentDigest, cmd.ActorID, a.Case.Revision),
			CaseID: caseID, BaselineDigest: a.Profile.ProfileDigest, ContentDigest: a.Case.ContentDigest,
			AuthorizedBy: cmd.ActorID, IssuedAt: now, ExpiresAt: now.Add(time.Duration(cmd.ValidHours) * time.Hour),
		}
		a.Release = &credential
		if err := a.Transition(accessibility.StatusReleased); err != nil {
			return Mutation{}, err
		}
		touch(a, now)
		event, err := eventFor(a, events, "RELEASE_ISSUED", cmd.ActorID, credential, now)
		return Mutation{Result: *a, Event: event}, err
	})
	if err != nil {
		return accessibility.CaseAggregate{}, replay, err
	}
	var result accessibility.CaseAggregate
	err = decodeResult(raw, &result)
	return result, replay, err
}

func (s *Service) Archive(ctx context.Context, caseID string, cmd ArchiveCommand) (accessibility.CaseAggregate, bool, error) {
	raw, replay, err := s.update(ctx, caseID, cmd.CommandMeta, cmd, func(a *accessibility.CaseAggregate, events []audit.Event, now time.Time) (Mutation, error) {
		if err := EnforceArchive(a, now); err != nil {
			return Mutation{}, err
		}
		verification := audit.Verify(events)
		if !verification.Valid {
			return Mutation{}, NewError("AUDIT_CHAIN_INVALID", verification.Diagnostic)
		}
		if err := a.Transition(accessibility.StatusArchived); err != nil {
			return Mutation{}, err
		}
		touch(a, now)
		manifestID, err := accessibility.DeterministicManifestID(*a)
		if err != nil {
			return Mutation{}, err
		}
		manifest := accessibility.ArchiveManifest{
			ManifestID: manifestID, CaseID: caseID, ReleaseToken: a.Release.Token,
			BaselineDigest: a.Profile.ProfileDigest, ContentDigest: a.Case.ContentDigest,
			EventChainHead: verification.ChainHead, VerificationStatus: "VERIFIED", ArchivedAt: now,
		}
		manifest.ManifestDigest = accessibility.CalculateManifestDigest(manifest)
		a.Manifest = &manifest
		event, err := eventFor(a, events, "CASE_ARCHIVED", cmd.ActorID, manifest, now)
		return Mutation{Result: *a, Event: event}, err
	})
	if err != nil {
		return accessibility.CaseAggregate{}, replay, err
	}
	var result accessibility.CaseAggregate
	err = decodeResult(raw, &result)
	return result, replay, err
}
