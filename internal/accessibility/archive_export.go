package accessibility

import "benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/audit"

type ComponentVerification struct {
	Status          string `json:"status"`
	ErrorCode       string `json:"error_code,omitempty"`
	FailureSequence int64  `json:"failure_sequence,omitempty"`
	Diagnostic      string `json:"diagnostic,omitempty"`
}

type ArchiveExportBody struct {
	Case              PublicationCase         `json:"case"`
	Profile           RequirementProfile      `json:"profile"`
	Findings          []AccessibilityFinding  `json:"findings"`
	Evidence          []RemediationEvidence   `json:"remediation_evidence"`
	ReviewDecisions   []ReviewDecision        `json:"review_decisions"`
	RemediationItems  []ReviewRemediationItem `json:"review_remediation_items"`
	ReleaseCredential ReleaseCredential       `json:"release_credential"`
	ArchiveManifest   ArchiveManifest         `json:"archive_manifest"`
	AuditEvents       []audit.Event           `json:"audit_events"`
}

type ArchiveExport struct {
	Verified       bool                             `json:"verified"`
	ManifestDigest string                           `json:"manifest_digest"`
	Components     map[string]ComponentVerification `json:"components"`
	Evidence       ArchiveExportBody                `json:"evidence"`
}
