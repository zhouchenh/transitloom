# agents/tasks/T-0030-live-node-probe-loop-lifecycle-integration-basics.md

## Task ID

T-0030

## Title

Live node probe-loop lifecycle integration basics

## Status

Queued

## Purpose

Implement the first bounded live node lifecycle integration for Transitloom’s active probe scheduling loop.

This task comes after:

- T-0002 — config loading scaffolding
- T-0003 — root/coordinator bootstrap scaffolding
- T-0004 — node identity and admission-token scaffolding
- T-0005 — minimal node-to-coordinator control session
- T-0006 — service registration basics
- T-0007 — association basics
- T-0008 — direct raw UDP carriage basics
- T-0009 — WireGuard-over-mesh direct-path validation
- T-0010 — single relay hop basics
- T-0011 — scheduler baseline and multi-WAN refinement
- T-0012 — control-plane transport hardening
- T-0013 — runtime observability and debugging basics
- T-0014 — scheduler-to-carrier integration
- T-0015 — external endpoint advertisement and DNAT-aware reachability basics
- T-0016 — tlctl runtime inspection and operator workflows basics
- T-0017 — targeted external endpoint probing and freshness revalidation basics
- T-0018 — path candidate distribution and consumption basics
- T-0019 — live path quality measurement basics
- T-0020 — quality-aware path selection refinement
- T-0021 — control-plane transport security maturation
- T-0022 — candidate refresh and revalidation automation basics
- T-0023 — direct-relay fallback and recovery basics
- T-0024 — multi-WAN policy and hysteresis basics
- T-0025 — operator path diagnostics and explainability basics
- T-0026 — path change event history and audit basics
- T-0027 — control-plane session resume and state reconciliation basics
- T-0028 — config profile and policy bundling basics
- T-0029 — active probe scheduling and path usability signal wiring basics

Its job is to take the bounded active probe loop introduced by T-0029 and integrate it into the actual node runtime lifecycle so that probe scheduling becomes operational behavior, not just a callable helper.

This task should create:

- startup wiring for the active probe loop
- bounded stop/shutdown behavior for probe scheduling
- explicit operator-visible probe-loop runtime state
- a practical live-node path-usability update path

This task should **not** become a broad runtime supervisor, giant background job framework, or full measurement platform redesign.

---

## Why this task matters

Transitloom already has, or is expected to have:

- targeted probe primitives
- endpoint freshness stores
- path-quality stores
- candidate refresh/revalidation behavior
- direct/relay fallback and recovery behavior
- bounded active probe loop primitives from T-0029

But an important gap remains:

- the probe loop exists
- the signal wiring exists
- the tests exist

yet the loop is not fully integrated into the node’s live startup/runtime lifecycle.

Without this task:
- T-0029 remains structurally complete but operationally incomplete
- runtime freshness and quality can still lag unless some caller explicitly starts the loop
- fallback/recovery behavior remains less truthful than it could be in live operation

The goal here is not “build a general background-job system.”  
The goal is:

**make the active probe loop a real, bounded part of the node runtime lifecycle**

---

## Objective

Add the minimum useful live lifecycle integration needed so that Transitloom nodes start, run, report, and stop the active probe loop in a bounded, explicit, observable way.

The implementation should remain:

- bounded
- targeted-first
- explicit
- operationally useful
- compatible with the current staged runtime architecture

This task should not become a broad redesign of node startup, background workers, or scheduling systems.

---

## Scope

This task includes:

- wiring the active probe loop into node startup/runtime lifecycle
- ensuring the loop starts only when its prerequisites are available
- ensuring the loop stops cleanly on node shutdown/cancellation
- exposing probe-loop runtime state and/or last-round status through existing status/observability surfaces
- keeping the loop bounded by existing cadence/max-target limits
- adding focused tests for non-trivial lifecycle integration behavior

This task may include:

- focused helpers under `internal/node`
- focused helpers under `internal/status`
- narrow changes under `cmd/transitloom-node/main.go`
- small `tlctl` or status additions if useful
- narrow config clarifications only if strictly necessary

---

## Non-goals

This task does **not** include:

- redesigning T-0029’s probe semantics
- broad scheduler redesign
- broad fallback-policy redesign
- giant background-job framework additions
- full active/passive measurement orchestration
- broad probe-adaptive intelligence
- uncontrolled concurrent probe fans

Do not accidentally turn this into “build a worker framework.”

---

## Design constraints

This task must preserve these architectural rules:

- probing remains **targeted-first**
- probe scheduling remains distinct from hysteresis/switch policy
- endpoint freshness remains distinct from measured path quality
- measured path quality remains distinct from chosen runtime path
- runtime lifecycle integration must remain bounded and inspectable
- probe-loop status must not silently disappear into background behavior

Especially important:

- do **not** let the live loop run uncontrolled
- do **not** make startup ordering ambiguous
- do **not** make shutdown behavior sloppy or leaky
- do **not** hide whether the probe loop is active, disabled, or blocked by prerequisites

---

## Expected outputs

This task should produce, at minimum:

1. A live node startup path for the bounded active probe loop
2. Clean loop shutdown/cancellation behavior
3. Operator-visible probe-loop runtime state or last-round status
4. Focused tests for non-trivial lifecycle integration behavior
5. A more operationally complete implementation of T-0029

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom nodes can start the bounded active probe loop as part of live runtime operation
2. the loop stops cleanly on shutdown/cancellation
3. probe-loop runtime state is inspectable enough for operators/debugging
4. the integration does not redesign probing into a broad scheduling framework
5. the implementation does not claim broad measurement/platform semantics beyond what is actually implemented
6. the implementation remains aligned with:
   - `spec/v1-data-plane.md`
   - `spec/v1-service-model.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
7. `go build ./...` succeeds
8. tests pass

---

## Files likely touched

Expected primary files:

- `internal/node/...`
- `cmd/transitloom-node/main.go`
- `internal/status/...`

Possibly:
- `cmd/tlctl/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. identify the narrowest startup point where the probe loop should begin
2. ensure required dependencies/stores are available first
3. wire bounded loop start/stop to node lifecycle context
4. expose minimal but useful runtime status
5. add focused tests
6. update task/context/memory files as needed

Keep the integration narrow and explicit.

Do **not** prematurely add:
- broad worker registries
- generic lifecycle frameworks
- hidden probe supervisors
- complex concurrency beyond what the bounded loop already needs

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - the probe loop starts in live node runtime when expected
  - the probe loop stops cleanly on shutdown/cancel
  - runtime state or last-round visibility is operator-usable
  - bounded cadence/max-target behavior remains intact
  - the integration remains distinct from policy/hysteresis logic

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: hidden background behavior
This is the biggest risk in this task.

Do not let the probe loop become a background mechanism that operators cannot see or reason about.

### Risk 2: sloppy lifecycle handling
Do not leak goroutines or make shutdown behavior ambiguous.

### Risk 3: startup-order confusion
Do not start probing before the required runtime state exists.

### Risk 4: boundary erosion
Do not let lifecycle integration accidentally absorb policy or scheduling responsibilities.

### Risk 5: weak observability
Do not make it hard to tell whether probing is active, disabled, blocked, or stale.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a focused runtime-policy-consumption task for T-0028 effective policy fields
- or a later operator/runtime refinement task building on live probe status
- or a later control-plane realism task if the path/runtime lane is sufficiently integrated

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now runs the bounded active probe loop as a real node lifecycle component rather than only as a library/helper capability.

---
