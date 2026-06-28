package timeline

import (
	"errors"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// Listen subscribes to the event bus and appends every event to the timeline.
// This enables zero-config integration: the main wire-up passes the bus to
// this helper, and all events flow into the log automatically.
// Returns the cancel function from the bus subscription.
func Listen(bus contracts.EventBus, tl *Timeline) func() {
	events, cancel := bus.Subscribe()
	go func() {
		for ev := range events {
			tl.Append(ev)
		}
	}()
	return cancel
}

// Replay applies all events in the timeline to the state store, in order.
// Used for deterministic replay at a specific simulation time.
// The caller must ensure the state is at the correct baseline (e.g., scenario initial state).
// Rejection errors are expected and collected; other errors cause immediate return.
func Replay(tl *Timeline, store contracts.StateStore, upTo contracts.SimTime) (finalVer contracts.StateVersion, rejected []contracts.Event, err error) {
	for _, entry := range tl.All() {
		ev := entry.Event
		if ev.Timestamp > upTo {
			break
		}
		ver, applyErr := store.Apply(ev)
		if applyErr != nil {
			var re *contracts.RejectionError
			if errors.As(applyErr, &re) {
				// Expected during replay; collect and continue.
				rejected = append(rejected, ev)
				continue
			}
			// Unexpected error type — fail fast.
			return finalVer, rejected, applyErr
		}
		finalVer = ver
	}
	return finalVer, rejected, nil
}
