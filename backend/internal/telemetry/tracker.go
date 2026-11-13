package telemetry

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"aibuddy/backend/internal/domain"
)

const defaultMaxEvents = 500

// Event types aligned with Avito Clickstream specification.
const (
	EventChatMessageSent      = "chat_message_sent"
	EventMCPActionCompleted   = "mcp_action_completed"
	EventRBACAccessDenied     = "rbac_access_denied"
	EventChecklistUpdated     = "checklist_updated"
	EventRoleSwitched         = "role_switched"
	EventRequestConfirmed     = "request_confirmed"
	EventRequestReviewed      = "request_reviewed"
	EventProactiveHintCreated = "proactive_hint_generated"
	EventHTTPRequest          = "http_request"
)

// ClickstreamEvent captures a structured audit/analytics event without leaking PII.
type ClickstreamEvent struct {
	ID        string            `json:"id"`
	EventType string            `json:"event_type"`
	ActorRole domain.Role       `json:"actor_role"`
	ActorID   string            `json:"actor_id,omitempty"`
	Route     string            `json:"route,omitempty"`
	LatencyMs float64           `json:"latency_ms,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp string            `json:"timestamp"`
}

// ClickstreamTracker records runtime telemetry and latency metrics.
type ClickstreamTracker interface {
	Record(event ClickstreamEvent)
	RecordLatency(route string, actorRole domain.Role, latency time.Duration)
	AvgLocalLatencyMs() float64
	ListRecent(limit int) []ClickstreamEvent
	CountByType() map[string]int
	Reset()
	SetEnabled(enabled bool)
	Enabled() bool
}

// Tracker is a thread-safe in-memory Clickstream collector with bounded retention.
type Tracker struct {
	mu           sync.RWMutex
	enabled      bool
	maxEvents    int
	events       []ClickstreamEvent
	counts       map[string]int
	latencySum   float64
	latencyCount int
	seq          atomic.Uint64
	sink         func(ClickstreamEvent)
}

// NewTracker creates a ClickstreamTracker. Passing enabled=false keeps telemetry no-op except latency measurement.
func NewTracker(enabled bool) *Tracker {
	return &Tracker{
		enabled:   enabled,
		maxEvents: defaultMaxEvents,
		events:    make([]ClickstreamEvent, 0, 64),
		counts:    make(map[string]int),
	}
}

// SetSink registers an optional asynchronous or persistent callback for recorded events.
func (t *Tracker) SetSink(sink func(ClickstreamEvent)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sink = sink
}

// Enabled reports whether event recording is active.
func (t *Tracker) Enabled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.enabled
}

// SetEnabled toggles Clickstream event recording at runtime.
func (t *Tracker) SetEnabled(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enabled = enabled
}

// Record stores an audit/clickstream event if telemetry is enabled.
func (t *Tracker) Record(event ClickstreamEvent) {
	t.mu.Lock()
	if !t.enabled {
		t.mu.Unlock()
		return
	}

	if event.ID == "" {
		n := t.seq.Add(1)
		event.ID = fmt.Sprintf("evt-%05d", n)
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	event.Metadata = cloneMetadata(event.Metadata)

	t.counts[event.EventType]++
	if len(t.events) >= t.maxEvents {
		copy(t.events, t.events[1:])
		t.events[len(t.events)-1] = event
	} else {
		t.events = append(t.events, event)
	}
	sink := t.sink
	t.mu.Unlock()

	if sink != nil {
		sink(event)
	}
}

// RecordLatency records endpoint response duration in milliseconds for SLA monitoring (<50ms target).
func (t *Tracker) RecordLatency(route string, actorRole domain.Role, latency time.Duration) {
	ms := float64(latency.Microseconds()) / 1000.0
	if ms < 0.01 {
		ms = 0.01
	}

	t.mu.Lock()
	t.latencySum += ms
	t.latencyCount++
	enabled := t.enabled
	t.mu.Unlock()

	if enabled && route != "" {
		t.Record(ClickstreamEvent{
			EventType: EventHTTPRequest,
			ActorRole: actorRole,
			Route:     route,
			LatencyMs: math.Round(ms*100) / 100,
		})
	}
}

// AvgLocalLatencyMs returns the rolling mean latency of measured local operations in milliseconds.
func (t *Tracker) AvgLocalLatencyMs() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.latencyCount == 0 {
		return 0
	}
	return math.Round((t.latencySum/float64(t.latencyCount))*100) / 100
}

// ListRecent returns up to limit most recent events ordered newest-first.
func (t *Tracker) ListRecent(limit int) []ClickstreamEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.events) {
		limit = len(t.events)
	}
	out := make([]ClickstreamEvent, 0, limit)
	for i := len(t.events) - 1; i >= len(t.events)-limit; i-- {
		cp := t.events[i]
		cp.Metadata = cloneMetadata(cp.Metadata)
		out = append(out, cp)
	}
	return out
}

// CountByType returns a copy of aggregated counts grouped by EventType.
func (t *Tracker) CountByType() map[string]int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]int, len(t.counts))
	for k, v := range t.counts {
		out[k] = v
	}
	return out
}

// Reset clears recorded events and rolling latency stats.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = t.events[:0]
	t.counts = make(map[string]int)
	t.latencySum = 0
	t.latencyCount = 0
}

func cloneMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
