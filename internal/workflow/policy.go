package workflow

import (
	"strings"
	"time"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
)

type ReviewPolicyResult struct {
	Allowed                bool     `json:"allowed"`
	ReviewerIsOwner        bool     `json:"reviewer_is_owner"`
	ReviewerSubmittedProof bool     `json:"reviewer_submitted_proof"`
	BlockingReasons        []string `json:"blocking_reasons"`
}

func EvaluateReviewer(a *accessibility.CaseAggregate, reviewerID string) ReviewPolicyResult {
	result := ReviewPolicyResult{Allowed: true, BlockingReasons: []string{}}
	reviewerID = strings.TrimSpace(reviewerID)
	if reviewerID == "" {
		result.Allowed = false
		result.BlockingReasons = append(result.BlockingReasons, "复核员身份不能为空")
		return result
	}
	if reviewerID == a.Case.OwnerID {
		result.Allowed = false
		result.ReviewerIsOwner = true
		result.BlockingReasons = append(result.BlockingReasons, "复核员是建档责任人")
	}
	for _, evidence := range a.Evidences {
		if evidence.SubmittedBy == reviewerID {
			result.Allowed = false
			result.ReviewerSubmittedProof = true
			result.BlockingReasons = append(result.BlockingReasons, "复核员提交过修复证据")
			break
		}
	}
	return result
}

func EnforceReviewer(a *accessibility.CaseAggregate, reviewerID string) error {
	result := EvaluateReviewer(a, reviewerID)
	if !result.Allowed {
		return NewError("SEPARATION_CONFLICT", "职责分离校验失败：%s", strings.Join(result.BlockingReasons, "；"))
	}
	return nil
}

type ReleasePolicyResult struct {
	Allowed          bool     `json:"allowed"`
	Approved         bool     `json:"approved"`
	BaselineValid    bool     `json:"baseline_valid"`
	ContentBound     bool     `json:"content_bound"`
	EvidenceComplete bool     `json:"evidence_complete"`
	Reasons          []string `json:"reasons"`
}

func EvaluateRelease(a *accessibility.CaseAggregate) ReleasePolicyResult {
	result := ReleasePolicyResult{Allowed: true, Reasons: []string{}}
	result.Approved = a.Case.Status == accessibility.StatusApproved && len(a.Decisions) > 0 && a.Decisions[len(a.Decisions)-1].Outcome == "APPROVE"
	if !result.Approved {
		result.Allowed = false
		result.Reasons = append(result.Reasons, "个案没有有效的最终批准决定")
	}
	if a.Profile != nil {
		expected := accessibility.CalculateProfileDigest(a.Profile.RulesetVersion, a.Profile.RuleCodes, a.Profile.BlockingSeverities)
		result.BaselineValid = expected == a.Profile.ProfileDigest
	}
	if !result.BaselineValid {
		result.Allowed = false
		result.Reasons = append(result.Reasons, "冻结基线摘要不一致")
	}
	result.ContentBound = accessibility.ValidDigest(a.Case.ContentDigest)
	if !result.ContentBound {
		result.Allowed = false
		result.Reasons = append(result.Reasons, "内容摘要无效")
	}
	readiness := a.Readiness()
	result.EvidenceComplete = blockingEvidenceComplete(a) && readiness.SeparationFailures == 0
	if !result.EvidenceComplete {
		result.Allowed = false
		result.Reasons = append(result.Reasons, "阻断项、证据或职责分离检查仍未满足")
	}
	return result
}

func blockingEvidenceComplete(a *accessibility.CaseAggregate) bool {
	for _, finding := range a.Findings {
		if !a.IsBlocking(finding.Severity) {
			continue
		}
		if finding.Status != accessibility.FindingResolved {
			return false
		}
		evidence := a.EvidenceFor(finding.FindingID)
		if evidence == nil || !accessibility.EvidenceIsValid(*evidence) {
			return false
		}
	}
	return true
}

func EnforceRelease(a *accessibility.CaseAggregate) error {
	result := EvaluateRelease(a)
	if !result.Allowed {
		return NewError("RELEASE_NOT_READY", "不具备发布资格：%s", strings.Join(result.Reasons, "；"))
	}
	return nil
}

func EnforceArchive(a *accessibility.CaseAggregate, at time.Time) error {
	if a.Case.Status != accessibility.StatusReleased || a.Release == nil || a.Profile == nil {
		return NewError("INVALID_STATE", "仅已发布且资料完整的个案可归档")
	}
	if !at.Before(a.Release.ExpiresAt) {
		return NewError("RELEASE_EXPIRED", "发布凭证已过有效期，不能归档")
	}
	verification := accessibility.VerifyRelease(*a, a.Release.Token, at)
	if verification.Status != "VALID" {
		return NewError("DIGEST_MISMATCH", "发布凭证权威核验失败：%s", strings.Join(verification.Diagnostics, "；"))
	}
	if a.Release.ContentDigest != a.Case.ContentDigest || a.Release.BaselineDigest != a.Profile.ProfileDigest {
		return NewError("DIGEST_MISMATCH", "发布凭证绑定的内容或基线已改变")
	}
	return nil
}
