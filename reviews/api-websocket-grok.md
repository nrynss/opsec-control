# Bug report — `internal/api` + `internal/websocket` (Grok Builder)

Reviewer: Claude Builder
Scope: Grok's HTTP/WS edge only.
Status at review: both build **in isolation**, `internal/api` tests pass,
gofmt clean, `go vet` clean. (`go build ./...` is red from an unrelated
in-progress lane — ignore for this review.)
Verdict: good first cut, routes are right — but **not done**: one boundary
violation (api) and one real concurrency bug (websocket) to fix before
`cmd/eoc` wiring.

Severity order. None of these are style nits except where marked 🔵.

---

## API

### 1. 🔴 Boundary violation — the API holds its own state

**Where:** `api.go:18-43` (`Server.events` + the bus-subscription goroutine in `New`).

`Server` keeps its own `events []contracts.Event` buffer and runs a goroutine to
fill it from the bus. Your own `doc.go` says **"Must NOT: hold state."** This
also duplicates `internal/timeline`, which already maintains exactly this
append-only log (`timeline.Listen(bus, tl)` + `All/Since/UpTo`). The code comment
admits it: *"simple buffering for MVD; real would use timeline."*

**Why it matters:** two parallel event logs (api's buffer + timeline) drift, and
the API is now stateful — it's supposed to *serialize* contract types and forward
events, nothing more (SPEC §12, §16.1). `/timeline` should serve the timeline,
not a private copy.

**Fix:** inject the log and delete the buffer + goroutine. Depend on a small read
interface (§0.2 r3), satisfied by `*timeline.Timeline`:

```go
// in api
type EventLog interface {
    All() []timeline.Entry
    Since(contracts.SimTime) []timeline.Entry
}

func New(store contracts.StateStore, bus contracts.EventBus,
         orch contracts.Orchestrator, log EventLog) *Server { ... }
```

Then `handleTimeline`/`handleGetEvents` read from `log`. `cmd/eoc` already needs
to construct a `timeline.Timeline` and `timeline.Listen(bus, tl)`, so just pass
it in. (Importing `internal/timeline` from `api` is fine — it's not a lane edit.)

### 2. 🟡 Incomplete vs. SPEC §12

- `GET /agents` returns a placeholder string map, not COP/cell outputs
  (`api.go:68`). There's no store of the last `CommonOperationalPicture` yet —
  the orchestrator produces it but nothing retains it. Decide where the latest
  COP lives (likely `cmd/eoc` keeps it and injects a getter) and serve it here.
- `/timeline` and `/events` return the **same** buffer — they should differ
  (timeline = ordered log/replay index; events = raw event list, or drop one).
- `/scenario/load` + `/scenario/reset` are `501` stubs. Fine for now, but they're
  in the §12 surface — wire them to the simulation/scenario lane when it's time
  (api forwards; it must not own scenario logic).

### 3. 🔵 Minor

- `ch, _ := bus.Subscribe()` (`api.go:33`) discards the cancel func → the
  goroutine can never be stopped. Tolerable for a process-lifetime singleton, but
  capture and call it on shutdown. (Moot once the buffer is removed per #1.)
- `POST /events` buffers the raw posted event before `state.Apply` validates it,
  so `/events` can show events that state will reject. Cosmetic, but worth noting.

---

## WebSocket

### 4. 🔴 Concurrent writes to one connection (data race / panic)

**Where:** `ws.go:57-62` (`serveWS` write loop) vs. `ws.go:66-75` (`Broadcast`).

gorilla/websocket **requires at most one concurrent writer per connection**
("Applications are responsible for ensuring that no more than one goroutine calls
the write methods … concurrently"). But the same `conn` is written by **two**
goroutines: its own `serveWS` bus loop *and* `Broadcast` (called from elsewhere,
e.g. the orchestrator pushing COP updates). Under the live fan-out — streaming
events while broadcasting a COP — this is a race and can panic.

**Fix:** serialize writes per connection. Cleanest is a hub with one writer
goroutine per conn fed by a buffered channel (also gives you slow-client
drop-on-full); minimal is a per-conn write mutex:

```go
type client struct {
    conn *websocket.Conn
    mu   sync.Mutex // guards all writes to conn
}
func (c *client) write(msg []byte) error {
    c.mu.Lock(); defer c.mu.Unlock()
    return c.conn.WriteMessage(websocket.TextMessage, msg)
}
```
Route both the per-conn bus loop and `Broadcast` through `client.write`.

### 5. 🟡 Connection leak + double delivery

- `serveWS` appends each conn to `s.conns` (`ws.go:48-50`) but **never removes it
  on disconnect** — only `Broadcast` prunes, and only on a write error. Dead
  conns accumulate. Remove the conn (under lock) when `serveWS` returns.
- A connection receives bus events via **both** its `serveWS` loop *and*
  `Broadcast` (if anything broadcasts bus events) → duplicate frames. Pick one
  delivery path: either every conn is driven solely by the hub/`Broadcast`, or
  each runs its own loop — not both.

### 6. 🟡 No tests

`internal/websocket` has no test files. Given #4/#5 are concurrency issues, add:
- a test that opens a conn (`httptest.Server` + `websocket.DefaultDialer`),
  publishes a bus event, and asserts the client receives it;
- a test that runs `Broadcast` concurrently with the per-conn loop under `-race`
  to lock in the fix for #4;
- a disconnect test asserting `s.conns` shrinks (locks in #5).

### 7. 🔵 `CheckOrigin` always true (`ws.go:28-30`)

Acknowledged "for demo/MVD." Fine for the recorded video; gate it before any
public exposure.

---

## Done well (keep)

- All seven §12 routes registered with clean Go 1.22 method-pattern routing.
- `GET /state` → `store.Snapshot()` and `POST /events` → `bus.Publish` are exactly
  right, with validation correctly deferred to `state.Apply` (the gatekeeper).
- Sensible gorilla usage and a clean `Handler()` seam; reasonable upgrader config.
- Good instinct keeping handlers thin.

## Priorities
1. **#4** (concurrent write) — it will bite during the live fan-out.
2. **#1** (api holds state) — fold into the timeline before `cmd/eoc` wiring.
3. **#5/#6** — leak + tests.
Everything else can follow.
