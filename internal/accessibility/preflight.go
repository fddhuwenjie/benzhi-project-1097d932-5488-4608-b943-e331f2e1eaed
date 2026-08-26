package accessibility

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var ruleCodePattern = regexp.MustCompile(`^[A-Z0-9]+(?:[._-][A-Z0-9]+)*$`)
var rulesetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type RuleCandidateResult struct {
	Index       int    `json:"index"`
	Input       string `json:"input"`
	Normalized  string `json:"normalized,omitempty"`
	Valid       bool   `json:"valid"`
	Code        string `json:"code,omitempty"`
	Message     string `json:"message,omitempty"`
	DuplicateOf *int   `json:"duplicate_of,omitempty"`
}

type ProfilePreflight struct {
	ExpectedRevision   int64                 `json:"expected_revision"`
	RulesetVersion     string                `json:"ruleset_version"`
	RuleResults        []RuleCandidateResult `json:"rule_results"`
	RuleCodes          []string              `json:"rule_codes"`
	BlockingSeverities []Severity            `json:"blocking_severities"`
	RuleCount          int                   `json:"rule_count"`
	BlockingErrors     []string              `json:"blocking_errors"`
	ThresholdHints     []string              `json:"threshold_hints"`
	ProfileDigest      string                `json:"profile_digest,omitempty"`
	CanFreeze          bool                  `json:"can_freeze"`
}

func PreflightProfile(revision int64, version string, candidates []string, severityInputs []string) ProfilePreflight {
	result := ProfilePreflight{ExpectedRevision: revision, RulesetVersion: strings.TrimSpace(version), RuleResults: make([]RuleCandidateResult, 0, len(candidates)), BlockingErrors: []string{}, ThresholdHints: []string{}}
	seen := map[string]int{}
	for i, input := range candidates {
		normalized := strings.ToUpper(strings.TrimSpace(input))
		item := RuleCandidateResult{Index: i, Input: input, Normalized: normalized, Valid: true}
		switch {
		case normalized == "":
			item.Valid, item.Code, item.Message = false, "EMPTY_RULE_CODE", "规则代码去空白后为空"
			result.BlockingErrors = append(result.BlockingErrors, fmt.Sprintf("rule_codes[%d] 不能为空", i))
		case !ruleCodePattern.MatchString(normalized):
			item.Valid, item.Code, item.Message = false, "INVALID_RULE_CODE", "规则代码格式无效"
			result.BlockingErrors = append(result.BlockingErrors, fmt.Sprintf("rule_codes[%d] 格式无效", i))
		case seen[normalized] >= 0:
			if previous, exists := seen[normalized]; exists {
				item.Valid, item.Code, item.Message, item.DuplicateOf = false, "DUPLICATE_RULE_CODE", "规则代码重复", &previous
			}
		}
		if normalized != "" && ruleCodePattern.MatchString(normalized) {
			if _, exists := seen[normalized]; !exists {
				seen[normalized] = i
			}
		}
		result.RuleResults = append(result.RuleResults, item)
	}
	result.RuleCodes = NormalizeCodes(candidates)
	if !rulesetPattern.MatchString(result.RulesetVersion) {
		result.BlockingErrors = append(result.BlockingErrors, "ruleset_version 格式无效")
	}
	if len(result.RuleCodes) == 0 {
		result.BlockingErrors = append(result.BlockingErrors, "至少需要一个有效规则代码")
	}
	severitySeen := map[Severity]bool{}
	for i, value := range severityInputs {
		s := Severity(strings.ToUpper(strings.TrimSpace(value)))
		if !ValidSeverity(s) {
			result.BlockingErrors = append(result.BlockingErrors, fmt.Sprintf("blocking_severities[%d] 无效", i))
			continue
		}
		if !severitySeen[s] {
			severitySeen[s] = true
			result.BlockingSeverities = append(result.BlockingSeverities, s)
		}
	}
	sort.Slice(result.BlockingSeverities, func(i, j int) bool { return result.BlockingSeverities[i] < result.BlockingSeverities[j] })
	if len(result.BlockingSeverities) == 0 {
		result.BlockingErrors = append(result.BlockingErrors, "至少选择一个阻断严重度")
	}
	if !severitySeen[SeverityCritical] {
		result.ThresholdHints = append(result.ThresholdHints, "CRITICAL 未设为阻断，冻结前需负责人确认")
	}
	if !severitySeen[SeverityMajor] {
		result.ThresholdHints = append(result.ThresholdHints, "MAJOR 未设为阻断，冻结前需负责人确认")
	}
	result.RuleCount = len(result.RuleCodes)
	result.CanFreeze = len(result.BlockingErrors) == 0
	if result.CanFreeze {
		result.ProfileDigest = CalculateProfileDigest(result.RulesetVersion, result.RuleCodes, result.BlockingSeverities)
	}
	return result
}

func NormalizeLocator(locator string) string { return strings.ToLower(strings.TrimSpace(locator)) }
func FindingDuplicateKey(ruleCode, locator string) string {
	return strings.ToUpper(strings.TrimSpace(ruleCode)) + "\x00" + NormalizeLocator(locator)
}
