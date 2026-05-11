package neo4j

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/agent-memory/agent-memory/internal/ports"
)

type GraphStore struct {
	endpoint string
	database string
	auth     string
	client   *http.Client
}

func NewGraphStore(endpoint, database, basicAuth string, client *http.Client) *GraphStore {
	if client == nil {
		client = http.DefaultClient
	}
	if database == "" {
		database = "neo4j"
	}
	return &GraphStore{
		endpoint: strings.TrimRight(endpoint, "/"),
		database: strings.Trim(database, "/"),
		auth:     basicAuth,
		client:   client,
	}
}

func (s *GraphStore) UpsertNode(ctx context.Context, node ports.GraphNode) error {
	labels := sanitizeLabels(node.Labels)
	if labels == "" {
		labels = ":Memory"
	}
	params := map[string]any{
		"id":      node.ID,
		"payload": node.Payload,
	}
	return s.exec(ctx, []statement{{
		Statement:  "MERGE (n" + labels + " {id: $id}) SET n += $payload",
		Parameters: params,
	}}, nil)
}

func (s *GraphStore) UpsertEdge(ctx context.Context, edge ports.GraphEdge) error {
	relType := sanitizeRelType(edge.Type)
	params := map[string]any{
		"from_id": edge.FromID,
		"to_id":   edge.ToID,
		"payload": edge.Payload,
	}
	return s.exec(ctx, []statement{{
		Statement:  "MERGE (a:Memory {id: $from_id}) MERGE (b:Memory {id: $to_id}) MERGE (a)-[r:" + relType + "]->(b) SET r += $payload",
		Parameters: params,
	}}, nil)
}

func (s *GraphStore) Traverse(ctx context.Context, query ports.GraphQuery) ([]ports.GraphResult, error) {
	depth := query.Depth
	if depth <= 0 {
		depth = 1
	}
	if depth > 4 {
		depth = 4
	}
	cypher := fmt.Sprintf(
		"MATCH path = (start:Memory {id: $start_id})-[*1..%d]->(n:Memory) WHERE coalesce(start.tenant_id, $tenant_id) = $tenant_id AND coalesce(n.tenant_id, $tenant_id) = $tenant_id RETURN DISTINCT n.id AS node_id, 1.0 / (length(path) + 1) AS score ORDER BY score DESC LIMIT 50",
		depth,
	)
	var response txResponse
	err := s.exec(ctx, []statement{{
		Statement: cypher,
		Parameters: map[string]any{
			"tenant_id": query.TenantID,
			"start_id":  query.StartID,
		},
	}}, &response)
	if err != nil {
		return nil, err
	}
	if len(response.Results) == 0 {
		return nil, nil
	}
	out := make([]ports.GraphResult, 0, len(response.Results[0].Data))
	for _, row := range response.Results[0].Data {
		if len(row.Row) < 2 {
			continue
		}
		nodeID, _ := row.Row[0].(string)
		score, _ := row.Row[1].(float64)
		if nodeID != "" {
			out = append(out, ports.GraphResult{NodeID: nodeID, Score: score})
		}
	}
	return out, nil
}

type statement struct {
	Statement  string         `json:"statement"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type txResponse struct {
	Results []struct {
		Data []struct {
			Row []any `json:"row"`
		} `json:"data"`
	} `json:"results"`
	Errors []map[string]any `json:"errors"`
}

func (s *GraphStore) exec(ctx context.Context, statements []statement, out *txResponse) error {
	body, err := json.Marshal(map[string]any{"statements": statements})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/db/%s/tx/commit", s.endpoint, s.database)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.auth != "" {
		req.Header.Set("Authorization", "Basic "+s.auth)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("neo4j tx: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var decoded txResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return err
	}
	if len(decoded.Errors) > 0 {
		return fmt.Errorf("neo4j tx errors: %v", decoded.Errors)
	}
	if out != nil {
		*out = decoded
	}
	return nil
}

var safeToken = regexp.MustCompile(`[^A-Za-z0-9_]`)

func sanitizeLabels(labels []string) string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		clean := safeToken.ReplaceAllString(label, "_")
		if clean != "" {
			out = append(out, ":"+clean)
		}
	}
	return strings.Join(out, "")
}

func sanitizeRelType(value string) string {
	clean := strings.ToUpper(safeToken.ReplaceAllString(value, "_"))
	if clean == "" {
		return "RELATED_TO"
	}
	return clean
}
