# agents/tasks/T-0006-service-registration-basics.md

## Task ID

T-0006

## Title

Service registration basics

## Status

Completed

## Purpose

Implement the first basic service registration flow for Transitloom.

This task comes after:

- T-0002 — config loading scaffolding
- T-0003 — root/coordinator bootstrap scaffolding
- T-0004 — node identity and admission-token scaffolding
- T-0005 — minimal node-to-coordinator control session

Its job is to establish the first real service-model interaction between a node and a coordinator, while staying within the current pre-v1 scope.

This task should create:

- a minimal service registration message shape
- a minimal coordinator-side service registry state or placeholder registry state
- a node-side registration attempt after bootstrap control-session success
- clear separation between:
  - service identity
  - service binding
  - local target
  - local ingress
- a safe base for later service discovery and association work

This task should **not** yet become the full service registry system or association engine.

---

## Why this task matters

Transitloom already has:

- config loading and validation
- root/coordinator bootstrap scaffolding
- node identity/admission scaffolding
- a bootstrap-only node-to-coordinator control path

But the node still cannot tell the coordinator:

- what service exists locally
- what service type it is
- how that service is bound
- what the coordinator should record about it

Without this task, the project risks having control-plane scaffolding without the first real service-oriented interaction that the architecture depends on.

The goal here is not “finish the service model.”  
The goal is:

**prove the first minimal service registration path without collapsing the generic service model into shortcuts**

---

## Objective

Add the minimum useful implementation scaffolding for a node to register one or more local services with a coordinator and receive a clear structured result.

The registration should be narrowly scoped and honest about its current guarantees.

The implementation should use:

- the existing config model
- the existing node bootstrap readiness checks
- the existing minimal node-to-coordinator control path

without jumping ahead to associations, relay/path policy, or final discovery behavior.

---

## Scope

This task includes:

- defining the minimum service registration message shape
- defining the minimum coordinator-side service record shape or placeholder registry state
- adding node-side service registration behavior using the existing bootstrap control path or the smallest extension of it
- validating service declarations sufficiently to register them safely
- preserving explicit distinction between:
  - service
  - service binding
  - local target
  - local ingress
- adding clear placeholder reporting on both sides about registration success/failure
- adding focused tests for non-trivial registration behavior

This task may include:

- focused helpers under `internal/service`
- focused extensions under `internal/controlplane`
- focused helpers under `internal/coordinator`
- focused helpers under `internal/node`
- small supporting test fixtures

---

## Non-goals

This task does **not** include:

- service discovery for other nodes
- association creation
- path/relay selection or policy distribution
- data-plane forwarding
- live coordinator federation of service state
- complete production auth semantics
- final service lifecycle engine
- generic TCP service implementation
- making WireGuard special in the core model

Do not accidentally turn this task into “implement the whole registry/discovery layer.”

---

## Design constraints

This task must preserve these architectural rules:

- the service model remains generic
- WireGuard is the flagship use case, but not a privileged core-only type
- service is not the same thing as service binding
- service binding is not the same thing as local ingress binding
- local target is not the same thing as local ingress
- config is not distributed truth
- service registration is not association authorization
- bootstrap-only control-session semantics must not be overstated into final control-plane guarantees

Especially important:

- registering a service does **not** mean another node is authorized to use it
- registering a service does **not** mean service discovery is fully implemented
- local ingress details must not be carelessly folded into the service identity

---

## Expected outputs

This task should produce, at minimum:

1. A minimal service registration request/response shape
2. Coordinator-side handling of service registration
3. A minimal coordinator-side stored service record or placeholder registry state
4. Node-side registration attempt(s) using existing configured services
5. Clear structured success/failure reporting for registration
6. Tests for non-trivial registration behavior
7. A safe foundation for later discovery and association tasks

---

## Acceptance criteria

This task is complete when all of the following are true:

1. `transitloom-node` can attempt service registration for configured local services after the bootstrap control session path succeeds
2. `transitloom-coordinator` can receive and evaluate the registration request
3. the coordinator can store or represent a basic service record for the registered service
4. the implementation preserves the distinction between:
   - service identity
   - service binding
   - local target
   - local ingress
5. the implementation does not claim that service registration implies association authorization or service discovery completion
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

- `internal/service/...`
- `internal/controlplane/...`
- `internal/coordinator/...`
- `internal/node/...`
- `cmd/transitloom-coordinator/main.go`
- `cmd/transitloom-node/main.go`

Possibly:
- `internal/config/...` if service config mapping needs a small clarifying adjustment
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define a very small service registration request/response model
2. map configured node services into explicit registration payloads
3. let the coordinator validate only the minimum safe registration properties
4. store or represent a basic local service record on the coordinator side
5. return a clear structured result
6. expose clear placeholder reporting on both sides
7. add focused tests

Keep the registry semantics narrow and explicit.

Do **not** prematurely add:
- discovery APIs
- association creation
- rich policy engines
- broad lifecycle state machinery

---

## Completion notes

This task was completed with the following implementation shape:

- `internal/service` now maps configured node services into explicit registration declarations with separate:
  - service identity
  - service binding/local target
  - requested local-ingress intent
- `internal/controlplane` now defines a bootstrap-only service-registration request/response model with per-service results
- `internal/coordinator` now accepts service registration on the existing bootstrap listener, validates each declaration independently, and stores bootstrap-only placeholder service records in an in-memory registry
- `internal/node` now builds registration requests from configured services and submits them to the same coordinator endpoint that accepted the bootstrap control session
- `transitloom-node` now reports bootstrap-only service-registration success vs partial/failure clearly after bootstrap control-session success
- valid registrations store requested local ingress intent separately from the service binding, and registration does not allocate a local ingress binding or authorize discovery/associations
- focused tests now cover:
  - service-declaration mapping and validation
  - coordinator-side stored service records
  - partial rejection of invalid service declarations
  - node-side registration attempts

This task still intentionally stops short of:

- discovery APIs or peer-visible service lookup
- association creation or distribution
- authenticated service ownership claims
- actual `LocalIngressBinding` allocation
- any claim that registration alone authorizes use of a service

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- manual or integration-style checks that:
  - a coordinator starts
  - a node with configured services attempts registration
  - valid services register successfully
  - invalid service declarations are rejected clearly
  - structured result reporting is visible

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they stay simple and reviewable

---

## Risks to watch

### Risk 1: collapsing service and binding concepts
This is the biggest risk in this task.

Do not let registration shape or stored coordinator state blur:
- service identity
- service binding
- local target
- local ingress

### Risk 2: overstating registration semantics
A registered service is not yet:
- discovered globally
- authorized for use
- associated with another service
- part of a full registry/discovery system

### Risk 3: WireGuard special-casing
Do not let the registration model become WireGuard-specific just because WireGuard is the flagship use case.

### Risk 4: premature registry/discovery expansion
Do not add broad discovery or lifecycle semantics that are not needed for the first registration slice.

---

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0007 — association basics`

unless implementation reveals that a narrower service-discovery or stronger live-validation prerequisite must be split out first.

The important outcome is that Transitloom now has a real but tightly scoped service registration path that future discovery and association work can extend safely.

---
