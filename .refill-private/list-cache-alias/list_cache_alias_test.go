package listcachealias

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

func TestListCasesDoesNotExposeRepositoryCache(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "cases.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := workflow.NewService(repo)
	const originalTitle = "无障碍出版物"
	_, _, err = service.CreateCase(context.Background(), workflow.CreateCaseCommand{
		RequestID: "cache-alias-create-01",
		Title:     originalTitle, Edition: "1.0", MediaFormat: "EPUB 3", OwnerID: "owner",
		ContentDigest: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.ListCases(context.Background())
	if err != nil || len(first) != 1 {
		t.Fatalf("首次列表读取失败: len=%d err=%v", len(first), err)
	}
	first[0].Title = "调用方篡改的标题"
	second, err := service.ListCases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].Title != originalTitle {
		t.Fatalf("列表缓存被调用方切片别名污染: %+v", second)
	}
}
