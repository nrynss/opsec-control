package orchestrator

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// Compile-time interface assertion — catches signature drift at build time,
// not at wiring time in cmd/eoc.
var _ contracts.Orchestrator = (*Engine)(nil)

// Engine implements contracts.Orchestrator. It is the ONLY place Cells are
// invoked, and it does so concurrently — sequential invocation is a spec
// violation (SPEC §1, §6).
type Engine struct {
	cells map[contracts.CellKind]contracts.Cell
}

// NewEngine creates a new orchestrator engine with the given set of Cells.
// The caller's map is defensively copied; subsequent mutations to the original
// map do not affect the Engine.
func NewEngine(cells map[contracts.CellKind]contracts.Cell) *Engine {
	cp := make(map[contracts.CellKind]contracts.Cell, len(cells))
	maps.Copy(cp, cells)
	return &Engine{cells: cp}
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
//
// Context cancellation: if ctx is cancelled while specialists are running,
// FanOut returns ctx.Err() promptly rather than blocking until every cell
// finishes on its own.
func (e *Engine) FanOut(ctx context.Context, snapshot contracts.WorldState, trigger contracts.Event, wake []contracts.CellKind) (contracts.CommonOperationalPicture, error) {
	startTime := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Filter out the Commander — it runs after specialists, not in parallel
	// with them. The Commander ALWAYS synthesises when registered, regardless
	// of whether it appears in the wake list (§6: "Commander synthesizes →
	// COP + prioritized actions" is an unconditional phase-2 step).
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

	// Wait for all goroutines OR context cancellation, whichever comes first.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All specialists finished — continue to collection.
	case <-ctx.Done():
		return contracts.CommonOperationalPicture{}, ctx.Err()
	}

	// Collect successful outputs; record errors but don't fail the whole fan-out
	// if a single specialist errors (the Commander still synthesizes what's available).
	var specialistOutputs []contracts.CellOutput
	var errs []error
	for i, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		// Stamp the StateVersion from the orchestrator's snapshot rather than
		// trusting the cell's self-reported value (defense in depth).
		r.output.StateVersion = snapshot.Version
		r.output.Cell = specialistKinds[i]
		specialistOutputs = append(specialistOutputs, r.output)
	}

	// If every single specialist failed, return an error.
	if len(specialistOutputs) == 0 && len(errs) > 0 {
		return contracts.CommonOperationalPicture{}, fmt.Errorf("all %d specialist cells failed; first error: %w", len(errs), errs[0])
	}

	// --- Phase 2: Commander synthesis ---
	commander, hasCommander := e.cells[contracts.CellCommander]
	if !hasCommander {
		latencyMS := time.Since(startTime).Milliseconds()
		if len(specialistOutputs) == 0 {
			return contracts.CommonOperationalPicture{
				Summary:      "No cells woken for this event.",
				StateVersion: snapshot.Version,
				OverallRisk:  contracts.RiskLow,
				Metrics: contracts.COPMetrics{
					FanOutLatencyMS: latencyMS,
					CellCount:       0,
				},
			}, nil
		}
		// No Commander registered — return a best-effort COP from specialist outputs.
		return buildFallbackCOP(snapshot, specialistOutputs, nil, latencyMS), nil
	}

	commanderInput := contracts.CellInput{
		Snapshot:     snapshot,
		Trigger:      trigger,
		StateVersion: snapshot.Version,
		Peers:        specialistOutputs,
	}

	commanderOut, cmdErr := commander.Analyze(ctx, commanderInput)
	latencyMS := time.Since(startTime).Milliseconds()
	if cmdErr != nil {
		// Commander failed — return a best-effort COP but surface the error in
		// the summary so the failure is visible on the HUD (not silently swallowed).
		return buildFallbackCOP(snapshot, specialistOutputs, cmdErr, latencyMS), nil
	}

	// Build the COP from the Commander's output.
	cop := contracts.CommonOperationalPicture{
		Summary:      commanderOut.Summary,
		StateVersion: snapshot.Version,
		OverallRisk:  commanderOut.RiskLevel,
		CellOutputs:  specialistOutputs,
		Metrics:      computeCOPMetrics(latencyMS, specialistOutputs, &commanderOut),
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

// computeCOPMetrics aggregates metrics across a fan-out run.
func computeCOPMetrics(latencyMS int64, specialists []contracts.CellOutput, commander *contracts.CellOutput) contracts.COPMetrics {
	var totalIn, totalOut int
	var peak float64
	cellCount := len(specialists)

	for _, out := range specialists {
		totalIn += out.Metrics.TokensIn
		totalOut += out.Metrics.TokensOut
		if out.Metrics.TokensPerSec > peak {
			peak = out.Metrics.TokensPerSec
		}
	}

	if commander != nil {
		cellCount++
		totalIn += commander.Metrics.TokensIn
		totalOut += commander.Metrics.TokensOut
		if commander.Metrics.TokensPerSec > peak {
			peak = commander.Metrics.TokensPerSec
		}
	}

	var aggregate float64
	if latencyMS > 0 {
		aggregate = float64(totalOut) / (float64(latencyMS) / 1000.0)
	} else {
		aggregate = peak
	}

	return contracts.COPMetrics{
		FanOutLatencyMS:       latencyMS,
		TotalTokensIn:         totalIn,
		TotalTokensOut:        totalOut,
		PeakTokensPerSec:      peak,
		AggregateTokensPerSec: aggregate,
		CellCount:             cellCount,
	}
}

// buildFallbackCOP constructs a best-effort COP when the Commander is missing
// or failed. If cmdErr is non-nil, the failure is surfaced in the summary.
func buildFallbackCOP(snapshot contracts.WorldState, outputs []contracts.CellOutput, cmdErr error, latencyMS int64) contracts.CommonOperationalPicture {
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

	summary := "Automated COP — Commander unavailable."
	if cmdErr != nil {
		summary = fmt.Sprintf("Automated COP — Commander failed: %v", cmdErr)
	}

	return contracts.CommonOperationalPicture{
		Summary:      summary,
		StateVersion: snapshot.Version,
		OverallRisk:  overallRisk,
		CellOutputs:  outputs,
		Metrics:      computeCOPMetrics(latencyMS, outputs, nil),
	}
}
