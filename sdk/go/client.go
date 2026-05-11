package memorysdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
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

func (c *Client) get(ctx context.Context, path string, out any) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
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
