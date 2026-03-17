# agents/tasks/T-0008-direct-raw-udp-carriage-basics.md

## Task ID

T-0008

## Title

Direct raw UDP carriage basics

## Status

Completed

## Purpose

Implement the first direct raw UDP carriage path for Transitloom.

This task comes after:

- T-0002 — config loading scaffolding
- T-0003 — root/coordinator bootstrap scaffolding
- T-0004 — node identity and admission-token scaffolding
- T-0005 — minimal node-to-coordinator control session
- T-0006 — service registration basics
- T-0007 — association basics

Its job is to establish the first real data-plane behavior for Transitloom:

- direct raw UDP only
- association-bound forwarding only
- clear local ingress send path
- clear local target delivery path
- no relay yet
- no generic encrypted transport yet
- no arbitrary multi-hop behavior

This task should create the first real end-to-end UDP service carriage slice without violating the v1 architecture.

---

## Why this task matters

Transitloom now already has:

- config loading and validation
- root/coordinator bootstrap scaffolding
- node identity/admission scaffolding
- a minimal bootstrap control path
- service registration
- association basics

But it still does not carry actual service traffic.

Without this task, Transitloom remains a control-plane-only skeleton.

The goal here is not “finish the data plane.”  
The goal is:

**prove the first minimal direct raw UDP carriage path for an existing association**

This is the first task that turns the project into a real overlay transport system rather than only a coordination framework.

---

## Objective

Add the minimum useful implementation scaffolding for direct raw UDP carriage between associated services.

The carriage should be:

- direct only
- association-bound
- explicit
- narrow in scope
- honest about what is not yet implemented

The implementation should use:

- the existing service registration foundation
- the existing association foundation
- existing service binding and local target concepts

without jumping ahead to:

- relays
- scheduler logic
- multi-WAN logic
- path scoring
- generic encrypted carriage
- generic TCP carriage

---

## Scope

This task includes:

- defining the minimum direct raw UDP carriage model and runtime flow
- defining or implementing the minimum association-bound forwarding context needed for direct carriage
- adding a local ingress receive/send path on the source side
- adding local target delivery on the destination side
- preserving the distinction between:
  - service identity
  - service binding
  - local target
  - local ingress
  - association
- adding clear structured reporting and errors for success/failure cases
- adding focused tests for non-trivial carriage behavior

This task may include:

- focused helpers under `internal/dataplane`
- focused helpers under `internal/service`
- focused helpers under `internal/node`
- focused helpers under `internal/transport`
- small supporting changes under `internal/controlplane` or `internal/coordinator` only if necessary for association-bound direct carriage setup
- simple integration-style tests if they remain small and reviewable

---

## Non-goals

This task does **not** include:

- relay behavior
- multi-hop forwarding
- scheduler refinement
- weighted burst/flowlet-aware scheduling implementation
- multi-WAN optimization
- relay candidate/path candidate logic beyond what is minimally necessary to keep the architecture honest
- generic encrypted data-plane behavior
- generic TCP data-plane behavior
- service discovery expansion
- full production transport hardening

Do not accidentally turn this task into “implement the whole data plane.”

---

## Design constraints

This task must preserve these architectural rules:

- raw UDP is the primary v1 data-plane transport
- raw UDP v1 requires **zero in-band overhead**
- this task is **direct only**
- association is not the same thing as forwarding state, but forwarding must remain association-bound
- local target is not the same thing as local ingress
- service binding is not the same thing as local ingress binding
- direct raw UDP carriage must not imply relay support is already implemented
- control-plane bootstrap semantics must not be overstated into full transport control semantics
- the generic service model must remain generic

Especially important:

- do not add a payload shim/header to carried UDP packets
- do not introduce arbitrary multi-hop behavior
- do not let carriage happen outside valid association context
- do not blur “configured association” into “unrestricted forwarding”

---

## Expected outputs

This task should produce, at minimum:

1. A minimal direct raw UDP carriage path between associated services
2. Source-side local ingress handling that can accept UDP from the local application/service side
3. Destination-side local target delivery to the registered service binding
4. Association-bound validation/context for direct carriage
5. Clear reporting or errors for invalid/unavailable direct-carriage cases
6. Focused tests for non-trivial direct-carriage behavior
7. A safe base for later relay and scheduler work

---

## Acceptance criteria

This task is complete when all of the following are true:

1. a node can send raw UDP into Transitloom for an existing association using the local ingress/send path
2. a peer node can receive and deliver that UDP to the correct local target for the associated service
3. carriage is limited to valid association context
4. the implementation keeps distinct:
   - service identity
   - service binding
   - local target
   - local ingress
   - association
5. the implementation does not claim relay, scheduler, multi-WAN, or encrypted-carriage support
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

- `internal/dataplane/...`
- `internal/node/...`
- `internal/service/...`
- `internal/transport/...`
- `cmd/transitloom-node/main.go`

Possibly:
- `internal/controlplane/...` if a small extension is truly needed for direct association-bound carriage setup
- `internal/coordinator/...` if a narrow supporting hook is necessary
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define a very small direct-carriage runtime model
2. define the association-bound lookup/context needed for direct forwarding
3. implement source-side local ingress receive/send behavior
4. implement destination-side local target delivery behavior
5. keep carriage direct and explicit
6. expose clear placeholder reporting about what is implemented vs not implemented
7. add focused tests
8. update task/context/memory files as needed

Keep the runtime model narrow and explicit.

Do **not** prematurely add:
- relay abstraction
- generic path selection
- scheduler framework
- transport plugins
- payload encapsulation headers

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- manual or integration-style checks that:
  - a source node starts
  - a destination node starts
  - direct UDP carriage works for a valid association
  - invalid or missing association context prevents carriage
  - delivery goes to the correct registered local target

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they stay simple and reviewable

---

## Risks to watch

### Risk 1: violating zero in-band overhead
This is the biggest risk in this task.

Do not add payload-visible shim headers for carried raw UDP traffic.

### Risk 2: collapsing association into forwarding freedom
Direct carriage must remain association-bound, not become unrestricted forwarding.

### Risk 3: collapsing local target and local ingress
The first real carriage task makes this temptation stronger. Resist it.

### Risk 4: premature relay/scheduler expansion
Do not let this task drift into:
- relay support
- scheduler work
- multi-WAN logic
- path scoring

### Risk 5: over-abstracting transport
Do not build a large generic transport framework before proving the first direct UDP slice.

---

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0009 — WireGuard-over-mesh direct-path validation`

unless implementation reveals that a narrower direct-carriage-hardening or local-ingress-allocation prerequisite should be split out first.

The important outcome is that Transitloom now has a real, direct, association-bound raw UDP carriage path that later WireGuard validation and relay/scheduler work can build on safely.

---
