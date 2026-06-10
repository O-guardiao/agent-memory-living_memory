package telemetry

// Define metrics such as memory_retrieval_latency_ms and memory_context_tokens here.

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
)

// Histogram bucket upper bounds in milliseconds/tokens.
var defaultBuckets = []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000}

type Counter struct {
	value atomic.Int64
}

func (c *Counter) Inc()            { c.value.Add(1) }
func (c *Counter) Add(delta int64) { c.value.Add(delta) }
func (c *Counter) Value() int64    { return c.value.Load() }

type Histogram struct {
	mu      sync.Mutex
	buckets []float64
	counts  []int64
	sum     float64
	count   int64
}

func newHistogram() *Histogram {
	return &Histogram{buckets: defaultBuckets, counts: make([]int64, len(defaultBuckets)+1)}
}

func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	idx := sort.SearchFloat64s(h.buckets, value)
	h.counts[idx]++
	h.sum += value
	h.count++
}

// Metrics is a lightweight stdlib registry rendered in Prometheus text
// format. Well-known metrics are pre-registered.
type Metrics struct {
	mu         sync.Mutex
	counters   map[string]*Counter
	histograms map[string]*Histogram
}

func NewMetrics() *Metrics {
	m := &Metrics{
		counters:   map[string]*Counter{},
		histograms: map[string]*Histogram{},
	}
	m.Counter("http_requests_total")
	m.Histogram("memory_retrieval_latency_ms")
	m.Histogram("memory_context_tokens")
	return m
}

func (m *Metrics) Counter(name string) *Counter {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.counters[name]; ok {
		return c
	}
	c := &Counter{}
	m.counters[name] = c
	return c
}

func (m *Metrics) Histogram(name string) *Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()
	if h, ok := m.histograms[name]; ok {
		return h
	}
	h := newHistogram()
	m.histograms[name] = h
	return h
}

// Handler renders the registry in Prometheus exposition text format.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		m.mu.Lock()
		counterNames := make([]string, 0, len(m.counters))
		for name := range m.counters {
			counterNames = append(counterNames, name)
		}
		histogramNames := make([]string, 0, len(m.histograms))
		for name := range m.histograms {
			histogramNames = append(histogramNames, name)
		}
		m.mu.Unlock()
		sort.Strings(counterNames)
		sort.Strings(histogramNames)

		for _, name := range counterNames {
			fmt.Fprintf(w, "# TYPE %s counter\n%s %d\n", name, name, m.Counter(name).Value())
		}
		for _, name := range histogramNames {
			h := m.Histogram(name)
			h.mu.Lock()
			fmt.Fprintf(w, "# TYPE %s histogram\n", name)
			cumulative := int64(0)
			for i, bound := range h.buckets {
				cumulative += h.counts[i]
				fmt.Fprintf(w, "%s_bucket{le=%q} %d\n", name, fmt.Sprintf("%g", bound), cumulative)
			}
			cumulative += h.counts[len(h.buckets)]
			fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", name, cumulative)
			fmt.Fprintf(w, "%s_sum %g\n", name, h.sum)
			fmt.Fprintf(w, "%s_count %d\n", name, h.count)
			h.mu.Unlock()
		}
	})
}
