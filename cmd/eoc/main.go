// Command eoc is the EOC server: the integration root that wires every package
// together and runs the live loop (SPEC §5, §16). It owns no operational logic of
// its own — it only composes the pieces and moves data between them.
//
// Flow: simulation replays a scenario onto the EventBus → the StateManager applies
// & validates each event (§14.2) → the anomaly classifier decides which Cells wake
// → the orchestrator fans them out concurrently and the Commander synthesizes a COP
// → the COP + state snapshots are pushed to clients over WebSocket. The HTTP/WS edge
// (internal/api, internal/websocket) is the only thing the frontend talks to.
package main

import (
	"context"
	_ "embed"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nrynss/opsec-control/internal/agents"
	"github.com/nrynss/opsec-control/internal/anomaly"
	"github.com/nrynss/opsec-control/internal/api"
	"github.com/nrynss/opsec-control/internal/contracts"
	"github.com/nrynss/opsec-control/internal/events"
	"github.com/nrynss/opsec-control/internal/llm"
	"github.com/nrynss/opsec-control/internal/orchestrator"
	"github.com/nrynss/opsec-control/internal/scenario"
	"github.com/nrynss/opsec-control/internal/simulation"
	"github.com/nrynss/opsec-control/internal/state"
	"github.com/nrynss/opsec-control/internal/timeline"
	"github.com/nrynss/opsec-control/internal/websocket"
)

//go:embed scenario.json
var embeddedScenario []byte

// copStore is a concurrency-safe holder for the latest Common Operational Picture.
// It satisfies api.COPProvider: the reasoning loop writes from one goroutine while
// HTTP handlers read from others.
type copStore struct {
	mu  sync.RWMutex
	cop contracts.CommonOperationalPicture
}

func (c *copStore) Current() contracts.CommonOperationalPicture {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cop
}

func (c *copStore) set(cop contracts.CommonOperationalPicture) {
	c.mu.Lock()
	c.cop = cop
	c.mu.Unlock()
}

// app holds the wired dependencies for the reasoning loop.
type app struct {
	store      contracts.StateStore
	classifier *anomaly.Detector
	orch       contracts.Orchestrator
	cop        *copStore
	ws         *websocket.Server // may be nil in tests
}

// handle processes one event: validate+apply, push the new snapshot, and — for a
// non-ambient event that wakes at least one specialist — run the parallel fan-out
// and broadcast the resulting COP. Returns true if a fan-out occurred.
func (a *app) handle(ctx context.Context, ev contracts.Event) bool {
	if _, err := a.store.Apply(ev); err != nil {
		// Rejections are expected and harmless (§14.2) — log and skip.
		log.Printf("[eoc] event %s (%s) rejected: %v", ev.ID, ev.Type, err)
		return false
	}
	snap := a.store.Snapshot()
	a.broadcast("state", snap)

	// Ambient/noise events are volume, not signal: they update state + feed but
	// must never trigger the expensive fan-out (Cerebras budget — HANDOFF §6).
	if ev.Source == "ambient" {
		return false
	}
	wake := a.classifier.Classify(snap, ev)
	if len(wake) == 0 {
		return false
	}
	cop, err := a.orch.FanOut(ctx, snap, ev, wake)
	if err != nil {
		log.Printf("[eoc] fan-out for %s failed: %v", ev.ID, err)
		return false
	}
	a.cop.set(cop)
	a.broadcast("cop", cop)
	log.Printf("[eoc] v%d %s → woke %v → COP risk=%s (%d actions)",
		a.store.Version(), ev.Type, wake, cop.OverallRisk, len(cop.PrioritizedActions))
	return true
}

// broadcast pushes a {kind,payload} envelope to all WS clients (the shape the web
// dashboard routes on). No-op when ws is nil.
func (a *app) broadcast(kind string, payload any) {
	if a.ws == nil {
		return
	}
	a.ws.Broadcast(map[string]any{"kind": kind, "payload": payload})
}

// runLoop reasons over each event from ch until ctx is done. The caller
// subscribes (synchronously, before replay starts) so the state-applying loop
// cannot miss early events.
func (a *app) runLoop(ctx context.Context, ch <-chan contracts.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			a.handle(ctx, ev)
		}
	}
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	scenarioPath := flag.String("scenario", "", "path to a scenario JSON file (default: embedded demo)")
	speed := flag.Float64("speed", 4.0, "simulation playback speed (1.0 = real scenario seconds)")
	flag.Parse()

	raw := embeddedScenario
	if *scenarioPath != "" {
		b, err := os.ReadFile(*scenarioPath)
		if err != nil {
			log.Fatalf("[eoc] read scenario: %v", err)
		}
		raw = b
	}
	scn, err := scenario.LoadJSON(raw)
	if err != nil {
		log.Fatalf("[eoc] load scenario: %v", err)
	}
	log.Printf("[eoc] scenario %q loaded: %d events", scn.Name, len(scn.Events))

	// --- Wire dependencies (composition root; no logic lives here) ---
	bus := events.New(64)
	store := state.New(scn.Initial)
	tl := timeline.New()
	stopTL := timeline.Listen(bus, tl)
	defer stopTL()

	// Mock mode kicks in automatically when CEREBRAS_API_KEY is unset or LLM_MOCK=true.
	llmClient := llm.NewClient(llm.Config{})
	cells := map[contracts.CellKind]contracts.Cell{
		contracts.CellInfrastructure: agents.NewInfrastructure(llmClient),
		contracts.CellMedical:        agents.NewMedical(llmClient),
		contracts.CellPopulation:     agents.NewPopulation(llmClient),
		contracts.CellCommander:      agents.NewCommander(llmClient),
	}
	orch := orchestrator.NewEngine(cells)
	cop := &copStore{}
	wsSrv := websocket.New(bus)

	a := &app{store: store, classifier: anomaly.New(), orch: orch, cop: cop, ws: wsSrv}

	// --- HTTP/WS edge ---
	mux := http.NewServeMux()
	api.New(store, bus, tl, cop, llmClient).Register(mux)
	mux.Handle("/stream", wsSrv.Handler())
	httpSrv := &http.Server{Addr: *addr, Handler: mux}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("[eoc] serving on %s (GET /state /agents /timeline /events, WS /stream)", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[eoc] http: %v", err)
		}
	}()

	// Subscribe the reasoning loop SYNCHRONOUSLY before replay — Publish only
	// reaches subscribers already registered, so subscribing inside the goroutine
	// would race the simulator's first (t=0) event and drop it from state.
	eventCh, cancelSub := bus.Subscribe()
	defer cancelSub()
	go a.runLoop(ctx, eventCh)

	// Replay the scenario onto the bus.
	sim := simulation.New(bus)
	if err := sim.Load(scn); err != nil {
		log.Fatalf("[eoc] sim load: %v", err)
	}
	sim.SetSpeed(*speed)
	go func() {
		if err := sim.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[eoc] sim stopped: %v", err)
		}
		log.Printf("[eoc] scenario replay complete (server still serving; Ctrl-C to exit)")
	}()

	<-ctx.Done()
	log.Println("[eoc] shutting down")
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shCtx)
}
