package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/store"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/web"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/workflow"
)

type selfEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func runSelfCheck(cfg config, logger *slog.Logger) error {
	tempDir, err := os.MkdirTemp("", "accessibility-release-selfcheck-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	repo, err := store.Open(filepath.Join(tempDir, "selfcheck.db"))
	if err != nil {
		return err
	}
	defer repo.Close()
	handler := web.New(workflow.NewService(repo), logger).Handler()
	server := newHTTPServer(cfg.Address, handler)
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("监听自检地址: %w", err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := "http://" + cfg.Address
	if err := executeSelfCheck(client, baseURL); err != nil {
		server.Close()
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-time.After(time.Second):
		return fmt.Errorf("自检服务未按时结束")
	}
	logger.Info("self_check_passed", "address", cfg.Address)
	return nil
}

func executeSelfCheck(client *http.Client, base string) error {
	digest := func(ch byte) string { return string(bytes.Repeat([]byte{ch}, 64)) }
	created, err := postMap(client, base+"/api/v1/cases", map[string]any{"request_id": "self-create-0001", "title": "自检无障碍出版物", "edition": "第 1 版", "media_format": "EPUB 3", "owner_id": "owner-selfcheck", "content_digest": digest('a')})
	if err != nil {
		return err
	}
	caseID := nestedString(created, "case", "case_id")
	if caseID == "" {
		return fmt.Errorf("自检建档未返回 case_id")
	}
	preflight, err := postMap(client, base+"/api/v1/cases/"+caseID+"/profile/preflight", map[string]any{"expected_revision": 1, "ruleset_version": "WCAG-2.2-AA", "rule_codes": []string{" 1.1.1 ", "1.1.1"}, "blocking_severities": []string{"CRITICAL", "MAJOR"}})
	if err != nil || preflight["can_freeze"] != true || len(fmt.Sprint(preflight["profile_digest"])) != 64 {
		return fmt.Errorf("自检基线预检失败: %v", err)
	}
	commands := []struct {
		path string
		body map[string]any
	}{
		{"/profile", map[string]any{"request_id": "self-profile-001", "actor_id": "owner-selfcheck", "expected_revision": 1, "ruleset_version": "WCAG-2.2-AA", "rule_codes": []string{"1.1.1"}, "blocking_severities": []string{"CRITICAL", "MAJOR"}}},
		{"/findings/batch", map[string]any{"request_id": "self-finding-001", "actor_id": "auditor-selfcheck", "expected_revision": 2, "findings": []map[string]any{{"rule_code": "1.1.1", "content_locator": "EPUB/cover.xhtml#image", "severity": "CRITICAL", "impact": "封面图缺少替代文本"}}}},
		{"/audit/complete", map[string]any{"request_id": "self-audit-00001", "actor_id": "auditor-selfcheck", "expected_revision": 3}},
	}
	var latest map[string]any
	for _, c := range commands {
		latest, err = postMap(client, base+"/api/v1/cases/"+caseID+c.path, c.body)
		if err != nil {
			return err
		}
		if aggregate, ok := latest["aggregate"].(map[string]any); ok {
			latest = aggregate
		}
	}
	findings, _ := latest["findings"].([]any)
	if len(findings) != 1 {
		return fmt.Errorf("自检发现项数量不正确")
	}
	findingID, _ := findings[0].(map[string]any)["finding_id"].(string)
	rest := []struct {
		path string
		body map[string]any
	}{
		{"/remediations", map[string]any{"request_id": "self-remedy-0001", "actor_id": "editor-selfcheck", "expected_revision": 4, "finding_id": findingID, "change_summary": "补充准确描述封面主题的替代文本", "before_digest": digest('b'), "after_digest": digest('c'), "verification_result": "PASS"}},
		{"/review/submit", map[string]any{"request_id": "self-review-submit", "actor_id": "owner-selfcheck", "expected_revision": 5}},
		{"/review/decision", map[string]any{"request_id": "self-review-decide", "actor_id": "reviewer-selfcheck", "expected_revision": 6, "outcome": "APPROVE", "reason": "证据完整且职责分离"}},
		{"/release", map[string]any{"request_id": "self-release-001", "actor_id": "release-selfcheck", "expected_revision": 7, "valid_hours": 24}},
		{"/archive", map[string]any{"request_id": "self-archive-001", "actor_id": "archive-selfcheck", "expected_revision": 8}},
	}
	for _, c := range rest {
		if c.path == "/archive" {
			verification, verifyErr := postMap(client, base+"/api/v1/cases/"+caseID+"/release/verify", map[string]any{"release_token": nestedString(latest, "release", "release_token")})
			if verifyErr != nil || verification["status"] != "VALID" {
				return fmt.Errorf("自检发布凭证核验失败: %v", verifyErr)
			}
		}
		latest, err = postMap(client, base+"/api/v1/cases/"+caseID+c.path, c.body)
		if err != nil {
			return err
		}
	}
	if latest["case"].(map[string]any)["status"] != "ARCHIVED" {
		return fmt.Errorf("自检最终状态不是 ARCHIVED")
	}
	exported, err := getRawMap(client, base+"/api/v1/cases/"+caseID+"/archive/export")
	if err != nil || exported["verified"] != true {
		return fmt.Errorf("自检归档导出复验失败: %v", err)
	}
	manifest, ok := latest["manifest"].(map[string]any)
	if !ok || manifest["verification_status"] != "VERIFIED" || len(fmt.Sprint(manifest["manifest_digest"])) != 64 {
		return fmt.Errorf("自检归档清单无效")
	}
	view, err := getMap(client, base+"/api/v1/cases/"+caseID)
	if err != nil {
		return err
	}
	auditInfo, _ := view["audit"].(map[string]any)
	if auditInfo["valid"] != true {
		return fmt.Errorf("自检审计链校验失败")
	}
	return nil
}

func postMap(client *http.Client, url string, body any) (map[string]any, error) {
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return doMap(client, req)
}
func getMap(client *http.Client, url string) (map[string]any, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	return doMap(client, req)
}
func getRawMap(client *http.Client, url string) (map[string]any, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return result, nil
}
func doMap(client *http.Client, req *http.Request) (map[string]any, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var env selfEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		if env.Error != nil {
			return nil, fmt.Errorf("HTTP %d %s: %s", resp.StatusCode, env.Error.Code, env.Error.Message)
		}
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.Unmarshal(env.Data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func nestedString(value map[string]any, key, field string) string {
	child, _ := value[key].(map[string]any)
	result, _ := child[field].(string)
	return result
}
