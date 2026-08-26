package workflow

import (
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/audit"
)

type CommandMeta struct {
	RequestID        string `json:"request_id"`
	ActorID          string `json:"actor_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type CreateCaseCommand struct {
	RequestID     string `json:"request_id"`
	Title         string `json:"title"`
	Edition       string `json:"edition"`
	MediaFormat   string `json:"media_format"`
	OwnerID       string `json:"owner_id"`
	ContentDigest string `json:"content_digest"`
}

type FreezeProfileCommand struct {
	CommandMeta
	RulesetVersion     string   `json:"ruleset_version"`
	RuleCodes          []string `json:"rule_codes"`
	BlockingSeverities []string `json:"blocking_severities"`
}

type PreflightProfileCommand struct {
	ExpectedRevision   int64    `json:"expected_revision"`
	RulesetVersion     string   `json:"ruleset_version"`
	RuleCodes          []string `json:"rule_codes"`
	BlockingSeverities []string `json:"blocking_severities"`
}

type AddFindingCommand struct {
	CommandMeta
	RuleCode       string `json:"rule_code"`
	ContentLocator string `json:"content_locator"`
	Severity       string `json:"severity"`
	Impact         string `json:"impact"`
}

type FindingInput struct {
	RuleCode       string `json:"rule_code"`
	ContentLocator string `json:"content_locator"`
	Severity       string `json:"severity"`
	Impact         string `json:"impact"`
}

type AddFindingsCommand struct {
	CommandMeta
	Findings []FindingInput `json:"findings"`
}

type AddFindingsResult struct {
	Aggregate            accessibility.CaseAggregate          `json:"aggregate"`
	CreatedFindings      []accessibility.AccessibilityFinding `json:"created_findings"`
	CreatedCount         int                                  `json:"created_count"`
	SeverityDistribution map[accessibility.Severity]int       `json:"severity_distribution"`
	OpenFindings         int                                  `json:"open_findings"`
	BlockingOpen         int                                  `json:"blocking_open"`
}

type CompleteAuditCommand struct{ CommandMeta }

type SubmitEvidenceCommand struct {
	CommandMeta
	FindingID            string `json:"finding_id"`
	ChangeSummary        string `json:"change_summary"`
	BeforeDigest         string `json:"before_digest"`
	AfterDigest          string `json:"after_digest"`
	VerificationResult   string `json:"verification_result"`
	FailureReason        string `json:"failure_reason,omitempty"`
	SupersedesEvidenceID string `json:"supersedes_evidence_id,omitempty"`
}

type SubmitReviewCommand struct{ CommandMeta }

type DecideReviewCommand struct {
	CommandMeta
	Outcome          string                 `json:"outcome"`
	Reason           string                 `json:"reason"`
	RemediationItems []RemediationItemInput `json:"remediation_items,omitempty"`
}

type RemediationItemInput struct {
	FindingID   string `json:"finding_id"`
	Requirement string `json:"requirement"`
	Priority    string `json:"priority"`
}

type IssueReleaseCommand struct {
	CommandMeta
	ValidHours int `json:"valid_hours"`
}

type ArchiveCommand struct{ CommandMeta }

type VerifyReleaseCommand struct {
	ReleaseToken string `json:"release_token"`
}

type EvidenceTimeline struct {
	Finding  accessibility.AccessibilityFinding  `json:"finding"`
	Evidence []accessibility.RemediationEvidence `json:"evidence"`
}

type CaseView struct {
	Aggregate         accessibility.CaseAggregate `json:"aggregate"`
	Readiness         accessibility.Readiness     `json:"readiness"`
	OpenFindings      int                         `json:"open_findings"`
	BlockingOpen      int                         `json:"blocking_open"`
	NextActions       []string                    `json:"next_actions"`
	Audit             audit.Verification          `json:"audit"`
	EvidenceTimelines []EvidenceTimeline          `json:"evidence_timelines"`
}
