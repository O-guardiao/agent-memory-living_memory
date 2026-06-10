package main

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-memory/agent-memory/internal/bootstrap"
	"github.com/agent-memory/agent-memory/internal/config"
	memorysdk "github.com/agent-memory/agent-memory/sdk/go"
)

func startServer(t *testing.T) (*httptest.Server, *memorysdk.Client) {
	t.Helper()
	app := bootstrap.NewApp(config.Config{Env: "test", Port: "0", DefaultTenant: "tenant_test", EvalRecorderEnabled: true})
	server := httptest.NewServer(app.HTTPServer)
	t.Cleanup(server.Close)
	client := memorysdk.New(server.URL)
	return server, client
}

func seed(t *testing.T, client *memorysdk.Client) {
	t.Helper()
	err := client.IngestEvent(context.Background(), map[string]any{
		"tenant_id": "tenant_test",
		"user_id":   "user_1",
		"role":      "user",
		"content":   "Prefiro Go para o core de produção.",
	}, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestSearchAndTraceCommands(t *testing.T) {
	server, client := startServer(t)
	seed(t, client)

	if err := runSearch(context.Background(), []string{
		"--base-url", server.URL, "--tenant", "tenant_test",
		"--query", "core de produção", "--user", "user_1",
	}); err != nil {
		t.Fatalf("search: %v", err)
	}

	var result searchResult
	if err := client.Search(context.Background(), map[string]any{
		"tenant_id": "tenant_test", "user_id": "user_1", "query": "core", "limit": 3,
	}, &result); err != nil {
		t.Fatal(err)
	}
	if err := runTrace(context.Background(), []string{
		"--base-url", server.URL, "--tenant", "tenant_test", "--id", result.TraceID,
	}); err != nil {
		t.Fatalf("trace: %v", err)
	}
}

func TestExportAndEvalCommands(t *testing.T) {
	server, client := startServer(t)
	seed(t, client)
	dir := t.TempDir()

	exportPath := filepath.Join(dir, "memories.jsonl")
	if err := runExport(context.Background(), []string{
		"--base-url", server.URL, "--tenant", "tenant_test", "--user", "user_1", "--out", exportPath,
	}); err != nil {
		t.Fatalf("export: %v", err)
	}
	data, err := os.ReadFile(exportPath)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		t.Fatalf("expected exported memories, got %q (%v)", data, err)
	}

	queriesPath := filepath.Join(dir, "queries.jsonl")
	if err := os.WriteFile(queriesPath, []byte(`{"query":"core de produção","user_id":"user_1"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "report.jsonl")
	if err := runEval(context.Background(), []string{
		"--base-url", server.URL, "--tenant", "tenant_test",
		"--queries", queriesPath, "--out", reportPath,
	}); err != nil {
		t.Fatalf("eval: %v", err)
	}
	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), `"summary":true`) {
		t.Fatalf("expected summary line in report: %s", report)
	}

	// Without --queries the command streams the server-side recorder.
	streamPath := filepath.Join(dir, "stream.jsonl")
	if err := runEval(context.Background(), []string{
		"--base-url", server.URL, "--out", streamPath,
	}); err != nil {
		t.Fatalf("eval stream: %v", err)
	}
}
