package accessibility

import "time"

type CaseStatus string

const (
	StatusDraft         CaseStatus = "DRAFT"
	StatusProfileFrozen CaseStatus = "PROFILE_FROZEN"
	StatusAuditing      CaseStatus = "AUDITING"
	StatusRemediating   CaseStatus = "REMEDIATING"
	StatusReviewPending CaseStatus = "REVIEW_PENDING"
	StatusApproved      CaseStatus = "APPROVED"
	StatusReleased      CaseStatus = "RELEASED"
	StatusArchived      CaseStatus = "ARCHIVED"
)

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityMajor    Severity = "MAJOR"
	SeverityMinor    Severity = "MINOR"
)

type FindingStatus string

const (
	FindingOpen     FindingStatus = "OPEN"
	FindingResolved FindingStatus = "RESOLVED"
)

type PublicationCase struct {
	CaseID        string     `json:"case_id"`
	Title         string     `json:"title"`
	Edition       string     `json:"edition"`
	MediaFormat   string     `json:"media_format"`
	OwnerID       string     `json:"owner_id"`
	ContentDigest string     `json:"content_digest"`
	Status        CaseStatus `json:"status"`
	Revision      int64      `json:"revision"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type RequirementProfile struct {
	ProfileID          string     `json:"profile_id"`
	CaseID             string     `json:"case_id"`
	RulesetVersion     string     `json:"ruleset_version"`
	RuleCodes          []string   `json:"rule_codes"`
	BlockingSeverities []Severity `json:"blocking_severities"`
	ProfileDigest      string     `json:"profile_digest"`
	FrozenBy           string     `json:"frozen_by"`
	FrozenAt           time.Time  `json:"frozen_at"`
}

type AccessibilityFinding struct {
	FindingID      string        `json:"finding_id"`
	CaseID         string        `json:"case_id"`
	RuleCode       string        `json:"rule_code"`
	ContentLocator string        `json:"content_locator"`
	Severity       Severity      `json:"severity"`
	Impact         string        `json:"impact"`
	Status         FindingStatus `json:"status"`
	ReportedBy     string        `json:"reported_by"`
	ReportedAt     time.Time     `json:"reported_at"`
}

type RemediationEvidence struct {
	EvidenceID           string    `json:"evidence_id"`
	FindingID            string    `json:"finding_id"`
	Round                int       `json:"round"`
	SupersedesEvidenceID string    `json:"supersedes_evidence_id,omitempty"`
	ChangeSummary        string    `json:"change_summary"`
	BeforeDigest         string    `json:"before_digest"`
	AfterDigest          string    `json:"after_digest"`
	VerificationResult   string    `json:"verification_result"`
	FailureReason        string    `json:"failure_reason,omitempty"`
	SubmittedBy          string    `json:"submitted_by"`
	SubmittedAt          time.Time `json:"submitted_at"`
}

type ReviewDecision struct {
	DecisionID               string    `json:"decision_id"`
	CaseID                   string    `json:"case_id"`
	ReviewerID               string    `json:"reviewer_id"`
	Outcome                  string    `json:"outcome"`
	Reason                   string    `json:"reason"`
	SeparationCheck          bool      `json:"separation_check"`
	ReviewedRevision         int64     `json:"reviewed_revision"`
	PreviousReturnDecisionID string    `json:"previous_return_decision_id,omitempty"`
	DecidedAt                time.Time `json:"decided_at"`
}

type RemediationItemStatus string

const (
	RemediationItemPending RemediationItemStatus = "PENDING"
	RemediationItemClosed  RemediationItemStatus = "CLOSED"
)

type ReviewRemediationItem struct {
	ItemID             string                `json:"item_id"`
	CaseID             string                `json:"case_id"`
	DecisionID         string                `json:"decision_id"`
	FindingID          string                `json:"finding_id"`
	Requirement        string                `json:"requirement"`
	Priority           string                `json:"priority"`
	Status             RemediationItemStatus `json:"status"`
	ClosedByEvidenceID string                `json:"closed_by_evidence_id,omitempty"`
	CreatedAt          time.Time             `json:"created_at"`
	ClosedAt           *time.Time            `json:"closed_at,omitempty"`
}

type ReleaseCredential struct {
	Token          string    `json:"release_token"`
	CaseID         string    `json:"case_id"`
	BaselineDigest string    `json:"baseline_digest"`
	ContentDigest  string    `json:"content_digest"`
	AuthorizedBy   string    `json:"authorized_by"`
	IssuedAt       time.Time `json:"issued_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type ArchiveManifest struct {
	ManifestID         string    `json:"manifest_id"`
	CaseID             string    `json:"case_id"`
	ReleaseToken       string    `json:"release_token"`
	BaselineDigest     string    `json:"baseline_digest"`
	ContentDigest      string    `json:"content_digest"`
	EventChainHead     string    `json:"event_chain_head"`
	ManifestDigest     string    `json:"manifest_digest"`
	VerificationStatus string    `json:"verification_status"`
	ArchivedAt         time.Time `json:"archived_at"`
}

type CaseAggregate struct {
	Case             PublicationCase         `json:"case"`
	Profile          *RequirementProfile     `json:"profile,omitempty"`
	Findings         []AccessibilityFinding  `json:"findings"`
	Evidences        []RemediationEvidence   `json:"evidences"`
	Decisions        []ReviewDecision        `json:"decisions"`
	RemediationItems []ReviewRemediationItem `json:"remediation_items"`
	Release          *ReleaseCredential      `json:"release,omitempty"`
	Manifest         *ArchiveManifest        `json:"manifest,omitempty"`
}
