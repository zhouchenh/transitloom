# agents/tasks/T-0007-association-basics.md

## Task ID

T-0007

## Title

Association basics

## Status

Completed

## Purpose

Implement the first basic association flow for Transitloom.

This task comes after:

- T-0002 — config loading scaffolding
- T-0003 — root/coordinator bootstrap scaffolding
- T-0004 — node identity and admission-token scaffolding
- T-0005 — minimal node-to-coordinator control session
- T-0006 — service registration basics

Its job is to establish the first minimal association model and coordinator-mediated association flow between registered services, while staying strictly within the current pre-v1 scope.

This task should create:

- a minimal association object shape
- a minimal association request/response path
- a coordinator-side basic association store or placeholder association state
- a clear distinction between:
  - service registration
  - association creation
  - authorization
  - later path/relay/data-plane use

This task should **not** yet become the full policy engine, discovery engine, or forwarding engine.

---

## Why this task matters

Transitloom now already has:

- config loading
- root/coordinator bootstrap scaffolding
- node identity/admission scaffolding
- a minimal bootstrap control session
- basic service registration

But it still does not have a way to represent:

- which service is intended to connect to which other service
- what an association object looks like
- how the coordinator records that relationship
- how nodes receive a minimal structured association result

Without this task, the project risks having service registration without the next core concept that the architecture depends on: **association**.

The goal here is not “finish authorization and routing.”  
The goal is:

**prove the first minimal association path without collapsing it into discovery, service registration, or forwarding behavior**

---

## Objective

Add the minimum useful implementation scaffolding for a node to request an association and for the coordinator to create/store/return a basic association record.

The association should be narrow, explicit, and honest about its current guarantees.

The implementation should use:

- the existing bootstrap control path
- the existing service registration foundation
- the existing identity/admission/bootstrap scaffolding

without jumping ahead to:

- path selection
- relay eligibility
- forwarding behavior
- full policy evaluation
- broad discovery behavior

---

## Scope

This task includes:

- defining a minimal association request/response model
- defining a minimal association object shape
- coordinator-side validation of the minimum safe association properties
- coordinator-side storage or representation of basic association records
- node-side association request behavior using the current control-session path or the smallest extension of it
- clear structured success/failure reporting on both sides
- focused tests for non-trivial association behavior

This task may include:

- focused helpers under `internal/controlplane`
- focused helpers under `internal/coordinator`
- focused helpers under `internal/service`
- focused helpers under `internal/node`
- small supporting fixtures or integration-style tests if they remain simple and reviewable

---

## Non-goals

This task does **not** include:

- service discovery for arbitrary other nodes
- path selection
- relay selection
- forwarding-state installation
- data-plane forwarding
- full policy engine
- complete production authentication/authorization semantics
- full association lifecycle engine
- broad multi-association orchestration
- making associations imply transport readiness

Do not accidentally turn this into “implement the real transport controller.”

---

## Design constraints

This task must preserve these architectural rules:

- service registration is not the same thing as association creation
- association is not the same thing as path/relay selection
- association is not the same thing as data-plane forwarding
- association creation must not imply final authorization for all future behavior
- the generic service model remains generic
- local target and local ingress must remain distinct concepts
- bootstrap-only control-session semantics must not be overstated into final control-plane guarantees

Especially important:

- an association is a **logical connectivity object**
- an association is **not yet** a data-plane path
- an association is **not yet** final proof that traffic is actively being forwarded
- the coordinator-side association record should remain intentionally narrow at this stage

Do not blur these distinctions.

---

## Expected outputs

This task should produce, at minimum:

1. A minimal association request/response model
2. A minimal coordinator-side association record or placeholder association state
3. Node-side association request behavior using the current control path
4. Coordinator-side validation and structured response for association requests
5. Clear placeholder reporting for association success/failure
6. Tests for non-trivial association behavior
7. A safe foundation for later policy/path/forwarding tasks

---

## Acceptance criteria

This task is complete when all of the following are true:

1. `transitloom-node` can attempt a minimal association request using the current control path
2. `transitloom-coordinator` can receive and evaluate that request
3. the coordinator can create/store/represent a basic association record
4. the implementation keeps association distinct from:
   - service registration
   - authorization completion
   - path selection
   - forwarding behavior
5. the implementation does not claim that association creation means traffic can already flow
6. the code remains aligned with:
   - `spec/v1-service-model.md`
   - `spec/v1-object-model.md`
   - `spec/v1-control-plane.md`
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
- `internal/service/...`
- `cmd/transitloom-coordinator/main.go`
- `cmd/transitloom-node/main.go`

Possibly:
- `internal/config/...` if a small config clarification is genuinely needed
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define a very small association request/response model
2. define a minimal association record shape on the coordinator side
3. allow the node to request association using already known service references
4. let the coordinator validate only the minimum safe association properties
5. store or represent the association in a narrow in-memory form
6. return a clear structured result
7. expose clear placeholder reporting on both sides
8. add focused tests

Keep the semantics narrow and explicit.

Do **not** prematurely add:
- discovery APIs
- path candidate selection
- relay candidate selection
- forwarding state
- rich policy engines
- broad lifecycle machinery

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- manual or integration-style checks that:
  - a coordinator starts
  - a node can request an association
  - valid association requests succeed
  - invalid association requests fail clearly
  - a coordinator-side association record is created or represented
  - structured result reporting is visible

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they stay simple and reviewable

---

## Risks to watch

### Risk 1: collapsing association into registration
This is the biggest risk in this task.

Do not let the system behave as if service registration already implies association existence.

### Risk 2: overstating association semantics
An association at this stage is not yet:
- path selection
- relay selection
- transport readiness
- active forwarding
- final policy resolution

### Risk 3: premature path/forwarding expansion
Do not let association work drag in:
- path candidates
- relay candidates
- scheduler logic
- forwarding-state installation

### Risk 4: blurring identity of service endpoints
Do not lose the distinction between:
- service identity
- service binding
- local target
- local ingress

when defining the association request or record.

---

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0008 — direct raw UDP carriage basics`

unless implementation reveals that a narrower policy or association-validation prerequisite must be split out first.

The important outcome is that Transitloom now has a real but tightly scoped association concept and coordinator-mediated association flow that future transport work can build on safely.

---
