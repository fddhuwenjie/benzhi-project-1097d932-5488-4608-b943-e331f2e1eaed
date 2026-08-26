package accessibility

import (
	"fmt"
	"sort"
	"strings"
)

type Readiness struct {
	TotalFindings      int `json:"total_findings"`
	OpenFindings       int `json:"open_findings"`
	BlockingFindings   int `json:"blocking_findings"`
	BlockingOpen       int `json:"blocking_open"`
	ValidEvidence      int `json:"valid_evidence"`
	InvalidEvidence    int `json:"invalid_evidence"`
	ReviewDecisions    int `json:"review_decisions"`
	SeparationFailures int `json:"separation_failures"`
}

func (a *CaseAggregate) Readiness() Readiness {
	r := Readiness{TotalFindings: len(a.Findings), ReviewDecisions: len(a.Decisions)}
	for _, finding := range a.Findings {
		if finding.Status == FindingOpen {
			r.OpenFindings++
		}
		if a.IsBlocking(finding.Severity) {
			r.BlockingFindings++
			if finding.Status == FindingOpen {
				r.BlockingOpen++
			}
		}
	}
	for _, evidence := range a.Evidences {
		if EvidenceIsValid(evidence) {
			r.ValidEvidence++
		} else {
			r.InvalidEvidence++
		}
	}
	for _, decision := range a.Decisions {
		if !decision.SeparationCheck || decision.ReviewerID == a.Case.OwnerID {
			r.SeparationFailures++
			continue
		}
		for _, evidence := range a.Evidences {
			if evidence.SubmittedBy == decision.ReviewerID {
				r.SeparationFailures++
				break
			}
		}
	}
	return r
}

func EvidenceIsValid(e RemediationEvidence) bool {
	return strings.TrimSpace(e.ChangeSummary) != "" &&
		ValidDigest(e.BeforeDigest) && ValidDigest(e.AfterDigest) &&
		e.BeforeDigest != e.AfterDigest && e.VerificationResult == "PASS" &&
		strings.TrimSpace(e.SubmittedBy) != ""
}

func (a *CaseAggregate) Validate() error {
	if err := ValidateNewCase(a.Case); err != nil {
		return err
	}
	if a.Case.CaseID == "" || a.Case.Revision < 1 || a.Case.CreatedAt.IsZero() || a.Case.UpdatedAt.IsZero() {
		return NewRuleError("AGGREGATE_INVALID", "个案标识、修订号和时间戳必须完整")
	}
	if a.Case.UpdatedAt.Before(a.Case.CreatedAt) {
		return NewRuleError("AGGREGATE_INVALID", "更新时间不能早于创建时间")
	}
	if a.Case.Status != StatusDraft {
		if err := validateFrozenProfile(a); err != nil {
			return err
		}
	} else if a.Profile != nil {
		return NewRuleError("AGGREGATE_INVALID", "DRAFT 个案不能已有冻结基线")
	}
	if err := validateFindingsAndEvidence(a); err != nil {
		return err
	}
	if err := validateReviewHistory(a); err != nil {
		return err
	}
	if err := validateReleaseAndManifest(a); err != nil {
		return err
	}
	return nil
}

func validateFrozenProfile(a *CaseAggregate) error {
	if a.Profile == nil {
		return NewRuleError("AGGREGATE_INVALID", "非 DRAFT 个案必须包含冻结基线")
	}
	if a.Profile.CaseID != a.Case.CaseID || a.Profile.ProfileID == "" || a.Profile.FrozenAt.IsZero() {
		return NewRuleError("AGGREGATE_INVALID", "冻结基线归属或标识无效")
	}
	if err := ValidateProfile(*a.Profile); err != nil {
		return err
	}
	expected := CalculateProfileDigest(a.Profile.RulesetVersion, a.Profile.RuleCodes, a.Profile.BlockingSeverities)
	if a.Profile.ProfileDigest != expected {
		return NewRuleError("DIGEST_MISMATCH", "冻结基线摘要无法重复计算")
	}
	if !sort.StringsAreSorted(a.Profile.RuleCodes) {
		return NewRuleError("AGGREGATE_INVALID", "冻结规则代码必须按稳定顺序保存")
	}
	return nil
}

func validateFindingsAndEvidence(a *CaseAggregate) error {
	findings := make(map[string]AccessibilityFinding, len(a.Findings))
	for _, finding := range a.Findings {
		if finding.FindingID == "" || finding.CaseID != a.Case.CaseID {
			return NewRuleError("AGGREGATE_INVALID", "发现项归属或标识无效")
		}
		if _, duplicate := findings[finding.FindingID]; duplicate {
			return NewRuleError("AGGREGATE_INVALID", "发现项 %s 重复", finding.FindingID)
		}
		if !a.IsRuleAllowed(finding.RuleCode) || !ValidSeverity(finding.Severity) {
			return NewRuleError("AGGREGATE_INVALID", "发现项 %s 不符合冻结基线", finding.FindingID)
		}
		if strings.TrimSpace(finding.ContentLocator) == "" || strings.TrimSpace(finding.Impact) == "" || finding.ReportedAt.IsZero() {
			return NewRuleError("AGGREGATE_INVALID", "发现项 %s 内容不完整", finding.FindingID)
		}
		findings[finding.FindingID] = finding
	}
	evidenceIDs := map[string]bool{}
	histories := map[string][]RemediationEvidence{}
	for _, evidence := range a.Evidences {
		if evidence.EvidenceID == "" || evidenceIDs[evidence.EvidenceID] {
			return NewRuleError("AGGREGATE_INVALID", "修复证据标识为空或重复")
		}
		evidenceIDs[evidence.EvidenceID] = true
		if _, exists := findings[evidence.FindingID]; !exists {
			return NewRuleError("AGGREGATE_INVALID", "修复证据 %s 引用了未知发现项", evidence.EvidenceID)
		}
		if evidence.VerificationResult != "PASS" && evidence.VerificationResult != "FAIL" {
			return NewRuleError("AGGREGATE_INVALID", "修复证据 %s 的验证结论无效", evidence.EvidenceID)
		}
		if evidence.VerificationResult == "FAIL" && strings.TrimSpace(evidence.FailureReason) == "" {
			return NewRuleError("AGGREGATE_INVALID", "失败证据 %s 缺少失败原因", evidence.EvidenceID)
		}
		if !ValidDigest(evidence.BeforeDigest) || !ValidDigest(evidence.AfterDigest) || evidence.SubmittedAt.IsZero() {
			return NewRuleError("AGGREGATE_INVALID", "修复证据 %s 不完整", evidence.EvidenceID)
		}
		histories[evidence.FindingID] = append(histories[evidence.FindingID], evidence)
	}
	for findingID, history := range histories {
		if err := ValidateEvidenceChain(history); err != nil {
			return NewRuleError("EVIDENCE_CHAIN_BROKEN", "发现项 %s: %v", findingID, err)
		}
	}
	for _, finding := range a.Findings {
		if finding.Status == FindingResolved {
			evidence := a.EvidenceFor(finding.FindingID)
			if evidence == nil || !EvidenceIsValid(*evidence) {
				return NewRuleError("AGGREGATE_INVALID", "已解决发现项 %s 缺少有效的最新证据", finding.FindingID)
			}
		}
	}
	if err := validateRemediationItems(a, findings, evidenceIDs); err != nil {
		return err
	}
	return nil
}

func validateRemediationItems(a *CaseAggregate, findings map[string]AccessibilityFinding, evidenceIDs map[string]bool) error {
	seenItems := map[string]bool{}
	seenAssociations := map[string]bool{}
	decisionIDs := map[string]bool{}
	for _, d := range a.Decisions {
		decisionIDs[d.DecisionID] = true
	}
	for _, item := range a.RemediationItems {
		if item.ItemID == "" || seenItems[item.ItemID] || item.CaseID != a.Case.CaseID || !decisionIDs[item.DecisionID] {
			return NewRuleError("AGGREGATE_INVALID", "整改项标识、归属或复核决定无效")
		}
		seenItems[item.ItemID] = true
		association := item.DecisionID + "\x00" + item.FindingID
		if seenAssociations[association] {
			return NewRuleError("AGGREGATE_INVALID", "同一复核决定重复关联发现项 %s", item.FindingID)
		}
		seenAssociations[association] = true
		if _, ok := findings[item.FindingID]; !ok {
			return NewRuleError("AGGREGATE_INVALID", "整改项引用未知发现项 %s", item.FindingID)
		}
		if strings.TrimSpace(item.Requirement) == "" || !ValidateRemediationItemPriority(item.Priority) || item.CreatedAt.IsZero() {
			return NewRuleError("AGGREGATE_INVALID", "整改项 %s 字段不完整", item.ItemID)
		}
		if item.Status != RemediationItemPending && item.Status != RemediationItemClosed {
			return NewRuleError("AGGREGATE_INVALID", "整改项 %s 状态无效", item.ItemID)
		}
		if item.Status == RemediationItemClosed && !evidenceIDs[item.ClosedByEvidenceID] {
			return NewRuleError("AGGREGATE_INVALID", "整改项 %s 缺少关闭证据", item.ItemID)
		}
	}
	return nil
}

func validateReviewHistory(a *CaseAggregate) error {
	seen := map[string]bool{}
	for _, decision := range a.Decisions {
		if decision.DecisionID == "" || seen[decision.DecisionID] || decision.CaseID != a.Case.CaseID {
			return NewRuleError("AGGREGATE_INVALID", "复核决定标识重复或归属无效")
		}
		seen[decision.DecisionID] = true
		if decision.Outcome != "APPROVE" && decision.Outcome != "RETURN" {
			return NewRuleError("AGGREGATE_INVALID", "复核决定 %s 的结论无效", decision.DecisionID)
		}
		if !decision.SeparationCheck || decision.ReviewerID == a.Case.OwnerID || decision.DecidedAt.IsZero() {
			return NewRuleError("SEPARATION_CONFLICT", "复核决定 %s 未通过职责分离", decision.DecisionID)
		}
		for _, evidence := range a.Evidences {
			if evidence.SubmittedBy == decision.ReviewerID {
				return NewRuleError("SEPARATION_CONFLICT", "复核人提交过个案修复证据")
			}
		}
	}
	if a.Case.Status == StatusApproved || a.Case.Status == StatusReleased || a.Case.Status == StatusArchived {
		if len(a.Decisions) == 0 || a.Decisions[len(a.Decisions)-1].Outcome != "APPROVE" {
			return NewRuleError("AGGREGATE_INVALID", "批准、发布或归档个案必须有最终批准决定")
		}
	}
	return nil
}

func validateReleaseAndManifest(a *CaseAggregate) error {
	requiresRelease := a.Case.Status == StatusReleased || a.Case.Status == StatusArchived
	if requiresRelease && a.Release == nil {
		return NewRuleError("AGGREGATE_INVALID", "发布或归档个案缺少发布凭证")
	}
	if a.Release != nil {
		if a.Profile == nil || a.Release.CaseID != a.Case.CaseID || a.Release.BaselineDigest != a.Profile.ProfileDigest || a.Release.ContentDigest != a.Case.ContentDigest {
			return NewRuleError("DIGEST_MISMATCH", "发布凭证与个案摘要不一致")
		}
		if a.Release.Token == "" || a.Release.AuthorizedBy == "" || !a.Release.ExpiresAt.After(a.Release.IssuedAt) {
			return NewRuleError("AGGREGATE_INVALID", "发布凭证字段不完整")
		}
	}
	if a.Case.Status == StatusArchived && a.Manifest == nil {
		return NewRuleError("AGGREGATE_INVALID", "ARCHIVED 个案缺少归档清单")
	}
	if a.Manifest != nil {
		if a.Release == nil || a.Manifest.ReleaseToken != a.Release.Token || a.Manifest.CaseID != a.Case.CaseID {
			return NewRuleError("AGGREGATE_INVALID", "归档清单与发布凭证不匹配")
		}
		if a.Manifest.VerificationStatus != "VERIFIED" || a.Manifest.EventChainHead == "" {
			return NewRuleError("AGGREGATE_INVALID", "归档清单未通过完整性校验")
		}
		if a.Manifest.ManifestDigest != CalculateManifestDigest(*a.Manifest) {
			return NewRuleError("DIGEST_MISMATCH", fmt.Sprintf("归档清单 %s 摘要不一致", a.Manifest.ManifestID))
		}
		expectedID, err := DeterministicManifestID(*a)
		if err != nil || a.Manifest.ManifestID != expectedID {
			return NewRuleError("DIGEST_MISMATCH", "归档清单标识与冻结证据图不一致")
		}
	}
	return nil
}
