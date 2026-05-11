package ingestion

import (
	"testing"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

func TestValidateEventRequiresTenant(t *testing.T) {
	err := ValidateEvent(memory.Event{UserID: "u", Role: memory.RoleUser, Content: "hello"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
