package orchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nrynss/opsec-control/internal/contracts"
)

// --- Mock Cell for testing ---

type mockCell struct {
	kind      contracts.CellKind
	delay     time.Duration
	output    contracts.CellOutput
	err       error
	callCount atomic.Int32
}

func (m *mockCell) Kind() contracts.CellKind { return m.kind }

func (m *mockCell) Analyze(_ context.Context, input contracts.CellInput) (contracts.CellOutput, error) {
	m.callCount.Add(1)
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.err != nil {
		return contracts.CellOutput{}, m.err
	}
	// Ensure stateVersion tracks input
	out := m.output
	out.StateVersion = input.StateVersion
	return out, nil
}

func newMockCell(kind contracts.CellKind, summary string, risk contracts.RiskLevel) *mockCell {
	return &mockCell{
		kind: kind,
		output: contracts.CellOutput{
			Cell:            kind,
			Summary:         summary,
			RiskLevel:       risk,
			Confidence:      0.95,
			Recommendations: []string{"Test recommendation"},
			Evidence:        []string{"Test evidence"},
		},
	}
}

// --- Helpers ---

func makeSnapshot(version uint64) contracts.WorldState {
	return contracts.WorldState{Version: contracts.StateVersion(version), Time: 300}
}

func makeTrigger() contracts.Event {
	return contracts.Event{
		ID:         "evt-test",
		Timestamp:  300,
		Source:     "test",
		Type:       contracts.EventMainshockOccurred,
		Confidence: 1.0,
	}
}

// --- Tests ---

func TestFanOut_ConcurrentExecution(t *testing.T) {
	// Each cell sleeps 100ms. If executed concurrently, total should be ~100ms.
	// If sequential, it would be ~300ms+. We check it's under 250ms.
	infra := newMockCell(contracts.CellInfrastructure, "Bridges down", contracts.RiskHigh)
	infra.delay = 100 * time.Millisecond

	medical := newMockCell(contracts.CellMedical, "Casualty surge", contracts.RiskCritical)
	medical.delay = 100 * time.Millisecond

	population := newMockCell(contracts.CellPopulation, "People trapped", contracts.RiskHigh)
	population.delay = 100 * time.Millisecond

	commander := newMockCell(contracts.CellCommander, "COP synthesis", contracts.RiskCritical)

	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: infra,
		contracts.CellMedical:        medical,
		contracts.CellPopulation:     population,
		contracts.CellCommander:      commander,
	}

	engine := NewEngine(cells)

	wake := []contracts.CellKind{
		contracts.CellInfrastructure,
		contracts.CellMedical,
		contracts.CellPopulation,
		contracts.CellCommander,
	}

	start := time.Now()
	cop, err := engine.FanOut(context.Background(), makeSnapshot(42), makeTrigger(), wake)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("FanOut failed: %v", err)
	}

	// Verify concurrency: 3 cells * 100ms each, but concurrent so should be ~100ms
	if elapsed > 250*time.Millisecond {
		t.Errorf("FanOut took %v — cells likely ran sequentially (expected <250ms)", elapsed)
	}

	if cop.StateVersion != 42 {
		t.Errorf("expected stateVersion 42, got %d", cop.StateVersion)
	}

	if len(cop.CellOutputs) != 3 {
		t.Errorf("expected 3 specialist outputs, got %d", len(cop.CellOutputs))
	}

	// Commander should have been called with Peers
	if commander.callCount.Load() != 1 {
		t.Errorf("Commander should have been called exactly once, got %d", commander.callCount.Load())
	}
}

func TestFanOut_CommanderReceivesPeers(t *testing.T) {
	var capturedInput contracts.CellInput
	var mu sync.Mutex

	commander := &mockCell{
		kind: contracts.CellCommander,
		output: contracts.CellOutput{
			Cell:            contracts.CellCommander,
			Summary:         "Synthesized COP",
			RiskLevel:       contracts.RiskHigh,
			Confidence:      0.99,
			Recommendations: []string{"Priority action 1", "Priority action 2"},
		},
	}

	// Wrap commander to capture input
	wrappedCommander := &capturingCell{
		inner: commander,
		onAnalyze: func(input contracts.CellInput) {
			mu.Lock()
			capturedInput = input
			mu.Unlock()
		},
	}

	infra := newMockCell(contracts.CellInfrastructure, "Bridges damaged", contracts.RiskHigh)

	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: infra,
		contracts.CellCommander:      wrappedCommander,
	}

	engine := NewEngine(cells)
	wake := []contracts.CellKind{contracts.CellInfrastructure, contracts.CellCommander}

	cop, err := engine.FanOut(context.Background(), makeSnapshot(10), makeTrigger(), wake)
	if err != nil {
		t.Fatalf("FanOut failed: %v", err)
	}

	mu.Lock()
	peers := capturedInput.Peers
	mu.Unlock()

	if len(peers) != 1 {
		t.Fatalf("Commander should have received 1 peer output, got %d", len(peers))
	}

	if peers[0].Cell != contracts.CellInfrastructure {
		t.Errorf("expected peer cell Infrastructure, got %s", peers[0].Cell)
	}

	// COP should have prioritized actions from Commander recommendations
	if len(cop.PrioritizedActions) != 2 {
		t.Errorf("expected 2 prioritized actions, got %d", len(cop.PrioritizedActions))
	}

	if cop.PrioritizedActions[0].Priority != 1 {
		t.Errorf("expected first action priority 1, got %d", cop.PrioritizedActions[0].Priority)
	}
}

func TestFanOut_EmptyWakeList_NoCommander(t *testing.T) {
	engine := NewEngine(map[contracts.CellKind]contracts.Cell{})

	cop, err := engine.FanOut(context.Background(), makeSnapshot(5), makeTrigger(), nil)
	if err != nil {
		t.Fatalf("FanOut with empty wake should not error: %v", err)
	}

	if cop.StateVersion != 5 {
		t.Errorf("expected stateVersion 5, got %d", cop.StateVersion)
	}

	if cop.OverallRisk != contracts.RiskLow {
		t.Errorf("expected RiskLow for empty wake, got %s", cop.OverallRisk)
	}
}

func TestFanOut_EmptyWakeList_WithCommander(t *testing.T) {
	commander := newMockCell(contracts.CellCommander, "Global synthesized COP", contracts.RiskHigh)
	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellCommander: commander,
	}
	engine := NewEngine(cells)

	cop, err := engine.FanOut(context.Background(), makeSnapshot(12), makeTrigger(), nil)
	if err != nil {
		t.Fatalf("FanOut with empty wake and Commander failed: %v", err)
	}

	if cop.StateVersion != 12 {
		t.Errorf("expected stateVersion 12, got %d", cop.StateVersion)
	}

	if cop.OverallRisk != contracts.RiskHigh {
		t.Errorf("expected RiskHigh from Commander, got %s", cop.OverallRisk)
	}

	if cop.Summary != "Global synthesized COP" {
		t.Errorf("expected Commander summary, got %q", cop.Summary)
	}

	if commander.callCount.Load() != 1 {
		t.Errorf("expected Commander to be invoked exactly once, got %d", commander.callCount.Load())
	}
}

func TestFanOut_SingleCellFailure_StillSynthesizes(t *testing.T) {
	infra := newMockCell(contracts.CellInfrastructure, "Working", contracts.RiskMedium)
	medical := &mockCell{
		kind: contracts.CellMedical,
		err:  errors.New("LLM timeout"),
	}
	commander := newMockCell(contracts.CellCommander, "Partial COP", contracts.RiskMedium)

	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: infra,
		contracts.CellMedical:        medical,
		contracts.CellCommander:      commander,
	}

	engine := NewEngine(cells)
	wake := []contracts.CellKind{contracts.CellInfrastructure, contracts.CellMedical, contracts.CellCommander}

	cop, err := engine.FanOut(context.Background(), makeSnapshot(7), makeTrigger(), wake)
	if err != nil {
		t.Fatalf("FanOut should succeed with partial results: %v", err)
	}

	// Only infra succeeded
	if len(cop.CellOutputs) != 1 {
		t.Errorf("expected 1 cell output (infra only), got %d", len(cop.CellOutputs))
	}

	if cop.CellOutputs[0].Cell != contracts.CellInfrastructure {
		t.Errorf("expected Infrastructure output, got %s", cop.CellOutputs[0].Cell)
	}
}

func TestFanOut_AllSpecialistsFail(t *testing.T) {
	infra := &mockCell{kind: contracts.CellInfrastructure, err: errors.New("fail 1")}
	medical := &mockCell{kind: contracts.CellMedical, err: errors.New("fail 2")}
	commander := newMockCell(contracts.CellCommander, "COP", contracts.RiskLow)

	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: infra,
		contracts.CellMedical:        medical,
		contracts.CellCommander:      commander,
	}

	engine := NewEngine(cells)
	wake := []contracts.CellKind{contracts.CellInfrastructure, contracts.CellMedical, contracts.CellCommander}

	_, err := engine.FanOut(context.Background(), makeSnapshot(1), makeTrigger(), wake)
	if err == nil {
		t.Fatal("expected error when all specialists fail")
	}

	if !strings.Contains(err.Error(), "all") {
		t.Errorf("error should mention all cells failed: %v", err)
	}
}

func TestFanOut_UnregisteredCell(t *testing.T) {
	commander := newMockCell(contracts.CellCommander, "COP", contracts.RiskLow)

	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellCommander: commander,
	}

	engine := NewEngine(cells)
	// Wake a cell that isn't registered
	wake := []contracts.CellKind{contracts.CellIntelligence, contracts.CellCommander}

	_, err := engine.FanOut(context.Background(), makeSnapshot(1), makeTrigger(), wake)
	if err == nil {
		t.Fatal("expected error when only unregistered cells are woken")
	}
}

func TestFanOut_NoCommander_FallbackCOP(t *testing.T) {
	infra := newMockCell(contracts.CellInfrastructure, "Bridges damaged", contracts.RiskHigh)
	medical := newMockCell(contracts.CellMedical, "Casualty surge", contracts.RiskCritical)

	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: infra,
		contracts.CellMedical:        medical,
	}

	engine := NewEngine(cells)
	wake := []contracts.CellKind{contracts.CellInfrastructure, contracts.CellMedical}

	cop, err := engine.FanOut(context.Background(), makeSnapshot(20), makeTrigger(), wake)
	if err != nil {
		t.Fatalf("FanOut without Commander should not error: %v", err)
	}

	if cop.OverallRisk != contracts.RiskCritical {
		t.Errorf("fallback COP should take max risk (Critical), got %s", cop.OverallRisk)
	}

	if len(cop.CellOutputs) != 2 {
		t.Errorf("expected 2 cell outputs, got %d", len(cop.CellOutputs))
	}

	if cop.Summary != "Automated COP — Commander unavailable." {
		t.Errorf("unexpected fallback summary: %s", cop.Summary)
	}
}

func TestFanOut_ContextCancellation(t *testing.T) {
	// Cell that blocks until context is cancelled.
	blocking := &mockCell{
		kind:  contracts.CellInfrastructure,
		delay: 5 * time.Second, // would block for 5s without ctx cancel
	}

	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: blocking,
	}

	engine := NewEngine(cells)
	wake := []contracts.CellKind{contracts.CellInfrastructure}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := engine.FanOut(ctx, makeSnapshot(1), makeTrigger(), wake)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}

	// Should return promptly, not after 5s.
	if elapsed > 500*time.Millisecond {
		t.Errorf("FanOut took %v after ctx cancel — should have returned promptly", elapsed)
	}
}

func TestFanOut_CommanderError_SurfacedInSummary(t *testing.T) {
	infra := newMockCell(contracts.CellInfrastructure, "Working", contracts.RiskMedium)

	commander := &mockCell{
		kind: contracts.CellCommander,
		err:  errors.New("Commander LLM quota exceeded"),
	}

	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: infra,
		contracts.CellCommander:      commander,
	}

	engine := NewEngine(cells)
	wake := []contracts.CellKind{contracts.CellInfrastructure, contracts.CellCommander}

	cop, err := engine.FanOut(context.Background(), makeSnapshot(15), makeTrigger(), wake)
	if err != nil {
		t.Fatalf("FanOut should not return hard error on Commander failure: %v", err)
	}

	if !strings.Contains(cop.Summary, "Commander failed") {
		t.Errorf("Commander failure should be surfaced in COP summary, got: %s", cop.Summary)
	}

	if !strings.Contains(cop.Summary, "quota exceeded") {
		t.Errorf("Commander error message should appear in COP summary, got: %s", cop.Summary)
	}
}

func TestFanOut_StateVersionPropagated(t *testing.T) {
	infra := newMockCell(contracts.CellInfrastructure, "Status", contracts.RiskLow)
	commander := newMockCell(contracts.CellCommander, "COP", contracts.RiskLow)

	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: infra,
		contracts.CellCommander:      commander,
	}

	engine := NewEngine(cells)
	wake := []contracts.CellKind{contracts.CellInfrastructure, contracts.CellCommander}

	cop, err := engine.FanOut(context.Background(), makeSnapshot(99), makeTrigger(), wake)
	if err != nil {
		t.Fatalf("FanOut failed: %v", err)
	}

	if cop.StateVersion != 99 {
		t.Errorf("COP stateVersion should be 99, got %d", cop.StateVersion)
	}

	for _, out := range cop.CellOutputs {
		if out.StateVersion != 99 {
			t.Errorf("cell %s stateVersion should be 99, got %d", out.Cell, out.StateVersion)
		}
	}
}

// --- capturingCell wraps a cell to capture the input it receives ---

type capturingCell struct {
	inner     contracts.Cell
	onAnalyze func(contracts.CellInput)
}

func (c *capturingCell) Kind() contracts.CellKind { return c.inner.Kind() }

func (c *capturingCell) Analyze(ctx context.Context, input contracts.CellInput) (contracts.CellOutput, error) {
	if c.onAnalyze != nil {
		c.onAnalyze(input)
	}
	return c.inner.Analyze(ctx, input)
}
