# agents/tasks/T-0012-control-plane-transport-hardening.md

## Task ID

T-0012

## Title

Control-plane transport hardening

## Status

Queued

## Purpose

Implement the first hardening pass for Transitloom control-plane transport behavior.

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

Its job is to improve the current control-plane transport from a bootstrap-oriented placeholder into a more robust, explicit, and operationally safer v1 control transport layer, without prematurely claiming that the final transport design is fully complete.

This task should create:

- clearer transport boundaries
- stronger runtime behavior around connection lifecycle and failure handling
- explicit timeout/retry/backoff behavior
- better status/reporting/observability for control transport health
- a cleaner base for later QUIC + TLS 1.3 mTLS and TCP + TLS 1.3 fallback maturation

This task should **not** become a broad rewrite of the control plane.

---

## Why this task matters

Transitloom already has:

- bootstrap control-session behavior
- service registration
- association basics
- direct data-plane behavior
- flagship direct-path validation
- relay-assisted carriage
- scheduler baseline

But the current control-plane transport likely still contains early-stage bootstrap assumptions and limited hardening.

That becomes more risky as the system gains:

- more runtime state
- more control-plane interactions
- more opportunity for partial failure
- more need for predictable error handling and observability

The goal here is not “perfect transport forever.”  
The goal is:

**make control-plane transport behavior substantially more explicit, robust, and reviewable without losing scope control**

---

## Objective

Add the minimum useful hardening for Transitloom control-plane transport so that node/coordinator interactions are more resilient, observable, and operationally safer.

The implementation should remain:

- explicit
- reviewable
- incremental
- aligned with the v1 control-plane design
- honest about what is and is not yet final

This task should prepare later transport maturation, not prematurely declare transport complete.

---

## Scope

This task includes:

- clarifying and tightening control-plane transport boundaries
- improving connection/session lifecycle handling
- improving timeout behavior
- improving retry/backoff behavior where appropriate
- improving cancellation/shutdown behavior
- improving transport-level status and observability
- improving transport error classification/reporting
- adding focused tests for non-trivial hardened behavior

This task may include:

- focused helpers under `internal/controlplane`
- focused helpers under `internal/node`
- focused helpers under `internal/coordinator`
- small supporting changes under `internal/status`
- narrow config clarifications if strictly necessary for transport behavior
- small benchmarks if a clearly hot repeated transport helper appears

---

## Non-goals

This task does **not** include:

- a full transport rewrite
- arbitrary protocol framework expansion
- generic message-bus architecture
- broad control-plane feature expansion
- service discovery redesign
- data-plane redesign
- relay redesign
- scheduler redesign
- production-perfect transport security claims beyond what is actually implemented
- replacing the staged v1 transport plan with a completely different model

Do not accidentally turn this into “re-architect the whole control plane.”

---

## Design constraints

This task must preserve these architectural rules:

- control plane remains distinct from data plane
- control-plane semantics must remain explicit
- bootstrap-only behavior must not be mislabeled as final auth semantics
- identity and admission remain separate
- transport hardening must not collapse role boundaries
- the system should remain compatible with the v1 direction of:
  - QUIC + TLS 1.3 mTLS as primary
  - TCP + TLS 1.3 fallback
- hardening must not quietly widen the protocol surface unnecessarily

Especially important:

- do not hide important failure behavior behind vague retries
- do not add retry logic that makes debugging impossible
- do not lose observability of why transport interactions failed
- do not overstate transport maturity beyond what is really implemented

---

## Expected outputs

This task should produce, at minimum:

1. Clearer control-plane transport lifecycle behavior
2. Improved timeout/retry/backoff handling where appropriate
3. Better shutdown/cancellation behavior
4. Better transport-level reporting and error visibility
5. Focused tests for non-trivial hardened transport behavior
6. A cleaner base for later transport maturation work

---

## Acceptance criteria

This task is complete when all of the following are true:

1. control-plane transport behavior is more explicit and robust than the earlier bootstrap version
2. timeout/retry/backoff behavior is defined and testable where relevant
3. shutdown/cancellation behavior is cleaner and less ambiguous
4. transport-level failures are reported clearly enough to debug
5. the implementation does not falsely claim fully mature final transport semantics
6. the implementation remains aligned with:
   - `spec/v1-control-plane.md`
   - `spec/v1-pki-admission.md`
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

Possibly:
- `internal/status/...`
- `internal/config/...` if a narrow transport-related config clarification is necessary
- `cmd/transitloom-node/main.go`
- `cmd/transitloom-coordinator/main.go`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. identify the most important brittle or under-specified transport behaviors
2. make timeout/retry/backoff rules explicit
3. make shutdown/cancellation paths explicit
4. improve transport-level reporting and error classification
5. add focused tests
6. add small benchmarks only if a clearly hot repeated helper exists
7. update task/context/memory files as needed

Keep the hardening narrow and explicit.

Do **not** prematurely add:
- broad protocol abstraction layers
- speculative complexity
- hidden retry loops
- overly magical auto-recovery behavior
- major transport redesign without clear need

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - timeout behavior is predictable
  - retry/backoff behavior is understandable and bounded
  - shutdown/cancellation behavior is clean
  - transport errors remain observable and classifiable
  - hardened behavior does not silently change control-plane semantics

If benchmarks are added, they should be run and reported.

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: hidden complexity
This is the biggest risk in this task.

Do not make transport behavior “more robust” by making it harder to reason about.

### Risk 2: retry confusion
Poorly designed retry/backoff logic can make failures harder to understand, not easier.

### Risk 3: overclaiming maturity
Do not let hardening work imply that the final control-plane transport architecture is fully complete.

### Risk 4: boundary erosion
Do not let transport hardening blur:
- control plane vs data plane
- bootstrap semantics vs final auth semantics
- node vs coordinator responsibilities

### Risk 5: unnecessary redesign
Do not use “hardening” as an excuse for a broad rewrite unless the existing structure truly cannot support the next steps.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower transport-security maturation task
- or another explicitly revealed runtime-hardening/observability task

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a significantly more robust and observable control-plane transport foundation without violating the staged v1 architecture.

---
