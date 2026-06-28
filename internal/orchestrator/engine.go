package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// Engine implements contracts.Orchestrator. It is the ONLY place Cells are
// invoked, and it does so concurrently — sequential invocation is a spec
// violation (SPEC §1, §6).
type Engine struct {
	cells map[contracts.CellKind]contracts.Cell
}

// NewEngine creates a new orchestrator engine with the given set of Cells.
// The Commander cell must be included in the cells map.
func NewEngine(cells map[contracts.CellKind]contracts.Cell) *Engine {
	return &Engine{cells: cells}
}

// cellResult pairs a CellOutput with an error for channel-based collection.
type cellResult struct {
	output contracts.CellOutput
	err    error
}

// FanOut fires the specified Cells concurrently on the given world-state
// snapshot and triggering event, collects their structured outputs, then
// invokes the Commander to synthesize the CommonOperationalPicture.
//
// Cells are invoked as simultaneous goroutines — the entire specialist phase
// should complete in under ~500ms on Cerebras (SPEC §1). Sequential invocation
// is a spec violation.
//
// The orchestrator reads a snapshot and Cells return data; it never mutates
// world state (SPEC §6, §16.1).
func (e *Engine) FanOut(ctx context.Context, snapshot contracts.WorldState, trigger contracts.Event, wake []contracts.CellKind) (contracts.CommonOperationalPicture, error) {
	if len(wake) == 0 {
		return contracts.CommonOperationalPicture{
			Summary:      "No cells woken for this event.",
			StateVersion: snapshot.Version,
			OverallRisk:  contracts.RiskLow,
		}, nil
	}

	// Filter out the Commander — it runs after specialists, not in parallel with them.
	var specialistKinds []contracts.CellKind
	for _, k := range wake {
		if k != contracts.CellCommander {
			specialistKinds = append(specialistKinds, k)
		}
	}

	// --- Phase 1: Concurrent specialist fan-out ---
	results := make([]cellResult, len(specialistKinds))
	var wg sync.WaitGroup
	wg.Add(len(specialistKinds))

	for i, kind := range specialistKinds {
		go func(idx int, k contracts.CellKind) {
			defer wg.Done()

			cell, ok := e.cells[k]
			if !ok {
				results[idx] = cellResult{err: fmt.Errorf("cell %q not registered", k)}
				return
			}

			input := contracts.CellInput{
				Snapshot:     snapshot,
				Trigger:      trigger,
				StateVersion: snapshot.Version,
			}

			out, err := cell.Analyze(ctx, input)
			results[idx] = cellResult{output: out, err: err}
		}(i, kind)
	}

	wg.Wait()

	// Collect successful outputs; record errors but don't fail the whole fan-out
	// if a single specialist errors (the Commander still synthesizes what's available).
	var specialistOutputs []contracts.CellOutput
	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		specialistOutputs = append(specialistOutputs, r.output)
	}

	// If every single specialist failed, return an error.
	if len(specialistOutputs) == 0 && len(errs) > 0 {
		return contracts.CommonOperationalPicture{}, fmt.Errorf("all %d specialist cells failed; first error: %w", len(errs), errs[0])
	}

	// --- Phase 2: Commander synthesis ---
	commander, hasCommander := e.cells[contracts.CellCommander]
	if !hasCommander {
		// No Commander registered — return a best-effort COP from specialist outputs.
		return buildFallbackCOP(snapshot, specialistOutputs), nil
	}

	commanderInput := contracts.CellInput{
		Snapshot:     snapshot,
		Trigger:      trigger,
		StateVersion: snapshot.Version,
		Peers:        specialistOutputs,
	}

	commanderOut, err := commander.Analyze(ctx, commanderInput)
	if err != nil {
		// Commander failed — still return a best-effort COP.
		return buildFallbackCOP(snapshot, specialistOutputs), nil
	}

	// Build the COP from the Commander's output.
	cop := contracts.CommonOperationalPicture{
		Summary:      commanderOut.Summary,
		StateVersion: snapshot.Version,
		OverallRisk:  commanderOut.RiskLevel,
		CellOutputs:  specialistOutputs,
	}

	// Commander recommendations become prioritized actions.
	for i, rec := range commanderOut.Recommendations {
		cop.PrioritizedActions = append(cop.PrioritizedActions, contracts.PrioritizedAction{
			Priority: i + 1,
			Action:   rec,
			Owner:    commanderOut.Cell,
		})
	}

	return cop, nil
}

// buildFallbackCOP constructs a best-effort COP when the Commander is missing or failed.
func buildFallbackCOP(snapshot contracts.WorldState, outputs []contracts.CellOutput) contracts.CommonOperationalPicture {
	// Determine overall risk as the max across all specialist outputs.
	overallRisk := contracts.RiskLow
	riskOrder := map[contracts.RiskLevel]int{
		contracts.RiskLow:      0,
		contracts.RiskMedium:   1,
		contracts.RiskHigh:     2,
		contracts.RiskCritical: 3,
	}
	for _, o := range outputs {
		if riskOrder[o.RiskLevel] > riskOrder[overallRisk] {
			overallRisk = o.RiskLevel
		}
	}

	return contracts.CommonOperationalPicture{
		Summary:      "Automated COP — Commander unavailable.",
		StateVersion: snapshot.Version,
		OverallRisk:  overallRisk,
		CellOutputs:  outputs,
	}
}
