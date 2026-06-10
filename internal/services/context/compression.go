package contextsvc

// Add semantic compression and token budget enforcement here.

import (
	"sort"
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

const (
	compressWordThreshold = 120
	compressSentences     = 2
)

// EnforceBudget trims the pack to the token budget: lowest-score memories
// are dropped first (across all sections), then long survivors are
// compressed to their leading sentences. No-op when budget <= 0.
func EnforceBudget(pack *retrieval.ContextPack, candidates []retrieval.Candidate, budget int) {
	if budget <= 0 || pack.TokenEstimate <= budget {
		return
	}

	scores := make(map[string]float64, len(candidates))
	for _, candidate := range candidates {
		scores[candidate.Memory.ID] = candidate.Score
	}
	sections := []*[]memory.Memory{
		&pack.Profile, &pack.Preferences, &pack.ProjectMemory,
		&pack.RecentEpisodes, &pack.RelevantFacts, &pack.Procedures,
	}

	// Drop lowest-score memories until the estimate fits (or one remains).
	type ref struct {
		section *[]memory.Memory
		id      string
		score   float64
	}
	var refs []ref
	total := 0
	for _, section := range sections {
		for _, mem := range *section {
			refs = append(refs, ref{section: section, id: mem.ID, score: scores[mem.ID]})
			total++
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].score < refs[j].score })

	estimate := pack.TokenEstimate
	for _, victim := range refs {
		if estimate <= budget || total <= 1 {
			break
		}
		for _, mem := range *victim.section {
			if mem.ID == victim.id {
				estimate -= EstimateTokens(mem.Content)
				break
			}
		}
		*victim.section = removeByID(*victim.section, victim.id)
		total--
	}

	// Compress long survivors to their leading sentences.
	for _, section := range sections {
		for i := range *section {
			content := (*section)[i].Content
			if len(strings.Fields(content)) <= compressWordThreshold {
				continue
			}
			compressed := leadingSentences(content, compressSentences) + " …"
			estimate -= EstimateTokens(content) - EstimateTokens(compressed)
			(*section)[i].Content = compressed
		}
	}

	pack.TokenEstimate = estimate
	pack.Warnings = append(pack.Warnings, "context_compressed_to_budget")
}

// EstimateTokens mirrors the retrieval-side estimate (words * 1.35).
func EstimateTokens(text string) int {
	n := len(strings.Fields(text))
	if n == 0 {
		return 0
	}
	return int(float64(n) * 1.35)
}

func removeByID(memories []memory.Memory, id string) []memory.Memory {
	out := memories[:0]
	for _, mem := range memories {
		if mem.ID != id {
			out = append(out, mem)
		}
	}
	return out
}

func leadingSentences(text string, n int) string {
	count := 0
	for i, r := range text {
		if r == '.' || r == '!' || r == '?' {
			count++
			if count == n {
				return strings.TrimSpace(text[:i+1])
			}
		}
	}
	return text
}
