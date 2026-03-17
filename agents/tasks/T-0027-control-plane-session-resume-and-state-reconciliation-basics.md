# agents/tasks/T-0027-control-plane-session-resume-and-state-reconciliation-basics.md

## Task ID

T-0027

## Title

Control-plane session resume and state reconciliation basics

## Status

Queued

## Purpose

Implement the first bounded control-plane session resume and state-reconciliation baseline for Transitloom.

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

Its job is to establish the first explicit behavior for what happens when a node’s secure control-plane session drops and reconnects.

This task should create:

- bounded session resume / reconnect behavior
- explicit re-synchronization of important node/coordinator state
- clear distinction between reconnecting transport and re-established logical state
- operator-visible session/state reconciliation status

This task should **not** become a giant distributed state-reconciliation framework.

---

## Why this task matters

Transitloom already has or is expected to have:

- more mature control-plane transport security
- service registration
- associations
- distributed path candidates
- runtime/path state that increasingly depends on coordinator communication

That means a transport reconnect alone is not enough. The system also needs to know:

- what state must be re-sent or re-fetched
- what state is still valid
- what state should be treated as stale until confirmed
- what the operator should see during recovery

Without this task, the system risks:
- reconnecting transport but leaving logical state ambiguous
- using stale coordinator-derived state too long after reconnect
- making recovery hard to explain and debug

The goal here is not “build a perfect distributed recovery engine.”  
The goal is:

**implement the first bounded resume and state-reconciliation behavior after control-plane reconnect**

---

## Objective

Add the minimum useful session-resume and state-reconciliation behavior needed so that Transitloom can reconnect control transport and explicitly recover important logical state in a bounded, understandable way.

The implementation should remain:

- explicit
- bounded
- observable
- architecture-preserving
- honest about what is and is not re-established

This task should not broaden into a full control-plane state machine redesign.

---

## Scope

This task includes:

- defining explicit post-reconnect reconciliation steps for important state such as:
  - service registration refresh
  - association refresh
  - path-candidate refresh
  - endpoint freshness or probe state revalidation if necessary
- distinguishing clearly between:
  - transport reconnected
  - secure/authenticated session re-established
  - logical runtime state reconciled
- adding bounded retry/resume behavior where appropriate
- exposing reconciliation status and recent outcomes through status/reporting
- adding focused tests for non-trivial reconnect/reconciliation behavior

This task may include:

- focused helpers under `internal/controlplane`
- focused helpers under `internal/node`
- focused helpers under `internal/coordinator`
- focused helpers under `internal/status`
- narrow additions under `internal/service` / `internal/transport` if needed
- small `tlctl` read-oriented status additions if useful

---

## Non-goals

This task does **not** include:

- a broad distributed state-management framework
- full transactional reconciliation semantics
- broad persistence/replay infrastructure
- giant leader-election or controller logic
- redesigning all control-plane messages
- automatic recovery of every possible future feature

Do not accidentally turn this into “build a distributed control-plane recovery engine.”

---

## Design constraints

This task must preserve these architectural rules:

- transport reconnect is **not** the same as logical state reconciliation
- service/association/path state remain distinct during reconciliation
- stale coordinator-derived state must not silently look current after reconnect
- resume/reconciliation status must be explicit and observable
- reconnect behavior must remain bounded and understandable

Especially important:

- do **not** imply that a live TCP/TLS session automatically means all logical state is current
- do **not** silently reuse stale distributed state forever after reconnect
- do **not** hide reconciliation phases in opaque background behavior
- do **not** overclaim convergence guarantees

---

## Expected outputs

This task should produce, at minimum:

1. Explicit reconnect/reconciliation phases or status
2. Bounded logic for refreshing important runtime state after reconnect
3. Clear status/reporting for reconciliation progress/outcome
4. Focused tests for non-trivial reconnect/reconciliation behavior
5. A more operationally realistic control-plane recovery story

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can explicitly distinguish transport reconnection from logical state reconciliation
2. important coordinator-derived state can be refreshed or revalidated after reconnect
3. reconciliation behavior is bounded and observable
4. stale distributed state is not silently treated as fully current after reconnect
5. the implementation does not claim perfect distributed recovery semantics
6. the implementation remains aligned with:
   - `spec/v1-control-plane.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
7. `go build ./...` succeeds
8. tests pass

---

## Files likely touched

Expected primary files:

- `internal/controlplane/...`
- `internal/node/...`
- `internal/coordinator/...`
- `internal/status/...`

Possibly:
- `internal/service/...`
- `internal/transport/...`
- `cmd/tlctl/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define narrow reconciliation phases/status
2. identify the minimum logical state that must be refreshed after reconnect
3. implement bounded refresh/resume steps
4. expose reconciliation status to operators
5. add focused tests
6. update task/context/memory files as needed

Keep the reconciliation layer narrow and explicit.

Do **not** prematurely add:
- giant recovery engines
- distributed transactions
- broad replay subsystems
- speculative complexity far beyond current runtime needs

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - transport reconnect and logical state reconciliation remain distinct
  - service/association/path state can be refreshed after reconnect
  - stale state is handled honestly
  - reconciliation status is inspectable and useful

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: false recovery confidence
This is the biggest risk in this task.

Do not let a reconnected transport session look like fully reconciled logical state unless it truly is.

### Risk 2: hidden reconciliation behavior
Do not hide important recovery phases in background magic.

### Risk 3: state collapse
Do not merge service, association, candidate, and path state into one vague reconciliation result.

### Risk 4: scope explosion
Do not let bounded resume behavior turn into a giant control-plane recovery redesign.

### Risk 5: poor observability
Do not make it hard for operators to understand what is still being reconciled.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower resilient-controlplane refinement task
- or a later persistence/replay task if reconciliation limits become clear
- or a runtime/operator workflow refinement informed by reconnect behavior

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a bounded, explicit, and observable control-plane reconnect/reconciliation story.

---
