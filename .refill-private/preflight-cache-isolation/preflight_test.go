package preflight_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func digest(ch string) string { return strings.Repeat(ch, 64) }

func TestPreflightCacheIsolatedByInput(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	svc := workflow.NewService(repo)
	ctx := context.Background()
	a, replay, err := svc.CreateCase(ctx, workflow.CreateCaseCommand{
		RequestID: "preflight-create-01", Title: "预检缓存测试", Edition: "1", MediaFormat: "EPUB", OwnerID: "owner", ContentDigest: digest("a"),
	})
	if err != nil || replay {
		t.Fatalf("create failed replay=%v err=%v", replay, err)
	}
	first, err := svc.PreflightProfile(ctx, a.Case.CaseID, workflow.PreflightProfileCommand{
		ExpectedRevision: 1, RulesetVersion: "WCAG-2.2", RuleCodes: []string{"1.1.1"}, BlockingSeverities: []string{"CRITICAL"},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.PreflightProfile(ctx, a.Case.CaseID, workflow.PreflightProfileCommand{
		ExpectedRevision: 1, RulesetVersion: "WCAG-2.2", RuleCodes: []string{"2.1.1"}, BlockingSeverities: []string{"MAJOR"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.RuleCodes) != 1 || first.RuleCodes[0] != "1.1.1" {
		t.Fatalf("unexpected first preflight: %+v", first)
	}
	if len(second.RuleCodes) != 1 || second.RuleCodes[0] != "2.1.1" || second.BlockingSeverities[0] != "MAJOR" {
		t.Fatalf("preflight reused another input: %+v", second)
	}
	if first.ProfileDigest == second.ProfileDigest {
		t.Fatalf("different profiles must have different digests: %s", first.ProfileDigest)
	}
}
