// memory-cli drives the HTTP API: search, trace, export, eval.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	memorysdk "github.com/agent-memory/agent-memory/sdk/go"
)

func main() {
	if len(os.Args) == 1 {
		usage()
		return
	}
	ctx := context.Background()
	var err error
	switch os.Args[1] {
	case "search":
		err = runSearch(ctx, os.Args[2:])
	case "trace":
		err = runTrace(ctx, os.Args[2:])
	case "export":
		err = runExport(ctx, os.Args[2:])
	case "eval":
		err = runEval(ctx, os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Printf("memory-cli: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory-cli %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`memory-cli: command line client for the agent-memory HTTP API.

Commands:
  search  --query <text> [--user u] [--tenant t] [--limit n] [--json]
  trace   --id <trace_id> [--tenant t]
  export  [--user u] [--tenant t] [--limit n] [--out file.jsonl]
  eval    [--queries file.jsonl] [--out report.jsonl]

Global flags (per command): --base-url (env MEMORY_BASE_URL,
default http://localhost:8080), --api-key (env MEMORY_API_KEY),
--tenant (env MEMORY_TENANT).
`)
}

func newFlagSet(name string) (*flag.FlagSet, *string, *string, *string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	baseURL := fs.String("base-url", envOr("MEMORY_BASE_URL", "http://localhost:8080"), "API base URL")
	apiKey := fs.String("api-key", os.Getenv("MEMORY_API_KEY"), "API key (Bearer)")
	tenant := fs.String("tenant", os.Getenv("MEMORY_TENANT"), "tenant ID")
	return fs, baseURL, apiKey, tenant
}

func clientFor(baseURL, apiKey string) *memorysdk.Client {
	client := memorysdk.New(baseURL)
	client.APIKey = apiKey
	return client
}

type searchResult struct {
	Memories []struct {
		Memory struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"memory"`
		Score float64 `json:"score"`
	} `json:"memories"`
	TraceID string `json:"trace_id"`
}

func runSearch(ctx context.Context, args []string) error {
	fs, baseURL, apiKey, tenant := newFlagSet("search")
	query := fs.String("query", "", "search text (required)")
	user := fs.String("user", "", "user ID")
	limit := fs.Int("limit", 8, "max results")
	asJSON := fs.Bool("json", false, "print raw JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *query == "" {
		return fmt.Errorf("--query is required")
	}
	client := clientFor(*baseURL, *apiKey)

	req := map[string]any{"query": *query, "limit": *limit}
	if *tenant != "" {
		req["tenant_id"] = *tenant
	}
	if *user != "" {
		req["user_id"] = *user
	}
	if *asJSON {
		var raw json.RawMessage
		if err := client.Search(ctx, req, &raw); err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	var result searchResult
	if err := client.Search(ctx, req, &result); err != nil {
		return err
	}
	fmt.Printf("trace: %s\n", result.TraceID)
	for _, item := range result.Memories {
		content := item.Memory.Content
		if len(content) > 80 {
			content = content[:80] + "…"
		}
		fmt.Printf("%-24s %-10s %.3f  %s\n", item.Memory.ID, item.Memory.Type, item.Score, content)
	}
	return nil
}

func runTrace(ctx context.Context, args []string) error {
	fs, baseURL, apiKey, tenant := newFlagSet("trace")
	id := fs.String("id", "", "trace ID (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("--id is required")
	}
	client := clientFor(*baseURL, *apiKey)
	var trace map[string]any
	if err := client.Trace(ctx, *tenant, *id, &trace); err != nil {
		return err
	}
	pretty, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(pretty))
	return nil
}

func runExport(ctx context.Context, args []string) error {
	fs, baseURL, apiKey, tenant := newFlagSet("export")
	user := fs.String("user", "", "user ID")
	limit := fs.Int("limit", 100, "max memories")
	out := fs.String("out", "", "output file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client := clientFor(*baseURL, *apiKey)

	var result struct {
		Memories []json.RawMessage `json:"memories"`
	}
	if err := client.ListMemories(ctx, *tenant, *user, *limit, &result); err != nil {
		return err
	}
	writer, closeFn, err := outputFor(*out)
	if err != nil {
		return err
	}
	defer closeFn()
	for _, mem := range result.Memories {
		if _, err := fmt.Fprintln(writer, string(mem)); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "exported %d memories\n", len(result.Memories))
	return nil
}

type evalQuery struct {
	Query     string   `json:"query"`
	UserID    string   `json:"user_id,omitempty"`
	ExpectIDs []string `json:"expect_ids,omitempty"`
}

func runEval(ctx context.Context, args []string) error {
	fs, baseURL, apiKey, tenant := newFlagSet("eval")
	queriesPath := fs.String("queries", "", "JSONL file with {query, expect_ids}; empty streams the recorder export")
	out := fs.String("out", "", "output file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client := clientFor(*baseURL, *apiKey)
	writer, closeFn, err := outputFor(*out)
	if err != nil {
		return err
	}
	defer closeFn()

	if *queriesPath == "" {
		// No query file: stream the server-side evaluation recorder.
		body, err := client.ExportEvals(ctx)
		if err != nil {
			return err
		}
		defer body.Close()
		_, err = io.Copy(writer, body)
		return err
	}

	file, err := os.Open(*queriesPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(writer)
	var latencies []float64
	hits, total := 0, 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var q evalQuery
		if err := json.Unmarshal(line, &q); err != nil {
			return fmt.Errorf("parse query line: %w", err)
		}
		req := map[string]any{"query": q.Query, "limit": 8}
		if *tenant != "" {
			req["tenant_id"] = *tenant
		}
		if q.UserID != "" {
			req["user_id"] = q.UserID
		}
		start := time.Now()
		var result searchResult
		if err := client.Search(ctx, req, &result); err != nil {
			return err
		}
		latency := float64(time.Since(start).Milliseconds())
		latencies = append(latencies, latency)

		returned := make([]string, 0, len(result.Memories))
		for _, item := range result.Memories {
			returned = append(returned, item.Memory.ID)
		}
		hit := len(q.ExpectIDs) == 0 || intersects(q.ExpectIDs, returned)
		if hit {
			hits++
		}
		total++
		if err := encoder.Encode(map[string]any{
			"query":        q.Query,
			"latency_ms":   latency,
			"hit":          hit,
			"returned_ids": returned,
		}); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if total > 0 {
		summary := map[string]any{
			"summary":        true,
			"queries":        total,
			"hit_rate":       float64(hits) / float64(total),
			"latency_p50_ms": percentile(latencies, 0.50),
			"latency_p95_ms": percentile(latencies, 0.95),
		}
		if err := encoder.Encode(summary); err != nil {
			return err
		}
	}
	return nil
}

func intersects(expected, returned []string) bool {
	set := make(map[string]struct{}, len(returned))
	for _, id := range returned {
		set[id] = struct{}{}
	}
	for _, id := range expected {
		if _, ok := set[id]; ok {
			return true
		}
	}
	return false
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	idx := int(p * float64(len(sorted)-1))
	return sorted[idx]
}

func outputFor(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return file, func() { file.Close() }, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
