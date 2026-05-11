package qdrant

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/agent-memory/agent-memory/internal/ports"
)

type VectorStore struct {
	endpoint   string
	collection string
	client     *http.Client
}

func NewVectorStore(endpoint, collection string, client *http.Client) *VectorStore {
	if client == nil {
		client = http.DefaultClient
	}
	return &VectorStore{
		endpoint:   strings.TrimRight(endpoint, "/"),
		collection: strings.Trim(collection, "/"),
		client:     client,
	}
}

func (s *VectorStore) EnsureCollection(ctx context.Context, vectorSize int) error {
	if vectorSize <= 0 {
		vectorSize = 64
	}
	if err := s.do(ctx, http.MethodGet, "", nil, nil); err == nil {
		return nil
	}
	body := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}
	err := s.do(ctx, http.MethodPut, "", body, nil)
	if err != nil && strings.Contains(err.Error(), "status 409") {
		return nil
	}
	return err
}

func (s *VectorStore) Upsert(ctx context.Context, items []ports.VectorItem) error {
	points := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := map[string]string{}
		for key, value := range item.Payload {
			payload[key] = value
		}
		payload["memory_id"] = item.ID
		points = append(points, map[string]any{
			"id":      qdrantPointID(item.ID),
			"vector":  item.Vector,
			"payload": payload,
		})
	}
	return s.do(ctx, http.MethodPut, "/points?wait=true", map[string]any{"points": points}, nil)
}

func (s *VectorStore) Search(ctx context.Context, query ports.VectorQuery) ([]ports.VectorResult, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 8
	}
	body := map[string]any{
		"vector":       query.Vector,
		"limit":        limit,
		"with_payload": true,
	}
	if filter := qdrantFilter(query); len(filter) > 0 {
		body["filter"] = map[string]any{"must": filter}
	}
	var response struct {
		Result []struct {
			Score   float64           `json:"score"`
			Payload map[string]string `json:"payload"`
		} `json:"result"`
	}
	if err := s.do(ctx, http.MethodPost, "/points/search", body, &response); err != nil {
		return nil, err
	}
	results := make([]ports.VectorResult, 0, len(response.Result))
	for _, item := range response.Result {
		id := item.Payload["memory_id"]
		if id == "" {
			continue
		}
		results = append(results, ports.VectorResult{ID: id, Score: item.Score})
	}
	return results, nil
}

func (s *VectorStore) Delete(ctx context.Context, ids []string) error {
	points := make([]string, 0, len(ids))
	for _, id := range ids {
		points = append(points, qdrantPointID(id))
	}
	return s.do(ctx, http.MethodPost, "/points/delete?wait=true", map[string]any{"points": points}, nil)
}

func qdrantFilter(query ports.VectorQuery) []map[string]any {
	values := map[string]string{}
	if query.TenantID != "" {
		values["tenant_id"] = query.TenantID
	}
	if query.UserID != "" {
		values["user_id"] = query.UserID
	}
	for key, value := range query.Filters {
		if value != "" {
			values[key] = value
		}
	}
	filter := make([]map[string]any, 0, len(values))
	for key, value := range values {
		filter = append(filter, map[string]any{
			"key": key,
			"match": map[string]any{
				"value": value,
			},
		})
	}
	return filter
}

func (s *VectorStore) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	url := fmt.Sprintf("%s/collections/%s%s", s.endpoint, s.collection, path)
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("qdrant %s %s: status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func qdrantPointID(memoryID string) string {
	sum := sha1.Sum([]byte(memoryID))
	raw := hex.EncodeToString(sum[:16])
	return fmt.Sprintf("%s-%s-5%s-%s-%s", raw[:8], raw[8:12], raw[13:16], raw[16:20], raw[20:32])
}
