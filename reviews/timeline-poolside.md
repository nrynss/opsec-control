# Review note — `internal/timeline` (Poolside Laguna M)

Reviewer: Claude Builder · Verdict: **ship-worthy after the Payload fix**
Status at review: builds, `go vet` clean, `gofmt` clean, 16 tests pass (run 2×).

This is a clean, well-tested package. The items below are ordered by severity.
Nothing here blocks others from integrating against it today; #1 is the one to
fix before the log is trusted as the replay substrate.

---

## 1. 🟡 Immutability is shallow — `Event.Payload` aliases the caller's bytes

**Where:** `timeline.go` — `Append`, and by extension `All`/`Last`/`Since`/`UpTo`.

**What:** `contracts.Event.Payload` is `json.RawMessage`, i.e. `[]byte`. `Append`
stores the event *by value*, which copies the slice header but **shares the
underlying byte array** with the caller. So after:

```go
ev := contracts.Event{ID: "e1", Payload: json.RawMessage(`{"bridgeId":"BR-12"}`)}
tl.Append(ev)
ev.Payload[2] = 'X'   // caller still holds the same backing array
```

…the logged entry's payload is now corrupted. The same aliasing applies on the
read side: `All()` copies the `[]Entry` slice, but each returned `Entry.Event.Payload`
still points at the timeline's stored backing array, so a consumer can mutate it
in place.

**Why it matters:** both the struct comment and `doc.go` promise an *immutable,
append-only* log, and this package is the substrate for **deterministic replay**
(§11, §16.1). A mutated payload replays differently → determinism violation
(§0.2 r5). Low likelihood in practice, but it's the gap between "documented
immutable" and "actually immutable."

**Fix (producer side — minimal, recommended):** clone the payload on append.

```go
func (t *Timeline) Append(ev contracts.Event) {
	ev.Payload = slices.Clone(ev.Payload) // nil-safe; returns json.RawMessage
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, Entry{Event: ev})
}
```

(`import "slices"`. `slices.Clone(nil)` returns `nil`, so no special-casing.)

**Decide on the consumer side too.** Clone-on-append stops the *producer* from
mutating after the fact, but consumers of `All`/`Last`/`Since`/`UpTo` still share
the stored array. Pick one:
- **(a)** Document that returned `Payload` is read-only (cheapest), or
- **(b)** Clone on read as well for full safety (more allocations).

For a replay log I'd do **(a)** + clone-on-append: producer can't corrupt the
record, and the consumer contract is explicit. Worth a one-line note in `doc.go`.

---

## 2. 🔵 Stale comment — `Entry` references a field that doesn't exist

**Where:** `timeline.go:10-11`.

```go
// Entry is an immutable log entry containing an event.
// The wall timestamp is for debugging/audit purposes only; logical ordering
// is determined by the event's Timestamp (SimTime) field, never by wall time.
type Entry struct {
	Event contracts.Event
}
```

`Entry` has **no wall-timestamp field** — only `Event`. The second sentence
describes a design that isn't here (leftover from an earlier draft). Either drop
the sentence or, if a wall-clock audit stamp is actually wanted, add the field
explicitly — but note it must stay out of any ordering/replay logic to preserve
determinism (§0.2 r5).

---

## 3. 🔵 `Last()` comment contradicts the (correct) behavior

**Where:** `timeline.go:54-64`.

The comment says the returned pointer "refers to internal storage; callers
should not modify it." The implementation actually returns a pointer to a
*copy*:

```go
e := t.entries[len(t.entries)-1].Event
return &e
```

That's the safe, correct thing to do — but the comment claims the opposite and
understates the guarantee. (Note: per #1, the copy's `Payload` still aliases
until clone-on-append lands.) Fix the comment to match.

---

## 4. 🔵 `Replay` silently swallows non-rejection errors

**Where:** `listener.go:30-40`.

```go
ver, err := store.Apply(ev)
if err != nil {
	var re *contracts.RejectionError
	if re, _ = err.(*contracts.RejectionError); re != nil {
		rejected = append(rejected, ev)
	}
} else {
	finalVer = ver
}
```

If `Apply` ever returns an error that is **not** a `*RejectionError`, it is
neither recorded in `rejected` nor surfaced, and `finalVer` isn't advanced — the
failure vanishes. Today `StateStore.Apply` only returns `*RejectionError`, so
this is latent, not live. Harden it with an `else` branch that captures the
unexpected error (return it, or collect into a separate slice) so a future
change can't hide a replay failure.

Also fine as-is but worth noting: `Replay` correctly *continues* past rejections
(they're expected and harmless per §14.2) rather than aborting. Good.

---

## 5. 🔵 Scope/placement question — `Replay` drives `StateStore`

**Where:** `listener.go` — `Replay(tl, store, upTo)`.

The §16.1 ownership table lists timeline's dependency as `events` and its job as
"immutable event log / replay **index**." `Replay` reaches through
`StateStore.Apply` to actually re-drive state — that's orchestration that
arguably belongs in `cmd/eoc` or `internal/simulation`. It's interface-clean
(uses only the `contracts` interface, so not a lane *violation*), but flag it for
a quick coordination check: should `Replay` live here, or should timeline just
expose the ordered slice and let the caller drive `Apply`?

---

## Things done well (keep doing)

- **Concurrency is correct:** `RWMutex` with write-lock on `Append`, read-lock on
  queries; `All()` returns a copied slice; there's a concurrent reader/writer
  test. (Race detector couldn't run locally — no cgo/gcc on Windows — so rely on
  Linux CI's `-race` job.)
- **Interface discipline (§0.2 r3):** production code imports only `contracts`;
  `Listen`/`Replay` depend on `EventBus`/`StateStore` interfaces, not impls.
- **Clean shutdown:** `Listen`'s goroutine ranges until cancel closes the
  channel; covered by `TestListenCancelStopsGoroutine`. No leak.
- **Order-independent queries:** `Since`/`UpTo` scan all entries with no early
  break, so they stay correct regardless of append order.
- Good test coverage of the happy paths, copy-isolation, ordering, and replay
  cutoff/rejection handling.

## Suggested test additions

- Payload-aliasing regression: append an event, mutate the caller's
  `Payload`, assert the stored entry is unchanged (will fail until #1 lands).
- `Replay` with a non-`RejectionError` from `Apply` (after #4) to lock in the
  new behavior.
