package hashembed

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
)

type Embedder struct {
	Dim int
}

func New(dim int) *Embedder {
	if dim <= 0 {
		dim = 64
	}
	return &Embedder{Dim: dim}
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float64, error) {
	_ = ctx
	vec := make([]float64, e.Dim)
	for _, token := range strings.Fields(strings.ToLower(text)) {
		h := fnv.New64a()
		_, _ = h.Write([]byte(token))
		idx := int(h.Sum64() % uint64(e.Dim))
		vec[idx] += 1
	}
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	if norm == 0 {
		return vec, nil
	}
	norm = math.Sqrt(norm)
	for i := range vec {
		vec[i] /= norm
	}
	return vec, nil
}

func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, 0, len(texts))
	for _, text := range texts {
		vec, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		out = append(out, vec)
	}
	return out, nil
}
