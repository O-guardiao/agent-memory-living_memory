package localrerank

// Placeholder for local reranker adapter. Implement ports.Reranker here.

import (
	"context"
	"math"
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

// BM25 parameters: standard defaults for short memory snippets.
const (
	k1 = 1.2
	b  = 0.75
)

// Reranker scores candidates with in-process BM25 over their contents,
// a stronger lexical signal than plain overlap, with zero dependencies.
type Reranker struct{}

func New() Reranker { return Reranker{} }

func (Reranker) Rerank(ctx context.Context, query string, candidates []retrieval.Candidate) ([]retrieval.Candidate, error) {
	_ = ctx
	if len(candidates) == 0 {
		return candidates, nil
	}
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		retrieval.SortCandidates(candidates)
		return candidates, nil
	}

	docs := make([]map[string]int, len(candidates))
	totalLen := 0
	docFreq := map[string]int{}
	for i, cand := range candidates {
		tf := map[string]int{}
		tokens := tokenize(cand.Memory.Content)
		for _, token := range tokens {
			tf[token]++
		}
		docs[i] = tf
		totalLen += len(tokens)
		for token := range tf {
			docFreq[token]++
		}
	}
	avgLen := float64(totalLen) / float64(len(candidates))
	if avgLen == 0 {
		avgLen = 1
	}

	n := float64(len(candidates))
	var maxScore float64
	scores := make([]float64, len(candidates))
	for i, tf := range docs {
		docLen := 0
		for _, count := range tf {
			docLen += count
		}
		var score float64
		for _, term := range queryTerms {
			freq := float64(tf[term])
			if freq == 0 {
				continue
			}
			idf := math.Log(1 + (n-float64(docFreq[term])+0.5)/(float64(docFreq[term])+0.5))
			score += idf * (freq * (k1 + 1)) / (freq + k1*(1-b+b*float64(docLen)/avgLen))
		}
		scores[i] = score
		if score > maxScore {
			maxScore = score
		}
	}

	for i := range candidates {
		normalized := 0.0
		if maxScore > 0 {
			normalized = scores[i] / maxScore
		}
		candidates[i].Score = 0.5*candidates[i].Score + 0.5*normalized
		if normalized > 0 {
			candidates[i].Reasons = append(candidates[i].Reasons, "local_rerank")
		}
	}
	retrieval.SortCandidates(candidates)
	return candidates, nil
}

func tokenize(text string) []string {
	fields := strings.Fields(strings.ToLower(text))
	out := make([]string, 0, len(fields))
	for _, token := range fields {
		token = strings.Trim(token, " .,;:!?()[]{}\"'")
		if len(token) > 2 {
			out = append(out, token)
		}
	}
	return out
}
