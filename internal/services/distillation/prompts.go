package distillation

import (
	"fmt"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

const ExtractionSystemPrompt = `Extract durable, useful and scoped memories from agent events. Return JSON only.`

const extractionUserTemplate = `Extract memory candidates from this agent event.

Event role: %s
Event content:
%s

Return a JSON array (possibly empty) of objects with fields:
- "type": one of "episode", "fact", "preference", "procedure"
- "content": a single self-contained sentence stating the memory
- "scope": "user" or "project"
- "confidence": number in [0,1] for how certain the memory is
- "importance": number in [0,1] for how useful it will be later

Only extract durable information worth recalling in future sessions.
Skip greetings, fillers and one-off chatter.`

func extractionUserPrompt(event memory.Event, text string) string {
	return fmt.Sprintf(extractionUserTemplate, event.Role, text)
}
