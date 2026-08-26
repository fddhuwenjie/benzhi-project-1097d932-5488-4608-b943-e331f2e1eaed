package workflow

import (
	"context"
	"sort"
	"strings"
	"time"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/audit"
)

const maxFindingBatch = 100

func (s *Service) PreflightProfile(ctx context.Context, caseID string, cmd PreflightProfileCommand) (accessibility.ProfilePreflight, error) {
	if cmd.ExpectedRevision < 1 {
		return accessibility.ProfilePreflight{}, NewError("VALIDATION_ERROR", "expected_revision 必须大于零")
	}
	a, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return accessibility.ProfilePreflight{}, WrapRule(err)
	}
	if a.Case.Revision != cmd.ExpectedRevision {
		return accessibility.ProfilePreflight{}, NewError("REVISION_CONFLICT", "预检修订号冲突：当前为 %d", a.Case.Revision)
	}
	if a.Case.Status != accessibility.StatusDraft {
		return accessibility.ProfilePreflight{}, NewError("INVALID_STATE", "仅 DRAFT 个案可预检基线")
	}
	return accessibility.PreflightProfile(a.Case.Revision, cmd.RulesetVersion, cmd.RuleCodes, cmd.BlockingSeverities), nil
}

func (s *Service) CreateCase(ctx context.Context, cmd CreateCaseCommand) (accessibility.CaseAggregate, bool, error) {
	if err := validateRequest(cmd.RequestID); err != nil {
		return accessibility.CaseAggregate{}, false, err
	}
	now := s.now().UTC()
	a := accessibility.CaseAggregate{Case: accessibility.PublicationCase{
		CaseID: newID("case"), Title: strings.TrimSpace(cmd.Title), Edition: strings.TrimSpace(cmd.Edition),
		MediaFormat: strings.TrimSpace(cmd.MediaFormat), OwnerID: strings.TrimSpace(cmd.OwnerID),
		ContentDigest: strings.ToLower(strings.TrimSpace(cmd.ContentDigest)), Status: accessibility.StatusDraft,
		Revision: 1, CreatedAt: now, UpdatedAt: now,
	}, Findings: []accessibility.AccessibilityFinding{}, Evidences: []accessibility.RemediationEvidence{}, Decisions: []accessibility.ReviewDecision{}}
	if err := accessibility.ValidateNewCase(a.Case); err != nil {
		return accessibility.CaseAggregate{}, false, WrapRule(err)
	}
	event, err := audit.NewEvent(a.Case.CaseID, newID("evt"), "CASE_CREATED", a.Case.OwnerID, 1, a.Case, audit.Event{}, now)
	if err != nil {
		return accessibility.CaseAggregate{}, false, err
	}
	hash, _ := payloadHash(cmd)
	raw, replay, err := s.repo.Create(ctx, cmd.RequestID, hash, CreateRecord{Aggregate: a, Event: event, Result: a})
	if err != nil {
		return accessibility.CaseAggregate{}, false, WrapRule(err)
	}
	var result accessibility.CaseAggregate
	if err := decodeResult(raw, &result); err != nil {
		return result, replay, err
	}
	return result, replay, nil
}

func (s *Service) FreezeProfile(ctx context.Context, caseID string, cmd FreezeProfileCommand) (accessibility.CaseAggregate, bool, error) {
	raw, replay, err := s.update(ctx, caseID, cmd.CommandMeta, cmd, func(a *accessibility.CaseAggregate, events []audit.Event, now time.Time) (Mutation, error) {
		if a.Case.Status != accessibility.StatusDraft {
			return Mutation{}, NewError("INVALID_STATE", "仅 DRAFT 个案可冻结基线")
		}
		preflight := accessibility.PreflightProfile(a.Case.Revision, cmd.RulesetVersion, cmd.RuleCodes, cmd.BlockingSeverities)
		if !preflight.CanFreeze {
			return Mutation{}, NewDetailedError("VALIDATION_ERROR", "候选要求基线未通过预检", preflight)
		}
		profile := accessibility.RequirementProfile{
			ProfileID: newID("profile"), CaseID: caseID, RulesetVersion: preflight.RulesetVersion,
			RuleCodes: preflight.RuleCodes, BlockingSeverities: preflight.BlockingSeverities,
			FrozenBy: cmd.ActorID, FrozenAt: now,
		}
		if cmd.ActorID != a.Case.OwnerID {
			return Mutation{}, NewError("ROLE_CONFLICT", "只有个案责任人可冻结基线")
		}
		if err := accessibility.ValidateProfile(profile); err != nil {
			return Mutation{}, err
		}
		profile.ProfileDigest = preflight.ProfileDigest
		a.Profile = &profile
		if err := a.Transition(accessibility.StatusProfileFrozen); err != nil {
			return Mutation{}, err
		}
		touch(a, now)
		event, err := eventFor(a, events, "PROFILE_FROZEN", cmd.ActorID, profile, now)
		return Mutation{Result: *a, Event: event}, err
	})
	if err != nil {
		return accessibility.CaseAggregate{}, replay, err
	}
	var result accessibility.CaseAggregate
	err = decodeResult(raw, &result)
	return result, replay, err
}

func (s *Service) AddFindings(ctx context.Context, caseID string, cmd AddFindingsCommand) (AddFindingsResult, bool, error) {
	raw, replay, err := s.update(ctx, caseID, cmd.CommandMeta, cmd, func(a *accessibility.CaseAggregate, events []audit.Event, now time.Time) (Mutation, error) {
		if a.Case.Status != accessibility.StatusProfileFrozen && a.Case.Status != accessibility.StatusAuditing {
			return Mutation{}, NewError("INVALID_STATE", "当前状态不能登记发现项")
		}
		candidates := make([]accessibility.FindingCandidate, len(cmd.Findings))
		for i, input := range cmd.Findings {
			candidates[i] = accessibility.FindingCandidate{RuleCode: input.RuleCode, ContentLocator: input.ContentLocator, Severity: accessibility.Severity(strings.ToUpper(strings.TrimSpace(input.Severity))), Impact: input.Impact}
		}
		if validation := accessibility.ValidateFindingBatch(a, candidates, maxFindingBatch); len(validation) > 0 {
			return Mutation{}, NewDetailedError("FINDING_BATCH_INVALID", "发现项批次校验失败", validation)
		}
		created := make([]accessibility.AccessibilityFinding, 0, len(candidates))
		distribution := map[accessibility.Severity]int{}
		for _, item := range candidates {
			finding := accessibility.AccessibilityFinding{FindingID: newID("finding"), CaseID: caseID, RuleCode: strings.ToUpper(strings.TrimSpace(item.RuleCode)), ContentLocator: strings.TrimSpace(item.ContentLocator), Severity: item.Severity, Impact: strings.TrimSpace(item.Impact), Status: accessibility.FindingOpen, ReportedBy: cmd.ActorID, ReportedAt: now}
			a.Findings = append(a.Findings, finding)
			created = append(created, finding)
			distribution[item.Severity]++
		}
		if a.Case.Status == accessibility.StatusProfileFrozen {
			if err := a.Transition(accessibility.StatusAuditing); err != nil {
				return Mutation{}, err
			}
		}
		touch(a, now)
		ids := make([]string, len(created))
		for i := range created {
			ids[i] = created[i].FindingID
		}
		sort.Strings(ids)
		eventData := map[string]any{"created_count": len(created), "severity_distribution": distribution, "finding_ids": ids}
		event, err := eventFor(a, events, "FINDINGS_BATCH_RECORDED", cmd.ActorID, eventData, now)
		result := AddFindingsResult{Aggregate: *a, CreatedFindings: created, CreatedCount: len(created), SeverityDistribution: distribution, OpenFindings: countOpen(a), BlockingOpen: a.BlockingOpenCount()}
		return Mutation{Result: result, Event: event}, err
	})
	if err != nil {
		return AddFindingsResult{}, replay, err
	}
	var result AddFindingsResult
	err = decodeResult(raw, &result)
	return result, replay, err
}

func countOpen(a *accessibility.CaseAggregate) int {
	count := 0
	for _, f := range a.Findings {
		if f.Status == accessibility.FindingOpen {
			count++
		}
	}
	return count
}

func (s *Service) AddFinding(ctx context.Context, caseID string, cmd AddFindingCommand) (accessibility.CaseAggregate, bool, error) {
	raw, replay, err := s.update(ctx, caseID, cmd.CommandMeta, cmd, func(a *accessibility.CaseAggregate, events []audit.Event, now time.Time) (Mutation, error) {
		if a.Case.Status != accessibility.StatusProfileFrozen && a.Case.Status != accessibility.StatusAuditing {
			return Mutation{}, NewError("INVALID_STATE", "当前状态不能登记发现项")
		}
		severity := accessibility.Severity(strings.ToUpper(cmd.Severity))
		if !accessibility.ValidSeverity(severity) {
			return Mutation{}, NewError("VALIDATION_ERROR", "严重度无效")
		}
		if !a.IsRuleAllowed(cmd.RuleCode) {
			return Mutation{}, NewError("RULE_NOT_IN_PROFILE", "发现项规则不在冻结基线中")
		}
		if strings.TrimSpace(cmd.ContentLocator) == "" || strings.TrimSpace(cmd.Impact) == "" {
			return Mutation{}, NewError("VALIDATION_ERROR", "定位和影响说明不能为空")
		}
		for _, existing := range a.Findings {
			if accessibility.FindingDuplicateKey(existing.RuleCode, existing.ContentLocator) == accessibility.FindingDuplicateKey(cmd.RuleCode, cmd.ContentLocator) {
				return Mutation{}, NewError("DUPLICATE_FINDING", "规则和内容定位与已有发现项重复")
			}
		}
		finding := accessibility.AccessibilityFinding{
			FindingID: newID("finding"), CaseID: caseID, RuleCode: strings.ToUpper(strings.TrimSpace(cmd.RuleCode)),
			ContentLocator: strings.TrimSpace(cmd.ContentLocator), Severity: severity, Impact: strings.TrimSpace(cmd.Impact),
			Status: accessibility.FindingOpen, ReportedBy: cmd.ActorID, ReportedAt: now,
		}
		a.Findings = append(a.Findings, finding)
		if a.Case.Status == accessibility.StatusProfileFrozen {
			if err := a.Transition(accessibility.StatusAuditing); err != nil {
				return Mutation{}, err
			}
		}
		touch(a, now)
		event, err := eventFor(a, events, "FINDING_RECORDED", cmd.ActorID, finding, now)
		return Mutation{Result: *a, Event: event}, err
	})
	if err != nil {
		return accessibility.CaseAggregate{}, replay, err
	}
	var result accessibility.CaseAggregate
	err = decodeResult(raw, &result)
	return result, replay, err
}

func (s *Service) CompleteAudit(ctx context.Context, caseID string, cmd CompleteAuditCommand) (accessibility.CaseAggregate, bool, error) {
	raw, replay, err := s.update(ctx, caseID, cmd.CommandMeta, cmd, func(a *accessibility.CaseAggregate, events []audit.Event, now time.Time) (Mutation, error) {
		if a.Case.Status != accessibility.StatusProfileFrozen && a.Case.Status != accessibility.StatusAuditing {
			return Mutation{}, NewError("INVALID_STATE", "当前状态不能完成审查")
		}
		if err := a.Transition(accessibility.StatusRemediating); err != nil {
			return Mutation{}, err
		}
		touch(a, now)
		event, err := eventFor(a, events, "AUDIT_COMPLETED", cmd.ActorID, map[string]int{"finding_count": len(a.Findings)}, now)
		return Mutation{Result: *a, Event: event}, err
	})
	if err != nil {
		return accessibility.CaseAggregate{}, replay, err
	}
	var result accessibility.CaseAggregate
	err = decodeResult(raw, &result)
	return result, replay, err
}
