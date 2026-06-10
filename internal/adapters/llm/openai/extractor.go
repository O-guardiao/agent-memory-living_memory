package openai

// Placeholder for structured memory extraction prompts using an LLM.

import (
	"context"
	"fmt"

	"github.com/agent-memory/agent-memory/internal/ports"
	"github.com/agent-memory/agent-memory/internal/services/distillation"
)

// Extractor asks any ports.LLM for typed memory candidates using the
// distillation schema. It works with the Anthropic and local adapters too.
type Extractor struct {
	llm ports.LLM
}

func NewExtractor(llm ports.LLM) *Extractor {
	return &Extractor{llm: llm}
}

const extractionPrompt = `Extract memory candidates from this %s event.

Content:
%s

Return a JSON array of objects with fields "type" (episode|fact|preference|procedure),
"content" (one self-contained sentence), "scope" (user|project),
"confidence" [0,1] and "importance" [0,1].
Only extract durable information worth recalling in future sessions.`

func (e *Extractor) ExtractMemories(ctx context.Context, eventText, role string) ([]distillation.ExtractedMemorySchema, error) {
	var out []distillation.ExtractedMemorySchema
	err := e.llm.GenerateJSON(ctx, ports.GenerateRequest{
		System: distillation.ExtractionSystemPrompt,
		Prompt: fmt.Sprintf(extractionPrompt, role, eventText),
	}, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
