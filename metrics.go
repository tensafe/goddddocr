package goddddocr

import (
	"strconv"
	"sync/atomic"
	"time"
)

type ServerMetricsSnapshot struct {
	StartedAt          string            `json:"started_at"`
	UptimeSeconds      float64           `json:"uptime_seconds"`
	TotalRequests      uint64            `json:"total_requests"`
	InFlightRequests   int64             `json:"in_flight_requests"`
	CompletedRequests  uint64            `json:"completed_requests"`
	ErrorRequests      uint64            `json:"error_requests"`
	StatusCodes        map[string]uint64 `json:"status_codes"`
	TotalLatencyMS     float64           `json:"total_latency_ms"`
	AverageLatencyMS   float64           `json:"average_latency_ms"`
	MaxLatencyMS       float64           `json:"max_latency_ms"`
	LastRequestAt      string            `json:"last_request_at,omitempty"`
	LastRequestStatus  int               `json:"last_request_status,omitempty"`
	LastRequestLatency float64           `json:"last_request_latency_ms,omitempty"`
}

type serverMetrics struct {
	startedAt          time.Time
	totalRequests      atomic.Uint64
	inFlightRequests   atomic.Int64
	completedRequests  atomic.Uint64
	errorRequests      atomic.Uint64
	totalLatencyMicros atomic.Uint64
	maxLatencyMicros   atomic.Uint64
	statusCodes        [600]atomic.Uint64
	lastRequestUnix    atomic.Int64
	lastRequestStatus  atomic.Int64
	lastLatencyMicros  atomic.Uint64
}

func newServerMetrics(startedAt time.Time) *serverMetrics {
	return &serverMetrics{startedAt: startedAt.UTC()}
}

func (m *serverMetrics) start() {
	m.totalRequests.Add(1)
	m.inFlightRequests.Add(1)
}

func (m *serverMetrics) finish(status int, duration time.Duration) {
	m.inFlightRequests.Add(-1)
	m.completedRequests.Add(1)
	if status >= httpStatusBadRequest {
		m.errorRequests.Add(1)
	}
	if status >= 0 && status < len(m.statusCodes) {
		m.statusCodes[status].Add(1)
	}

	micros := uint64(duration.Microseconds())
	m.totalLatencyMicros.Add(micros)
	m.lastLatencyMicros.Store(micros)
	m.lastRequestStatus.Store(int64(status))
	m.lastRequestUnix.Store(time.Now().UTC().UnixNano())
	for {
		current := m.maxLatencyMicros.Load()
		if micros <= current || m.maxLatencyMicros.CompareAndSwap(current, micros) {
			return
		}
	}
}

func (m *serverMetrics) snapshot(now time.Time) ServerMetricsSnapshot {
	completed := m.completedRequests.Load()
	totalLatencyMicros := m.totalLatencyMicros.Load()

	statusCodes := map[string]uint64{}
	for status := range m.statusCodes {
		count := m.statusCodes[status].Load()
		if count > 0 {
			statusCodes[strconv.Itoa(status)] = count
		}
	}

	var averageLatencyMS float64
	if completed > 0 {
		averageLatencyMS = microsToMillis(totalLatencyMicros) / float64(completed)
	}

	snapshot := ServerMetricsSnapshot{
		StartedAt:          m.startedAt.Format(time.RFC3339Nano),
		UptimeSeconds:      now.Sub(m.startedAt).Seconds(),
		TotalRequests:      m.totalRequests.Load(),
		InFlightRequests:   m.inFlightRequests.Load(),
		CompletedRequests:  completed,
		ErrorRequests:      m.errorRequests.Load(),
		StatusCodes:        statusCodes,
		TotalLatencyMS:     microsToMillis(totalLatencyMicros),
		AverageLatencyMS:   averageLatencyMS,
		MaxLatencyMS:       microsToMillis(m.maxLatencyMicros.Load()),
		LastRequestStatus:  int(m.lastRequestStatus.Load()),
		LastRequestLatency: microsToMillis(m.lastLatencyMicros.Load()),
	}
	if lastUnix := m.lastRequestUnix.Load(); lastUnix > 0 {
		snapshot.LastRequestAt = time.Unix(0, lastUnix).UTC().Format(time.RFC3339Nano)
	}
	return snapshot
}

func microsToMillis(micros uint64) float64 {
	return float64(micros) / 1000.0
}

const httpStatusBadRequest = 400
