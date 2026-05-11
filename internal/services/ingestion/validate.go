package ingestion

import "github.com/agent-memory/agent-memory/internal/domain/memory"

func ValidateEvent(event memory.Event) error {
	return event.Validate()
}
