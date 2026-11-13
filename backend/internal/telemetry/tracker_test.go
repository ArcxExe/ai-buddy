package telemetry

import (
	"sync"
	"testing"
	"time"

	"aibuddy/backend/internal/domain"
)

func TestTrackerRecordAndRecentOrdering(t *testing.T) {
	tr := NewTracker(true)
	if !tr.Enabled() {
		t.Fatalf("expected tracker enabled")
	}

	var sinkCount int
	var mu sync.Mutex
	tr.SetSink(func(ev ClickstreamEvent) {
		mu.Lock()
		sinkCount++
		mu.Unlock()
	})

	tr.Record(ClickstreamEvent{
		EventType: EventChecklistUpdated,
		ActorRole: domain.RoleIntern,
		ActorID:   "usr-intern-01",
		Metadata:  map[string]string{"item_id": "step-01-access"},
	})
	tr.Record(ClickstreamEvent{
		EventType: EventRBACAccessDenied,
		ActorRole: domain.RoleIntern,
		Metadata:  map[string]string{"resource": "prod-billing-db"},
	})

	recent := tr.ListRecent(1)
	if len(recent) != 1 || recent[0].EventType != EventRBACAccessDenied {
		t.Fatalf("expected newest event first, got %+v", recent)
	}

	all := tr.ListRecent(10)
	if len(all) != 2 || all[1].Metadata["item_id"] != "step-01-access" {
		t.Fatalf("unexpected event history: %+v", all)
	}

	counts := tr.CountByType()
	if counts[EventChecklistUpdated] != 1 || counts[EventRBACAccessDenied] != 1 {
		t.Fatalf("unexpected counts: %+v", counts)
	}

	mu.Lock()
	if sinkCount != 2 {
		t.Fatalf("expected sink called 2 times, got %d", sinkCount)
	}
	mu.Unlock()
}

func TestTrackerLatencyAndDisabledState(t *testing.T) {
	tr := NewTracker(true)
	if tr.AvgLocalLatencyMs() != 0 {
		t.Fatalf("expected zero initial latency")
	}

	tr.RecordLatency("/api/v1/context", domain.RoleIntern, 12*time.Millisecond)
	tr.RecordLatency("/api/v1/auth/switch-role", domain.RoleMentor, 4*time.Millisecond)
	tr.RecordLatency("/api/v1/health", domain.RoleIntern, 0)

	avg := tr.AvgLocalLatencyMs()
	if avg <= 0 || avg > 20 {
		t.Fatalf("unexpected average latency: %v", avg)
	}

	tr.SetEnabled(false)
	if tr.Enabled() {
		t.Fatalf("expected tracker disabled")
	}
	before := len(tr.ListRecent(100))
	tr.Record(ClickstreamEvent{EventType: EventRoleSwitched, ActorRole: domain.RoleManager})
	if len(tr.ListRecent(100)) != before {
		t.Fatalf("expected no new events when disabled")
	}

	tr.Reset()
	if len(tr.ListRecent(10)) != 0 || tr.AvgLocalLatencyMs() != 0 || len(tr.CountByType()) != 0 {
		t.Fatalf("expected clean state after Reset")
	}
}

func TestTrackerBoundedBufferAndConcurrency(t *testing.T) {
	tr := NewTracker(true)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 80; i++ {
				tr.Record(ClickstreamEvent{
					EventType: EventMCPActionCompleted,
					ActorRole: domain.RoleIntern,
				})
				tr.RecordLatency("/api/v1/context", domain.RoleIntern, time.Millisecond)
			}
		}(g)
	}
	wg.Wait()

	recent := tr.ListRecent(1000)
	if len(recent) != defaultMaxEvents {
		t.Fatalf("expected capped buffer size %d, got %d", defaultMaxEvents, len(recent))
	}
}
