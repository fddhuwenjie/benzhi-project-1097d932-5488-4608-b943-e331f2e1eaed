package accessibility

import (
	"encoding/json"
	"sort"
)

type ArchiveEvidence struct {
	Case             PublicationCase         `json:"case"`
	Profile          RequirementProfile      `json:"profile"`
	Findings         []AccessibilityFinding  `json:"findings"`
	Evidences        []RemediationEvidence   `json:"remediation_evidence"`
	Decisions        []ReviewDecision        `json:"review_decisions"`
	RemediationItems []ReviewRemediationItem `json:"review_remediation_items"`
	Release          ReleaseCredential       `json:"release_credential"`
}

func BuildArchiveEvidence(a CaseAggregate) (ArchiveEvidence, error) {
	if a.Profile == nil || a.Release == nil {
		return ArchiveEvidence{}, NewRuleError("INVALID_STATE", "生成归档证据前必须冻结基线并签发发布凭证")
	}
	evidence := ArchiveEvidence{
		Case: a.Case, Profile: *a.Profile, Release: *a.Release,
		Findings:         append([]AccessibilityFinding(nil), a.Findings...),
		Evidences:        append([]RemediationEvidence(nil), a.Evidences...),
		Decisions:        append([]ReviewDecision(nil), a.Decisions...),
		RemediationItems: append([]ReviewRemediationItem(nil), a.RemediationItems...),
	}
	sort.Slice(evidence.Findings, func(i, j int) bool {
		if evidence.Findings[i].ReportedAt.Equal(evidence.Findings[j].ReportedAt) {
			return evidence.Findings[i].FindingID < evidence.Findings[j].FindingID
		}
		return evidence.Findings[i].ReportedAt.Before(evidence.Findings[j].ReportedAt)
	})
	sort.Slice(evidence.Evidences, func(i, j int) bool {
		left, right := evidence.Evidences[i], evidence.Evidences[j]
		if left.FindingID != right.FindingID {
			return left.FindingID < right.FindingID
		}
		if left.SubmittedAt.Equal(right.SubmittedAt) {
			return left.EvidenceID < right.EvidenceID
		}
		return left.SubmittedAt.Before(right.SubmittedAt)
	})
	sort.Slice(evidence.Decisions, func(i, j int) bool {
		if evidence.Decisions[i].DecidedAt.Equal(evidence.Decisions[j].DecidedAt) {
			return evidence.Decisions[i].DecisionID < evidence.Decisions[j].DecisionID
		}
		return evidence.Decisions[i].DecidedAt.Before(evidence.Decisions[j].DecidedAt)
	})
	sort.Slice(evidence.RemediationItems, func(i, j int) bool { return evidence.RemediationItems[i].ItemID < evidence.RemediationItems[j].ItemID })
	return evidence, nil
}

func CanonicalArchiveEvidence(a CaseAggregate) ([]byte, error) {
	evidence, err := BuildArchiveEvidence(a)
	if err != nil {
		return nil, err
	}
	return json.Marshal(evidence)
}

func ArchiveEvidenceDigest(a CaseAggregate) (string, error) {
	raw, err := CanonicalArchiveEvidence(a)
	if err != nil {
		return "", err
	}
	return DigestBytes(raw), nil
}

func DeterministicManifestID(a CaseAggregate) (string, error) {
	digest, err := ArchiveEvidenceDigest(a)
	if err != nil {
		return "", err
	}
	return "manifest_" + digest[:32], nil
}
