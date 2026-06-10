package memorysdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	// APIKey, when set, is sent as a Bearer token on every request.
	APIKey string
}

func New(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTP: http.DefaultClient}
}

func (c *Client) IngestEvent(ctx context.Context, req any, out any) error {
	return c.post(ctx, "/v1/events", req, out)
}

func (c *Client) Search(ctx context.Context, req any, out any) error {
	return c.post(ctx, "/v1/memories/search", req, out)
}

func (c *Client) AssembleContext(ctx context.Context, req any, out any) error {
	return c.post(ctx, "/v1/context/assemble", req, out)
}

func (c *Client) AgenticSkills(ctx context.Context, out any) error {
	return c.get(ctx, "/v1/agentic/skills", out)
}

func (c *Client) AgenticDecide(ctx context.Context, req any, out any) error {
	return c.post(ctx, "/v1/agentic/decide", req, out)
}

func (c *Client) CreateSpec(ctx context.Context, req any, out any) error {
	return c.post(ctx, "/v1/agentic/specs", req, out)
}

func (c *Client) CreatePlan(ctx context.Context, req any, out any) error {
	return c.post(ctx, "/v1/agentic/plans", req, out)
}

func (c *Client) AssembleAgenticContext(ctx context.Context, req any, out any) error {
	return c.post(ctx, "/v1/agentic/context", req, out)
}

// Trace fetches a retrieval trace by ID.
func (c *Client) Trace(ctx context.Context, tenantID, traceID string, out any) error {
	query := url.Values{}
	if tenantID != "" {
		query.Set("tenant_id", tenantID)
	}
	return c.get(ctx, "/v1/traces/"+url.PathEscape(traceID)+"?"+query.Encode(), out)
}

// ListMemories lists memories for a tenant/user.
func (c *Client) ListMemories(ctx context.Context, tenantID, userID string, limit int, out any) error {
	query := url.Values{}
	if tenantID != "" {
		query.Set("tenant_id", tenantID)
	}
	if userID != "" {
		query.Set("user_id", userID)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	return c.get(ctx, "/v1/memories?"+query.Encode(), out)
}

// ExportEvals streams the recorded retrievals as JSONL. The caller must
// close the returned reader.
func (c *Client) ExportEvals(ctx context.Context) (io.ReadCloser, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/evals/export", nil)
	if err != nil {
		return nil, err
	}
	c.authorize(httpReq)
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("memory api returned status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (c *Client) authorize(req *http.Request) {
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	c.authorize(httpReq)
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("memory api returned status %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) post(ctx context.Context, path string, req any, out any) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.authorize(httpReq)
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("memory api returned status %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
