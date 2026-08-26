package workflow

import (
	"context"
	"strings"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/audit"
)

func (s *Service) VerifyRelease(ctx context.Context, caseID string, cmd VerifyReleaseCommand) (accessibility.ReleaseVerification, error) {
	if strings.TrimSpace(cmd.ReleaseToken) == "" {
		return accessibility.ReleaseVerification{}, NewError("VALIDATION_ERROR", "release_token 不能为空")
	}
	a, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return accessibility.ReleaseVerification{}, WrapRule(err)
	}
	return accessibility.VerifyRelease(a, cmd.ReleaseToken, s.now().UTC()), nil
}

func (s *Service) ExportArchive(ctx context.Context, caseID string) (accessibility.ArchiveExport, error) {
	a, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return accessibility.ArchiveExport{}, WrapRule(err)
	}
	if a.Case.Status != accessibility.StatusArchived || a.Profile == nil || a.Release == nil || a.Manifest == nil {
		return accessibility.ArchiveExport{}, NewError("INVALID_STATE", "仅 ARCHIVED 个案可导出证据清单")
	}
	events, err := s.repo.Events(ctx, caseID)
	if err != nil {
		return accessibility.ArchiveExport{}, WrapRule(err)
	}
	components := map[string]accessibility.ComponentVerification{}
	verified := true
	set := func(name string, ok bool, code, diagnostic string) {
		v := accessibility.ComponentVerification{Status: "VERIFIED"}
		if !ok {
			v.Status = "FAILED"
			v.ErrorCode = code
			v.Diagnostic = diagnostic
			verified = false
		}
		components[name] = v
	}
	baseline := accessibility.CalculateProfileDigest(a.Profile.RulesetVersion, a.Profile.RuleCodes, a.Profile.BlockingSeverities)
	set("baseline", baseline == a.Profile.ProfileDigest, "BASELINE_DIGEST_MISMATCH", "冻结基线摘要无法重复计算")
	contentOK := accessibility.ValidDigest(a.Case.ContentDigest) && a.Release.ContentDigest == a.Case.ContentDigest && a.Manifest.ContentDigest == a.Case.ContentDigest
	set("content_binding", contentOK, "CONTENT_BINDING_MISMATCH", "出版内容摘要绑定不一致")
	manifestID, graphErr := accessibility.DeterministicManifestID(a)
	storedGraphErr := s.repo.VerifyGraph(ctx, a)
	graphOK := graphErr == nil && storedGraphErr == nil && manifestID == a.Manifest.ManifestID
	graphDiagnostic := "证据图与 manifest_id 不一致"
	if storedGraphErr != nil {
		graphDiagnostic = storedGraphErr.Error()
	}
	set("evidence_graph", graphOK, "EVIDENCE_GRAPH_DIGEST_MISMATCH", graphDiagnostic)
	storedManifestDigest, storedManifestErr := s.repo.StoredManifestDigest(ctx, caseID)
	manifestOK := storedManifestErr == nil && storedManifestDigest == a.Manifest.ManifestDigest && a.Manifest.ManifestDigest == accessibility.CalculateManifestDigest(*a.Manifest)
	set("manifest", manifestOK, "MANIFEST_DIGEST_MISMATCH", "已存 manifest_digest 与重算结果不一致")
	chain := audit.Verify(events)
	auditComponent := accessibility.ComponentVerification{Status: "VERIFIED"}
	if !chain.Valid {
		verified = false
		auditComponent.Status = "FAILED"
		auditComponent.ErrorCode = chain.ErrorCode
		auditComponent.FailureSequence = chain.FailureSequence
		auditComponent.Diagnostic = chain.Diagnostic
	} else if len(events) == 0 || events[len(events)-1].EventType != "CASE_ARCHIVED" || events[len(events)-1].PreviousDigest != a.Manifest.EventChainHead {
		verified = false
		auditComponent.Status = "FAILED"
		auditComponent.ErrorCode = "ARCHIVE_CHAIN_HEAD_MISMATCH"
		auditComponent.FailureSequence = int64(len(events))
		auditComponent.Diagnostic = "归档清单链头与归档事件前序摘要不一致"
	}
	components["audit_chain"] = auditComponent
	evidence, err := accessibility.BuildArchiveEvidence(a)
	if err != nil {
		return accessibility.ArchiveExport{}, WrapRule(err)
	}
	body := accessibility.ArchiveExportBody{Case: evidence.Case, Profile: evidence.Profile, Findings: evidence.Findings, Evidence: evidence.Evidences, ReviewDecisions: evidence.Decisions, RemediationItems: evidence.RemediationItems, ReleaseCredential: evidence.Release, ArchiveManifest: *a.Manifest, AuditEvents: events}
	return accessibility.ArchiveExport{Verified: verified, ManifestDigest: a.Manifest.ManifestDigest, Components: components, Evidence: body}, nil
}
