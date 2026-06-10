package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agent-memory/agent-memory/internal/adapters/storage/postgres"
	"github.com/agent-memory/agent-memory/internal/config"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := config.Load()
	if cfg.PostgresDSN == "" {
		log.Fatal("MEMORY_POSTGRES_DSN is required")
	}
	db, err := postgres.Open(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer db.Close()
	if err := postgres.WaitFor(ctx, db); err != nil {
		log.Fatalf("wait for postgres: %v", err)
	}
	if err := postgres.RunMigrations(ctx, db, env("MEMORY_POSTGRES_MIGRATIONS_DIR", "migrations/postgres")); err != nil {
		log.Fatalf("postgres migrations failed: %v", err)
	}
	if cfg.Neo4jEndpoint != "" {
		if err := runNeo4jMigrations(ctx, cfg, env("MEMORY_NEO4J_MIGRATIONS_DIR", "migrations/neo4j")); err != nil {
			log.Fatalf("neo4j migrations failed: %v", err)
		}
	}
	log.Println("migrations applied")
}

func runNeo4jMigrations(ctx context.Context, cfg config.Config, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.cypher"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		statements := splitCypherStatements(string(data))
		if len(statements) == 0 {
			continue
		}
		if err := execNeo4j(ctx, cfg, statements); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		log.Printf("applied neo4j migration %s", filepath.Base(file))
	}
	return nil
}

func splitCypherStatements(raw string) []string {
	parts := strings.Split(raw, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}
	return statements
}

func execNeo4j(ctx context.Context, cfg config.Config, statements []string) error {
	payload := make([]map[string]any, 0, len(statements))
	for _, stmt := range statements {
		payload = append(payload, map[string]any{"statement": stmt})
	}
	body, err := json.Marshal(map[string]any{"statements": payload})
	if err != nil {
		return err
	}
	url := strings.TrimRight(cfg.Neo4jEndpoint, "/") + "/db/" + strings.Trim(cfg.Neo4jDatabase, "/") + "/tx/commit"
	var lastErr error
	for attempt := 0; attempt < 60; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if cfg.Neo4jBasicAuth != "" {
			req.Header.Set("Authorization", "Basic "+cfg.Neo4jBasicAuth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			lastErr = decodeNeo4jResponse(resp)
			if lastErr == nil {
				return nil
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return lastErr
}

func decodeNeo4jResponse(resp *http.Response) error {
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var decoded struct {
		Errors []map[string]any `json:"errors"`
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
	}
	if len(decoded.Errors) > 0 {
		return fmt.Errorf("errors: %v", decoded.Errors)
	}
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
