package accessibility

import (
	"fmt"
	"strings"
	"time"
)

type FindingCandidate struct {
	RuleCode       string   `json:"rule_code"`
	ContentLocator string   `json:"content_locator"`
	Severity       Severity `json:"severity"`
	Impact         string   `json:"impact"`
}

type IndexedValidationError struct {
	Index   int    `json:"index"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func ValidateFindingBatch(a *CaseAggregate, items []FindingCandidate, limit int) []IndexedValidationError {
	errors := []IndexedValidationError{}
	if len(items) == 0 || len(items) > limit {
		return []IndexedValidationError{{Index: -1, Field: "findings", Code: "BATCH_SIZE_INVALID", Message: fmt.Sprintf("发现项数量必须为 1 到 %d", limit)}}
	}
	seen := map[string]int{}
	existing := map[string]bool{}
	for _, f := range a.Findings {
		existing[FindingDuplicateKey(f.RuleCode, f.ContentLocator)] = true
	}
	for i, item := range items {
		code := strings.ToUpper(strings.TrimSpace(item.RuleCode))
		locator := strings.TrimSpace(item.ContentLocator)
		key := FindingDuplicateKey(code, locator)
		if !a.IsRuleAllowed(code) {
			errors = append(errors, IndexedValidationError{i, "rule_code", "RULE_NOT_IN_PROFILE", "规则不在冻结基线中"})
		}
		if !ValidSeverity(item.Severity) {
			errors = append(errors, IndexedValidationError{i, "severity", "INVALID_SEVERITY", "严重度无效"})
		}
		if locator == "" {
			errors = append(errors, IndexedValidationError{i, "content_locator", "REQUIRED", "内容定位不能为空"})
		}
		if strings.TrimSpace(item.Impact) == "" {
			errors = append(errors, IndexedValidationError{i, "impact", "REQUIRED", "影响说明不能为空"})
		}
		if existing[key] {
			errors = append(errors, IndexedValidationError{i, "content_locator", "DUPLICATE_FINDING", "与个案现有发现项重复"})
		}
		if previous, ok := seen[key]; ok {
			errors = append(errors, IndexedValidationError{i, "content_locator", "DUPLICATE_IN_BATCH", fmt.Sprintf("与批次第 %d 项重复", previous)})
		} else {
			seen[key] = i
		}
	}
	return errors
}

func (a *CaseAggregate) EvidenceHistory(findingID string) []RemediationEvidence {
	result := []RemediationEvidence{}
	for _, e := range a.Evidences {
		if e.FindingID == findingID {
			result = append(result, e)
		}
	}
	return result
}

func ValidateEvidenceChain(history []RemediationEvidence) error {
	for i, e := range history {
		expectedRound := i + 1
		if e.Round != expectedRound {
			return NewRuleError("EVIDENCE_CHAIN_BROKEN", "发现项 %s 第 %d 轮的轮次不连续", e.FindingID, expectedRound)
		}
		if i == 0 {
			if e.SupersedesEvidenceID != "" {
				return NewRuleError("EVIDENCE_CHAIN_BROKEN", "发现项 %s 首轮证据不能替代其他证据", e.FindingID)
			}
		} else {
			previous := history[i-1]
			if e.SupersedesEvidenceID != previous.EvidenceID || !strings.EqualFold(e.BeforeDigest, previous.AfterDigest) {
				return NewRuleError("EVIDENCE_CHAIN_BROKEN", "发现项 %s 第 %d 轮证据与上一轮断链", e.FindingID, e.Round)
			}
		}
	}
	return nil
}

func ValidateRemediationItemPriority(priority string) bool {
	switch strings.ToUpper(strings.TrimSpace(priority)) {
	case "HIGH", "MEDIUM", "LOW":
		return true
	}
	return false
}

func (a *CaseAggregate) PendingRemediationItems() []ReviewRemediationItem {
	result := []ReviewRemediationItem{}
	for _, item := range a.RemediationItems {
		if item.Status == RemediationItemPending {
			result = append(result, item)
		}
	}
	return result
}

type ReleaseVerification struct {
	Status           string    `json:"status"`
	VerifiedAt       time.Time `json:"verified_at"`
	AuthorizedBy     string    `json:"authorized_by,omitempty"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	RemainingSeconds int64     `json:"remaining_seconds"`
	Diagnostics      []string  `json:"diagnostics"`
}

func VerifyRelease(a CaseAggregate, token string, now time.Time) ReleaseVerification {
	r := ReleaseVerification{Status: "MISMATCH", VerifiedAt: now.UTC(), Diagnostics: []string{}}
	if a.Release == nil {
		r.Diagnostics = append(r.Diagnostics, "发布凭证不存在")
		return r
	}
	c := a.Release
	if token != c.Token {
		r.Diagnostics = append(r.Diagnostics, "release_token 与当前个案凭证不匹配")
		return r
	}
	r.AuthorizedBy, r.ExpiresAt = c.AuthorizedBy, c.ExpiresAt
	if strings.TrimSpace(c.AuthorizedBy) == "" {
		r.Diagnostics = append(r.Diagnostics, "授权人为空")
	}
	if !c.ExpiresAt.After(c.IssuedAt) {
		r.Diagnostics = append(r.Diagnostics, "凭证时间顺序无效")
	}
	if a.Profile == nil || c.BaselineDigest != a.Profile.ProfileDigest {
		r.Diagnostics = append(r.Diagnostics, "冻结基线摘要不匹配")
	}
	if c.ContentDigest != a.Case.ContentDigest {
		r.Diagnostics = append(r.Diagnostics, "内容摘要不匹配")
	}
	if len(a.Decisions) == 0 || a.Decisions[len(a.Decisions)-1].Outcome != "APPROVE" {
		r.Diagnostics = append(r.Diagnostics, "最终复核决定不是有效批准")
	}
	if len(r.Diagnostics) > 0 {
		return r
	}
	if now.Before(c.IssuedAt) {
		r.Status = "NOT_YET_VALID"
		r.Diagnostics = append(r.Diagnostics, "尚未到签发时间")
		return r
	}
	if !now.Before(c.ExpiresAt) {
		r.Status = "EXPIRED"
		r.Diagnostics = append(r.Diagnostics, "凭证已过期")
		return r
	}
	r.Status = "VALID"
	r.RemainingSeconds = int64(c.ExpiresAt.Sub(now).Seconds())
	return r
}
