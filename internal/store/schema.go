package store

const schemaVersion = 2

const schemaSQL = `
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS publication_cases (
 case_id TEXT PRIMARY KEY, title TEXT NOT NULL, edition TEXT NOT NULL, media_format TEXT NOT NULL,
 owner_id TEXT NOT NULL, content_digest TEXT NOT NULL, status TEXT NOT NULL, revision INTEGER NOT NULL,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL, aggregate_json BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cases_status_updated ON publication_cases(status, updated_at);
CREATE TABLE IF NOT EXISTS requirement_profiles (
 profile_id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES publication_cases(case_id),
 ruleset_version TEXT NOT NULL, rule_codes_json BLOB NOT NULL, blocking_severities_json BLOB NOT NULL,
 profile_digest TEXT NOT NULL, frozen_by TEXT NOT NULL, frozen_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS accessibility_findings (
 finding_id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES publication_cases(case_id), rule_code TEXT NOT NULL,
 content_locator TEXT NOT NULL, severity TEXT NOT NULL, impact TEXT NOT NULL, status TEXT NOT NULL,
 reported_by TEXT NOT NULL, reported_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_findings_case_status ON accessibility_findings(case_id, status, severity);
CREATE TABLE IF NOT EXISTS remediation_evidence (
	 evidence_id TEXT PRIMARY KEY, finding_id TEXT NOT NULL REFERENCES accessibility_findings(finding_id),
	 round INTEGER NOT NULL, supersedes_evidence_id TEXT,
	 change_summary TEXT NOT NULL, before_digest TEXT NOT NULL, after_digest TEXT NOT NULL,
	 verification_result TEXT NOT NULL, failure_reason TEXT NOT NULL, submitted_by TEXT NOT NULL, submitted_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS review_decisions (
 decision_id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES publication_cases(case_id), reviewer_id TEXT NOT NULL,
 outcome TEXT NOT NULL, reason TEXT NOT NULL, separation_check INTEGER NOT NULL, reviewed_revision INTEGER NOT NULL, previous_return_decision_id TEXT NOT NULL, decided_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS review_remediation_items (
 item_id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES publication_cases(case_id), decision_id TEXT NOT NULL REFERENCES review_decisions(decision_id),
 finding_id TEXT NOT NULL REFERENCES accessibility_findings(finding_id), requirement TEXT NOT NULL, priority TEXT NOT NULL, status TEXT NOT NULL,
 closed_by_evidence_id TEXT, created_at TEXT NOT NULL, closed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_remediation_items_case_status ON review_remediation_items(case_id,status);
CREATE TABLE IF NOT EXISTS release_credentials (
 release_token TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES publication_cases(case_id), baseline_digest TEXT NOT NULL,
 content_digest TEXT NOT NULL, authorized_by TEXT NOT NULL, issued_at TEXT NOT NULL, expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS archive_manifests (
 manifest_id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES publication_cases(case_id), release_token TEXT NOT NULL,
 baseline_digest TEXT NOT NULL, content_digest TEXT NOT NULL, event_chain_head TEXT NOT NULL,
 manifest_digest TEXT NOT NULL, verification_status TEXT NOT NULL, archived_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_events (
 event_id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES publication_cases(case_id), sequence INTEGER NOT NULL,
 event_type TEXT NOT NULL, actor_id TEXT NOT NULL, revision INTEGER NOT NULL, payload BLOB NOT NULL,
 payload_digest TEXT NOT NULL, previous_digest TEXT NOT NULL, event_digest TEXT NOT NULL, occurred_at TEXT NOT NULL,
 UNIQUE(case_id, sequence), UNIQUE(case_id, event_digest)
);
CREATE INDEX IF NOT EXISTS idx_events_case_sequence ON audit_events(case_id, sequence);
CREATE TABLE IF NOT EXISTS idempotency_results (
 request_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, payload_hash TEXT NOT NULL, response_json BLOB NOT NULL, created_at TEXT NOT NULL
);
`
