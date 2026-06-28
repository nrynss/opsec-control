package simulation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// mockBus is a simple EventBus for testing. It records published events.
type mockBus struct {
	mu     sync.Mutex
	events []contracts.Event
}

func (m *mockBus) Publish(e contracts.Event) {
	m.mu.Lock()
	m.events = append(m.events, e)
	m.mu.Unlock()
}

func (m *mockBus) Subscribe() (<-chan contracts.Event, func()) {
	ch := make(chan contracts.Event)
	return ch, func() { close(ch) }
}

func (m *mockBus) published() []contracts.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]contracts.Event, len(m.events))
	copy(cp, m.events)
	return cp
}

func makeScenario(events []contracts.Event) *contracts.Scenario {
	return &contracts.Scenario{
		SchemaVersion: "0.1",
		Name:          "test",
		Seed:          12345,
		Initial:       contracts.WorldState{Version: 0, Time: 0},
		Events:        events,
	}
}

func TestEngine_StepAndCurrentTime(t *testing.T) {
	bus := &mockBus{}
	eng := New(bus)

	sc := makeScenario([]contracts.Event{
		{ID: "e1", Timestamp: 0, Source: "sim", Type: contracts.EventMainshockOccurred, Confidence: 1.0},
		{ID: "e2", Timestamp: 90, Source: "sim", Type: contracts.EventBridgeClosed, Confidence: 0.9},
	})

	if err := eng.Load(sc); err != nil {
		t.Fatal(err)
	}

	if eng.CurrentTime() != 0 {
		t.Errorf("initial currentTime = %d, want 0", eng.CurrentTime())
	}

	more, err := eng.Step()
	if err != nil || !more {
		t.Fatalf("step1: more=%v err=%v", more, err)
	}
	if got := eng.CurrentTime(); got != 0 {
		t.Errorf("after first step time=%d, want 0", got)
	}

	more, err = eng.Step()
	if err != nil || !more {
		t.Fatalf("step2: more=%v err=%v", more, err)
	}
	if got := eng.CurrentTime(); got != 90 {
		t.Errorf("after second step time=%d, want 90", got)
	}

	more, err = eng.Step()
	if err != nil || more {
		t.Fatalf("step3 at end: more=%v err=%v", more, err)
	}

	pubs := bus.published()
	if len(pubs) != 2 {
		t.Fatalf("published %d events, want 2", len(pubs))
	}
	if pubs[0].ID != "e1" || pubs[1].ID != "e2" {
		t.Errorf("wrong event order: %+v", pubs)
	}
}

func TestEngine_Reset(t *testing.T) {
	bus := &mockBus{}
	eng := New(bus)
	sc := makeScenario([]contracts.Event{
		{ID: "e1", Timestamp: 10, Source: "s", Type: contracts.EventMainshockOccurred, Confidence: 1},
		{ID: "e2", Timestamp: 20, Source: "s", Type: contracts.EventAftershockOccurred, Confidence: 1},
	})
	_ = eng.Load(sc)
	_, _ = eng.Step()
	_, _ = eng.Step()

	eng.Reset()
	if eng.CurrentTime() != 0 {
		t.Errorf("after reset time=%d, want 0", eng.CurrentTime())
	}

	// Replay from start
	_, _ = eng.Step()
	if eng.CurrentTime() != 10 {
		t.Errorf("replayed first time=%d, want 10", eng.CurrentTime())
	}
}

func TestEngine_RunPaced(t *testing.T) {
	bus := &mockBus{}
	eng := New(bus)
	sc := makeScenario([]contracts.Event{
		{ID: "e0", Timestamp: 0, Source: "s", Type: contracts.EventMainshockOccurred, Confidence: 1},
		{ID: "e1", Timestamp: 2, Source: "s", Type: contracts.EventBridgeClosed, Confidence: 1},
	})
	_ = eng.Load(sc)

	// Use a short timeout context; with speed=1 this would take ~2s real time.
	// We use high speed to make test fast.
	eng.SetSpeed(1000)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := eng.Run(ctx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	pubs := bus.published()
	if len(pubs) != 2 {
		t.Fatalf("Run emitted %d events, want 2", len(pubs))
	}
}

func TestEngine_PauseResume(t *testing.T) {
	bus := &mockBus{}
	eng := New(bus)
	sc := makeScenario([]contracts.Event{
		{ID: "e0", Timestamp: 0, Source: "s", Type: contracts.EventMainshockOccurred, Confidence: 1},
		{ID: "e1", Timestamp: 1, Source: "s", Type: contracts.EventAftershockOccurred, Confidence: 1},
	})
	_ = eng.Load(sc)
	eng.SetSpeed(100) // still reasonably fast

	eng.Pause()

	done := make(chan error, 1)
	go func() {
		done <- eng.Run(context.Background())
	}()

	// Give the goroutine a moment to enter the paused state.
	time.Sleep(10 * time.Millisecond)

	// While paused, Step should still work.
	more, _ := eng.Step()
	if !more {
		t.Fatal("expected to be able to Step while paused")
	}

	// Now resume; the Run should continue and finish the remaining event.
	eng.Resume()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run after resume failed: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not complete after Resume")
	}

	if len(bus.published()) < 1 {
		t.Error("expected at least one event published")
	}
}

func TestEngine_DeterministicReplay(t *testing.T) {
	// Two separate engines with same scenario must emit identical sequences.
	sc := makeScenario([]contracts.Event{
		{ID: "a", Timestamp: 5, Source: "s", Type: contracts.EventMainshockOccurred, Confidence: 1},
		{ID: "b", Timestamp: 15, Source: "s", Type: contracts.EventFireIgnited, Confidence: 0.8},
	})

	bus1 := &mockBus{}
	e1 := New(bus1)
	_ = e1.Load(sc)
	for {
		more, _ := e1.Step()
		if !more {
			break
		}
	}

	bus2 := &mockBus{}
	e2 := New(bus2)
	_ = e2.Load(sc)
	for {
		more, _ := e2.Step()
		if !more {
			break
		}
	}

	p1 := bus1.published()
	p2 := bus2.published()
	if len(p1) != len(p2) {
		t.Fatalf("different lengths: %d vs %d", len(p1), len(p2))
	}
	for i := range p1 {
		if p1[i].ID != p2[i].ID || p1[i].Timestamp != p2[i].Timestamp {
			t.Errorf("event %d differs: %+v vs %+v", i, p1[i], p2[i])
		}
	}
}

func TestEngine_NoWallClockInLogic(t *testing.T) {
	// The only place wall time is read is inside Run for sleeps.
	// Step and CurrentTime are pure with respect to the scenario.
	// This is a documentation / review test; actual enforcement is by code inspection.
	bus := &mockBus{}
	eng := New(bus)
	sc := makeScenario([]contracts.Event{{ID: "only", Timestamp: 42, Source: "s", Type: contracts.EventMainshockOccurred, Confidence: 1}})
	_ = eng.Load(sc)

	start := time.Now()
	_, _ = eng.Step()
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		t.Errorf("Step took wall time %v; logic must not wait on wall clock", elapsed)
	}
	if eng.CurrentTime() != 42 {
		t.Errorf("current time after step = %d", eng.CurrentTime())
	}
}

// TestEngine_ResetInterruptsRun exercises that Reset() during a Run() sleep
// causes Run() to pick up the reset state instead of continuing with stale idx.
// This is a regression test for the previous race + stale state bugs.
func TestEngine_ResetInterruptsRun(t *testing.T) {
	bus := &mockBus{}
	eng := New(bus)

	sc := makeScenario([]contracts.Event{
		{ID: "e0", Timestamp: 0, Source: "s", Type: contracts.EventMainshockOccurred, Confidence: 1},
		{ID: "e1", Timestamp: 500, Source: "s", Type: contracts.EventBridgeClosed, Confidence: 1}, // long gap
		{ID: "e2", Timestamp: 501, Source: "s", Type: contracts.EventAftershockOccurred, Confidence: 1},
	})
	_ = eng.Load(sc)
	eng.SetSpeed(1) // real time scale → will sleep a long time for e1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- eng.Run(ctx)
	}()

	// Let Run() enter the sleep waiting for the large delta to e1
	time.Sleep(15 * time.Millisecond)

	// Reset while sleeping — this should wake the waiter via resetCh
	// and make Run() see the reset state on next iteration.
	eng.Reset()

	// Let it react
	time.Sleep(15 * time.Millisecond)

	// Cancel the Run so it exits cleanly
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled && err != nil {
			t.Fatalf("Run exited with unexpected error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Run did not exit after cancel")
	}

	pubs := bus.published()

	// We should never have seen the long-gap event (e1 at t=500) because
	// we reset before it could fire.
	for _, p := range pubs {
		if p.ID == "e1" {
			t.Errorf("Run published stale future event e1 after Reset(): %+v", p)
		}
	}
}
