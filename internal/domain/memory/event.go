package memory

import (
	"errors"
	"strings"
	"time"
)

type EventRole string

const (
	RoleUser      EventRole = "user"
	RoleAssistant EventRole = "assistant"
	RoleSystem    EventRole = "system"
	RoleTool      EventRole = "tool"
)

type Event struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	UserID     string         `json:"user_id"`
	AgentID    string         `json:"agent_id"`
	SessionID  string         `json:"session_id"`
	ProjectID  string         `json:"project_id,omitempty"`
	Role       EventRole      `json:"role"`
	Content    string         `json:"content"`
	ToolName   string         `json:"tool_name,omitempty"`
	ToolResult string         `json:"tool_result,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.TenantID) == "" {
		return errors.New("tenant_id is required")
	}
	if strings.TrimSpace(e.UserID) == "" {
		return errors.New("user_id is required")
	}
	if strings.TrimSpace(e.Content) == "" && strings.TrimSpace(e.ToolResult) == "" {
		return errors.New("content or tool_result is required")
	}
	switch e.Role {
	case RoleUser, RoleAssistant, RoleSystem, RoleTool:
		return nil
	case "":
		return errors.New("role is required")
	default:
		return errors.New("unsupported role")
	}
}
