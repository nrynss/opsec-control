# Seam bug — anomaly → orchestrator (Grok ↔ Antigravity collaboration)

Reviewer: Claude Builder
Severity: **High (functional)** — resource events produce no COP.
Status: whole suite is **green**; this is invisible to current tests.
Primary fix owner: **Antigravity** (orchestrator). Sign-off needed: **Grok**.

---

## TL;DR

The two lanes agreed on a convention and wrote it into *both* their comments —
*"anomaly returns specialists only; the orchestrator runs the Commander
unconditionally as phase-2."* The **contract encodes it** too
(`contracts/interfaces.go`, `Classifier` doc). Grok implemented to that
contract correctly. **Antigravity's orchestrator does not honor it**: `FanOut`
early-returns on an empty wake list and skips the Commander. Because Grok now
(correctly, per the contract) returns an **empty** wake list for resource
events, those events hit the early-return and **the Commander never runs → no
COP** — violating SPEC §7 ("Resource → all, via Commander").

It's not really a disagreement between the two builders — it's the orchestrator
**drifting from the contract it claims to implement**. Grok is in the clear;
Antigravity has a one-spot fix.

---

## Evidence

**Contract (source of truth)** — `internal/contracts/interfaces.go`:
> `Classifier` … "the orchestrator **unconditionally** invokes the Commander as
> a phase-2 synthesis step after all specialists return."

**Grok / anomaly** — `internal/anomaly/detector.go`:
- `order` slice is specialists only; Commander removed. Comment:
  *"specialists only. Commander is always run by the orchestrator as phase-2 (§6)."*
- Resource events have no case → return `[]`.
- `detector_test.go::TestClassify_NoWakeForMinor` asserts resource → `len==0`,
  comment: *"resource handled in Commander phase … Resource may legitimately
  return empty."* ← Grok is explicitly relying on the orchestrator running the
  Commander.

**Antigravity / orchestrator** — `internal/orchestrator/engine.go` `FanOut`:
```go
if len(wake) == 0 {
    return contracts.CommonOperationalPicture{
        Summary:      "No cells woken for this event.",
        StateVersion: snapshot.Version,
        OverallRisk:  contracts.RiskLow,
    }, nil   // ← Commander NOT invoked
}
```
…three lines below which the code comments: *"The Commander ALWAYS synthesises
when registered, regardless of whether it appears in the wake list."* The code
contradicts its own comment (and the contract) for the empty-wake case.

**Result for a `ResourceDeployed` event:**
`Classify → []` → `FanOut(…, [])` → early return *"No cells woken"* →
Commander never runs → no COP. Should have been a full Commander synthesis.

## Why no test caught it

Each lane tested its own half against its own assumption:
- anomaly: resource → `[]` ✔
- orchestrator: empty wake → low-risk COP, no Commander ✔

Both pass. There is **no test crossing the seam**, so the contradiction is
invisible. This is the core risk of parallel builders: green-in-isolation,
broken-at-the-join.

## The fix (Antigravity — orchestrator lane)

Remove the empty-wake early return; let the Commander run as a genuine
unconditional phase-2 even with zero specialists (it synthesizes from whole
world state + empty `Peers`). After the change, the empty-specialist path flows
through to Commander synthesis and matches the contract + both comments + §7.

Sketch:
```go
// (delete the len(wake)==0 early return)
// build specialistKinds (may be empty)
// run phase-1 fan-out over specialistKinds (no-op if empty)
// phase-2: if Commander registered, ALWAYS invoke it with whatever peers exist
//          (possibly none) and build the COP from its output.
// if no Commander registered AND no specialists -> THEN return the
//   "no cells" COP as a genuine fallback.
```
Keep the existing fallback-COP behavior for the "Commander missing/failed" case.

Land the **seam integration test** below in the same commit so the fix is
proven and the gap can't reopen.

## Sign-off needed (Grok — anomaly lane)

No code change required *if* the team confirms the convention: **Commander is an
unconditional phase-2 step, run even on empty wake.** Grok's anomaly (and its
`TestClassify_NoWakeForMinor`) bakes in that assumption. If instead the team
decides empty-wake should mean "do nothing," then anomaly must re-add a
trigger for resource events — but that contradicts the agreed specialists-only
convention and the `Classifier` contract, so the orchestrator fix is preferred.
Either way, Grok should confirm, because their lane's correctness is contingent
on the resolution.

## Prevention

§0.5 governs the interface *shape* but nothing forces a test *at the seam*. The
attached `contracttest` drives every event type through the real
`Classify → FanOut` and asserts a Commander COP. Adding seam tests to
`internal/contracts/contracttest/` for every cross-lane join would catch this
whole class of "synchronized the prose, not the behavior" bug.
