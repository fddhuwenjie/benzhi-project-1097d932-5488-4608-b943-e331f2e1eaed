package accessibility

import "strings"

func ValidateNewCase(c PublicationCase) error {
	if strings.TrimSpace(c.Title) == "" || strings.TrimSpace(c.Edition) == "" {
		return NewRuleError("VALIDATION_ERROR", "书名和版本不能为空")
	}
	if strings.TrimSpace(c.MediaFormat) == "" || strings.TrimSpace(c.OwnerID) == "" {
		return NewRuleError("VALIDATION_ERROR", "载体格式和责任人不能为空")
	}
	if !ValidDigest(c.ContentDigest) {
		return NewRuleError("VALIDATION_ERROR", "content_digest 必须是 SHA-256 十六进制摘要")
	}
	return nil
}

func ValidateProfile(p RequirementProfile) error {
	if strings.TrimSpace(p.RulesetVersion) == "" || len(p.RuleCodes) == 0 {
		return NewRuleError("VALIDATION_ERROR", "规则版本和规则代码不能为空")
	}
	if len(p.BlockingSeverities) == 0 {
		return NewRuleError("VALIDATION_ERROR", "至少选择一个阻断严重度")
	}
	for _, s := range p.BlockingSeverities {
		if !ValidSeverity(s) {
			return NewRuleError("VALIDATION_ERROR", "未知严重度 %s", s)
		}
	}
	return nil
}

func ValidSeverity(s Severity) bool {
	return s == SeverityCritical || s == SeverityMajor || s == SeverityMinor
}

func (a *CaseAggregate) IsRuleAllowed(code string) bool {
	if a.Profile == nil {
		return false
	}
	for _, allowed := range a.Profile.RuleCodes {
		if strings.EqualFold(allowed, code) {
			return true
		}
	}
	return false
}

func (a *CaseAggregate) IsBlocking(s Severity) bool {
	if a.Profile == nil {
		return false
	}
	for _, blocking := range a.Profile.BlockingSeverities {
		if s == blocking {
			return true
		}
	}
	return false
}

func (a *CaseAggregate) Finding(id string) (*AccessibilityFinding, int) {
	for i := range a.Findings {
		if a.Findings[i].FindingID == id {
			return &a.Findings[i], i
		}
	}
	return nil, -1
}

func (a *CaseAggregate) EvidenceFor(findingID string) *RemediationEvidence {
	for i := len(a.Evidences) - 1; i >= 0; i-- {
		if a.Evidences[i].FindingID == findingID {
			return &a.Evidences[i]
		}
	}
	return nil
}

func (a *CaseAggregate) BlockingOpenCount() int {
	count := 0
	for _, f := range a.Findings {
		if a.IsBlocking(f.Severity) && f.Status != FindingResolved {
			count++
		}
	}
	return count
}

func (a *CaseAggregate) CanSubmitReview() error {
	if a.Case.Status != StatusRemediating {
		return NewRuleError("INVALID_STATE", "仅整改中的个案可提交复核")
	}
	if pending := a.PendingRemediationItems(); len(pending) > 0 {
		ids := make([]string, len(pending))
		for i := range pending {
			ids[i] = pending[i].FindingID
		}
		return NewRuleError("REMEDIATION_ITEMS_PENDING", "本轮整改项尚未闭环：%s", strings.Join(ids, ","))
	}
	if a.BlockingOpenCount() > 0 {
		return NewRuleError("BLOCKERS_REMAIN", "仍有阻断项未关闭")
	}
	for _, f := range a.Findings {
		if a.IsBlocking(f.Severity) {
			history := a.EvidenceHistory(f.FindingID)
			if err := ValidateEvidenceChain(history); err != nil {
				return err
			}
			e := a.EvidenceFor(f.FindingID)
			if e == nil || e.VerificationResult != "PASS" || !ValidDigest(e.BeforeDigest) || !ValidDigest(e.AfterDigest) {
				round := 0
				if e != nil {
					round = e.Round
				}
				return NewRuleError("INVALID_EVIDENCE", "阻断项 %s 最新第 %d 轮缺少有效 PASS 证据", f.FindingID, round)
			}
		}
	}
	return nil
}
