package forgetting

// Add PII redaction and partial deletion here.

import (
	"context"
	"regexp"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/policy"
	"github.com/agent-memory/agent-memory/internal/ports"
	"github.com/agent-memory/agent-memory/internal/services/embedding"
)

type piiPattern struct {
	kind string
	re   *regexp.Regexp
}

var piiPatterns = []piiPattern{
	{"email", regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)},
	{"cpf", regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`)},
	{"card", regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`)},
	{"phone", regexp.MustCompile(`(?:\+?\d{1,3}[ \-]?)?(?:\(\d{2,3}\)[ \-]?)?\d{4,5}[ \-]?\d{4}\b`)},
}

// RedactPII masks emails, CPF, card-like and phone-like sequences,
// returning the redacted text and whether anything changed.
func RedactPII(text string) (string, bool) {
	changed := false
	for _, pattern := range piiPatterns {
		if pattern.re.MatchString(text) {
			text = pattern.re.ReplaceAllString(text, "[REDACTED:"+pattern.kind+"]")
			changed = true
		}
	}
	return text, changed
}

// Redactor performs partial deletion: PII is masked in place, the memory
// is re-embedded, and a deletion-style receipt is returned.
type Redactor struct {
	memories ports.MemoryStore
	embed    *embedding.Service
	clock    ports.Clock
}

func NewRedactor(memories ports.MemoryStore, embed *embedding.Service, clock ports.Clock) *Redactor {
	return &Redactor{memories: memories, embed: embed, clock: clock}
}

func (r *Redactor) Redact(ctx context.Context, tenantID, memoryID, reason string) (policy.DeletionReceipt, error) {
	mem, err := r.memories.Get(ctx, tenantID, memoryID)
	if err != nil {
		return policy.DeletionReceipt{}, err
	}
	now := r.clock.Now()

	content, contentChanged := RedactPII(mem.Content)
	summary, summaryChanged := RedactPII(mem.Summary)
	if contentChanged || summaryChanged {
		mem.Content = content
		mem.Summary = summary
		mem.SafetyLabels = withoutLabel(mem.SafetyLabels, memory.SafetyPII)
		mem.Version++
		mem.UpdatedAt = now
		if err := r.memories.Upsert(ctx, mem); err != nil {
			return policy.DeletionReceipt{}, err
		}
		if r.embed != nil {
			if err := r.embed.IndexMemory(ctx, mem); err != nil {
				return policy.DeletionReceipt{}, err
			}
		}
	}
	if reason == "" {
		reason = "pii_redaction"
	}
	return policy.DeletionReceipt{
		ID:        "redact_" + memoryID,
		TenantID:  tenantID,
		MemoryID:  memoryID,
		DeletedAt: now,
		Reason:    reason,
	}, nil
}

func withoutLabel(labels []memory.SafetyLabel, drop memory.SafetyLabel) []memory.SafetyLabel {
	out := labels[:0]
	for _, label := range labels {
		if label != drop {
			out = append(out, label)
		}
	}
	return out
}
