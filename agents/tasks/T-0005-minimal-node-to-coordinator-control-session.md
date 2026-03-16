# agents/tasks/T-0005-minimal-node-to-coordinator-control-session.md

## Task ID

T-0005

## Title

Minimal node-to-coordinator control session

## Status

Completed

## Purpose

Implement the first minimal node-to-coordinator control session for Transitloom.

This task comes after:

- T-0002 — config loading scaffolding
- T-0003 — root/coordinator bootstrap scaffolding
- T-0004 — node identity and admission-token scaffolding

Its job is to establish the first real control-plane communication path between a node and a coordinator, while staying strictly inside the current bootstrap scope.

This task should create:

- a minimal session transport and handshake shape
- explicit use of existing node identity/admission readiness scaffolding
- explicit coordinator-side bootstrap validation inputs
- a clear foundation for later authentication, service registration, and association work

This task should **not** yet become the full Transitloom control plane.

---

## Why this task matters

Transitloom now has:

- role-specific config loading
- root/coordinator bootstrap validation scaffolding
- node identity/admission bootstrap scaffolding

But it still does not have any actual control-plane interaction between node and coordinator.

Without this task, the project risks building many isolated local-state pieces without proving that the roles can actually interact in a controlled and reviewable way.

The goal here is not “finish authentication.”  
The goal is:

**prove the first minimal node-to-coordinator control path without collapsing the architecture**

This task should make later work on:

- stronger authentication
- service registration
- association distribution
- admission enforcement over live sessions

much safer.

---

## Objective

Add the minimum useful implementation scaffolding for a node to establish a minimal control session with a coordinator and receive a clear coordinator response.

The session should be bootstrap-level and tightly constrained.

The implementation should use the existing local readiness concepts from T-0003 and T-0004, while avoiding premature expansion into the full control-plane protocol.

---

## Scope

This task includes:

- defining the minimum control-session scaffolding and package boundaries
- adding a minimal transport path between:
  - `transitloom-node`
  - `transitloom-coordinator`
- adding a minimal request/response exchange sufficient to prove:
  - the node can reach the coordinator
  - the coordinator can evaluate bootstrap-level prerequisites
  - the node can receive a structured result
- using existing node identity/admission bootstrap readiness as the only allowed local readiness inputs
- adding clear placeholder reporting for session establishment success/failure
- adding tests for non-trivial session/bootstrap validation behavior where practical

This task may include:
- focused helpers under `internal/controlplane`
- focused helpers under `internal/coordinator`
- focused helpers under `internal/node`
- minimal supporting helpers under `internal/identity` or `internal/admission` if needed
- small test fixtures or test helpers

---

## Non-goals

This task does **not** include:

- full QUIC + mTLS implementation
- full TCP + TLS fallback implementation
- full certificate issuance
- full node enrollment
- real admission-token issuance
- full service registration
- associations
- data-plane behavior
- relay behavior
- distributed coordinator-state logic
- complete production authentication semantics

Do not accidentally turn this task into “implement the full control plane.”

---

## Design constraints

This task must preserve these architectural rules:

- identity and admission remain separate
- local cached admission state is not authoritative truth
- root authority is not a normal node-facing coordinator
- this task must not bypass the coordinator bootstrap checks already introduced
- this task must not treat local token presence as equivalent to live authorization
- this task must not silently invent final auth semantics
- this task must not jump ahead to service registration or association logic

Especially important:

- a minimal control session is allowed
- a fake “fully authenticated” control plane is not

The point is to create a narrow, honest bootstrap session layer.

---

## Expected outputs

This task should produce, at minimum:

1. A minimal control-session implementation path between node and coordinator
2. Explicit coordinator-side evaluation of bootstrap-level prerequisites for the incoming node session attempt
3. Explicit node-side use of local identity/admission bootstrap readiness inputs
4. Clear placeholder reporting from both sides about session success/failure/rejection
5. Tests for the non-trivial session/bootstrap logic
6. A foundation that later tasks can extend toward real authenticated control sessions

---

## Acceptance criteria

This task is complete when all of the following are true:

1. `transitloom-coordinator` can expose a minimal control-session endpoint or equivalent bootstrap session listener
2. `transitloom-node` can attempt a minimal control session to the coordinator
3. the node can receive a structured success/failure result from the coordinator
4. the session path uses existing bootstrap-level identity/admission readiness inputs rather than inventing unrelated new state
5. the implementation does not claim stronger authentication semantics than it actually provides
6. the code remains aligned with:
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
- `internal/coordinator/...`
- `internal/node/...`
- `cmd/transitloom-coordinator/main.go`
- `cmd/transitloom-node/main.go`

Possibly:
- `internal/identity/...`
- `internal/admission/...`
- `internal/config/...` if a minimal config clarification is needed
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define a very small session model and message shape
2. define the minimal bootstrap-level listener/client behavior
3. let the node send only the minimum readiness-related session input
4. let the coordinator evaluate only bootstrap-level prerequisites
5. return a clear structured result
6. expose clear placeholder reporting on both sides
7. add focused tests

Keep the protocol small and explicit.

Do **not** prematurely build:
- a large message framework
- broad request routing
- final auth claims
- service registration surfaces

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- manual or integration-style checks that:
  - coordinator starts with minimal control-session bootstrap support
  - node attempts a session
  - valid bootstrap-level conditions produce expected placeholder success
  - invalid bootstrap-level conditions produce clear rejection/failure

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they stay simple and reviewable

---

## Risks to watch

### Risk 1: overstating authentication
This is the biggest risk in this task.

Do not let a minimal bootstrap session look like a final authenticated control plane if it is not one.

### Risk 2: collapsing readiness and authorization
Node bootstrap readiness is not the same thing as fully validated live participation.

### Risk 3: premature control-plane expansion
Do not add:
- service registration
- association logic
- large control message routing
- final session/auth semantics

just because a session path now exists.

### Risk 4: weak reporting clarity
If the coordinator rejects or downgrades a bootstrap attempt, the reason should be specific and reviewable.

---

## Completion handoff

Completed implementation summary:

- `internal/controlplane` now defines a narrow bootstrap-session request and response model that carries only node-local readiness summary data and a structured bootstrap-only result.
- `internal/coordinator` now exposes a bootstrap-only HTTP JSON endpoint on the configured TCP control listener(s), evaluates coordinator bootstrap state plus the node-reported readiness phase, and returns explicit accept/reject reasons without claiming final authentication.
- `internal/node` now builds the readiness request from the existing identity/admission bootstrap inspection, retries bootstrap coordinator endpoints until one returns a structured result, and reports transport failures separately from coordinator rejections.
- `transitloom-coordinator` now starts the minimal bootstrap listener and stays running until signaled.
- `transitloom-node` now attempts the bootstrap session after local readiness inspection and exits clearly on success vs rejection/failure.
- Focused tests now cover coordinator acceptance/rejection plus node-side endpoint fallback and structured rejection handling.

When this task is complete, the next likely task should be:

- `T-0006 — service registration basics`

unless implementation reveals that a narrower authentication/issuance prerequisite must be split out first.

The important outcome is that Transitloom now has a real but tightly scoped node-to-coordinator control path that future work can extend safely.

---
