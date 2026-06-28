package simulation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// Engine is a deterministic scenario replay engine.
// It advances logical SimTime by emitting the scenario's pre-recorded Events
// onto the provided EventBus. It never mutates world state (only the bus).
// The clock, not wall time, drives event emission order.
//
// Controls: Load, Reset, Step (for tests / precise control), Run (paced),
// Pause/Resume, SetSpeed.
//
// Determinism: all ordering and current time decisions are driven exclusively
// by the Scenario's Events and SimTime values + the Seed (for any future RNG).
// Wall time is used only to calculate pacing sleeps during Run.
type Engine struct {
	bus contracts.EventBus

	mu       sync.Mutex
	scenario *contracts.Scenario
	idx      int
	current  contracts.SimTime
	paused   bool
	resumeCh chan struct{}
	resetCh  chan struct{} // closed to interrupt sleepers in Run() on Reset/Load
	speed    float64       // 1.0 = real-time. <=0 means as fast as possible.
	seed     int64
}

// New creates a new Engine that will publish events to the given bus.
func New(bus contracts.EventBus) *Engine {
	if bus == nil {
		panic("simulation: EventBus must not be nil")
	}
	return &Engine{
		bus:     bus,
		speed:   1.0,
		resetCh: make(chan struct{}),
	}
}

// Load installs a new scenario and resets playback to the beginning.
// The scenario is assumed to be validated upstream (§14.2).
func (e *Engine) Load(sc *contracts.Scenario) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if sc == nil {
		return fmt.Errorf("scenario must not be nil")
	}
	e.scenario = sc
	e.seed = sc.Seed
	e.idx = 0
	e.current = 0
	e.paused = false
	e.resumeCh = nil

	// Interrupt any waiter in a concurrent Run() and allocate a fresh notification channel.
	e.interruptResetLocked()
	e.resetCh = make(chan struct{})
	return nil
}

// interruptResetLocked closes the current resetCh (if open) under the assumption
// that the caller already holds e.mu. A fresh channel will be assigned by the caller.
func (e *Engine) interruptResetLocked() {
	if e.resetCh != nil {
		select {
		case <-e.resetCh:
		default:
			close(e.resetCh)
		}
	}
}

// Reset returns playback to the start of the current scenario (time 0, first event).
// If no scenario is loaded, this is a no-op.
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.scenario == nil {
		return
	}
	e.idx = 0
	e.current = 0
	e.paused = false
	e.resumeCh = nil

	// Wake any sleeper in Run() so it re-evaluates the new state.
	e.interruptResetLocked()
	e.resetCh = make(chan struct{})
}

// CurrentTime returns the logical SimTime of the last emitted (or next to be emitted) event.
func (e *Engine) CurrentTime() contracts.SimTime {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.current
}

// Step publishes at most the next event from the scenario and advances logical time.
// Returns (true, nil) if an event was published, (false, nil) if at end or no scenario.
func (e *Engine) Step() (bool, error) {
	e.mu.Lock()
	if e.scenario == nil || e.idx >= len(e.scenario.Events) {
		e.mu.Unlock()
		return false, nil
	}
	ev := e.scenario.Events[e.idx]
	e.current = ev.Timestamp
	e.idx++
	e.mu.Unlock()

	e.bus.Publish(ev)
	return true, nil
}

// SetSpeed sets the playback speed for Run().
// 1.0 = real time (delta SimTime seconds == delta wall seconds).
// Values >1 speed up; values in (0,1) slow down.
// <= 0 means "as fast as possible" (no deliberate sleeps between events).
func (e *Engine) SetSpeed(f float64) {
	e.mu.Lock()
	e.speed = f
	e.mu.Unlock()
}

// Pause stops Run() progress. Subsequent calls to Run will block until Resume or ctx done.
// Step continues to work while paused.
func (e *Engine) Pause() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.paused {
		e.paused = true
		e.resumeCh = make(chan struct{})
	}
}

// Resume unblocks a paused Run().
func (e *Engine) Resume() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.paused {
		e.paused = false
		if e.resumeCh != nil {
			close(e.resumeCh)
			e.resumeCh = nil
		}
	}
}

// getResetCh returns the current reset notification channel.
// Callers should capture the returned channel and select on that specific value
// (to observe a particular "generation" of reset).
func (e *Engine) getResetCh() <-chan struct{} {
	e.mu.Lock()
	ch := e.resetCh
	e.mu.Unlock()
	if ch == nil {
		// Should not normally happen; defensive.
		ch = make(chan struct{})
	}
	return ch
}

// Run drives the scenario to completion (or until ctx is cancelled).
// It sleeps between events proportional to (delta SimTime / speed).
// While paused it waits for Resume or cancellation.
//
// Design notes for correctness:
//   - We snapshot the *resetCh* we are waiting on (a specific generation).
//   - After waking from a reset, we continue the loop instead of publishing
//     a potentially stale pre-sleep event.
//   - After a timer, we re-check that idx is still what we expected before
//     publishing (guards against manual Step(), Reset(), or Load() during wait).
func (e *Engine) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Check for pause.
		e.mu.Lock()
		paused := e.paused
		resCh := e.resumeCh
		e.mu.Unlock()

		if paused {
			resetCh := e.getResetCh()
			select {
			case <-resCh:
			case <-ctx.Done():
				return ctx.Err()
			case <-resetCh:
				// Reset/Load interrupted the pause wait
			}
			continue
		}

		// Not paused. Capture current next event + the resetCh generation we will wait on.
		e.mu.Lock()
		if e.scenario == nil || e.idx >= len(e.scenario.Events) {
			e.mu.Unlock()
			return nil
		}
		nextIdx := e.idx
		nextEv := e.scenario.Events[nextIdx]
		delta := float64(nextEv.Timestamp - e.current)
		sp := e.speed
		resetCh := e.resetCh // snapshot the specific channel for this wait
		e.mu.Unlock()

		waitedForTime := true
		if delta > 0 && sp > 0 {
			sleep := time.Duration(delta/sp) * time.Second
			timer := time.NewTimer(sleep)
			select {
			case <-timer.C:
				// normal time to publish nextEv (if still valid)
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-resetCh:
				timer.Stop()
				waitedForTime = false
				// A Reset or Load happened. Do not publish the old event.
			}
		}

		if !waitedForTime {
			continue // re-evaluate fresh state
		}

		// Time to publish. Re-check under lock that the idx we decided to
		// advance is still the current one (someone may have stepped or reset).
		e.mu.Lock()
		if e.scenario == nil || e.idx != nextIdx {
			e.mu.Unlock()
			continue
		}
		ev := e.scenario.Events[e.idx]
		e.current = ev.Timestamp
		e.idx++
		e.mu.Unlock()

		e.bus.Publish(ev)
	}
}
