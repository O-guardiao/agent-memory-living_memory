package telemetry

// Initialize OpenTelemetry exporters here.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// otlpExporter is a minimal OTLP/HTTP-JSON span exporter: fire-and-forget
// with a bounded queue, honoring the file's intent without pulling the
// OpenTelemetry SDK tree. Configure via Init (MEMORY_OTLP_ENDPOINT).
type otlpExporter struct {
	endpoint string
	client   *http.Client
	queue    chan otlpSpan
	once     sync.Once
}

type otlpSpan struct {
	name     string
	start    time.Time
	duration time.Duration
	err      error
}

var (
	exporterMu sync.RWMutex
	exporter   *otlpExporter
)

// Init enables OTLP span export to the given endpoint ("" disables it).
func Init(otlpEndpoint string) {
	exporterMu.Lock()
	defer exporterMu.Unlock()
	if otlpEndpoint == "" {
		exporter = nil
		return
	}
	exporter = &otlpExporter{
		endpoint: otlpEndpoint,
		client:   &http.Client{Timeout: 5 * time.Second},
		queue:    make(chan otlpSpan, 256),
	}
	exporter.once.Do(func() { go exporter.loop() })
}

func exportSpan(name string, start time.Time, duration time.Duration, err error) {
	exporterMu.RLock()
	current := exporter
	exporterMu.RUnlock()
	if current == nil {
		return
	}
	select {
	case current.queue <- otlpSpan{name: name, start: start, duration: duration, err: err}:
	default:
		// Queue full: drop rather than block the hot path.
	}
}

func (e *otlpExporter) loop() {
	for span := range e.queue {
		e.send(span)
	}
}

func (e *otlpExporter) send(span otlpSpan) {
	status := map[string]any{"code": 1}
	if span.err != nil {
		status = map[string]any{"code": 2, "message": span.err.Error()}
	}
	payload := map[string]any{
		"resourceSpans": []any{map[string]any{
			"resource": map[string]any{
				"attributes": []any{map[string]any{
					"key":   "service.name",
					"value": map[string]any{"stringValue": "agent-memory"},
				}},
			},
			"scopeSpans": []any{map[string]any{
				"scope": map[string]any{"name": "agent-memory/telemetry"},
				"spans": []any{map[string]any{
					"traceId":           randomHex(16),
					"spanId":            randomHex(8),
					"name":              span.name,
					"kind":              1,
					"startTimeUnixNano": span.start.UnixNano(),
					"endTimeUnixNano":   span.start.Add(span.duration).UnixNano(),
					"status":            status,
				}},
			}},
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	resp, err := e.client.Post(e.endpoint+"/v1/traces", "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}
