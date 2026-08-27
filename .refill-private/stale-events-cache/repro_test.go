package stale_events_cache_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func TestEventsCacheRefreshesAfterMutation(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := workflow.NewService(repo)
	ctx := context.Background()
	created, _, err := service.CreateCase(ctx, workflow.CreateCaseCommand{
		RequestID: "events-cache-create", Title: "缓存测试", Edition: "1", MediaFormat: "EPUB",
		OwnerID: "owner", ContentDigest: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := service.Events(ctx, created.Case.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 {
		t.Fatalf("创建后事件数=%d", len(before))
	}
	_, _, err = service.FreezeProfile(ctx, created.Case.CaseID, workflow.FreezeProfileCommand{
		CommandMeta:    workflow.CommandMeta{RequestID: "events-cache-freeze", ActorID: "owner", ExpectedRevision: created.Case.Revision},
		RulesetVersion: "WCAG-2.2", RuleCodes: []string{"1.1.1"}, BlockingSeverities: []string{"CRITICAL"},
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := service.Events(ctx, created.Case.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 {
		t.Fatalf("变更后事件缓存未刷新，事件数=%d", len(after))
	}
}
