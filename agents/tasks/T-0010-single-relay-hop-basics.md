# agents/tasks/T-0010-single-relay-hop-basics.md

## Task ID

T-0010

## Title

Single relay hop basics

## Status

Completed

## Purpose

Implement the first single-relay-hop carriage path for Transitloom.

This task comes after:

- T-0002 — config loading scaffolding
- T-0003 — root/coordinator bootstrap scaffolding
- T-0004 — node identity and admission-token scaffolding
- T-0005 — minimal node-to-coordinator control session
- T-0006 — service registration basics
- T-0007 — association basics
- T-0008 — direct raw UDP carriage basics
- T-0009 — WireGuard-over-mesh direct-path validation

Its job is to establish the first relay-assisted data-plane behavior for Transitloom, while remaining strictly inside the v1 boundary:

- **single relay hop only**
- still **raw UDP**
- still **zero in-band overhead**
- still **association-bound**
- no arbitrary multi-hop forwarding
- no scheduler/multi-WAN behavior yet

This task should create the first real fallback path when direct carriage is unavailable or intentionally not used, without turning Transitloom into a generic routed mesh.

---

## Why this task matters

Transitloom’s v1 architecture explicitly allows:

- direct public paths
- direct intranet/private paths
- single coordinator relay hop
- single node relay hop

T-0008 and T-0009 proved the first direct-only path and the flagship WireGuard-over-mesh direct-path validation.

But Transitloom is not meant to be valuable only when direct paths are available.

The next important capability is:

**relay-assisted carriage with exactly one relay hop**

This matters because it proves that Transitloom can preserve its control-plane and association model while adding constrained fallback transport behavior, without collapsing into arbitrary overlay routing.

The goal here is not “finish relay support forever.”  
The goal is:

**prove one honest, narrow, single-relay-hop raw UDP path**

---

## Objective

Add the minimum useful implementation scaffolding for single-relay-hop raw UDP carriage between associated services.

The relay path should be:

- exactly one hop
- explicit
- association-bound
- narrow in scope
- honest about what is and is not implemented

The implementation should support at least one of these as the first relay role:

- coordinator relay
- or node relay

A coordinator relay is the likely simpler first slice unless implementation evidence suggests otherwise.

This task should not expand into multi-hop routing, scheduler policy, or dynamic relay optimization.

---

## Scope

This task includes:

- defining the minimum relay-assisted raw UDP carriage model
- defining the minimum relay-bound forwarding context needed for one relay hop
- implementing relay-assisted carriage for an existing association
- preserving the distinction between:
  - association
  - direct carriage
  - relay-assisted carriage
  - relay participant
  - forwarding state
- adding clear reporting/errors for valid and invalid relay-assisted carriage cases
- adding focused tests for non-trivial single-relay-hop behavior

This task may include:

- focused helpers under `internal/dataplane`
- focused helpers under `internal/node`
- focused helpers under `internal/coordinator`
- focused helpers under `internal/transport`
- small supporting changes under `internal/controlplane` if narrowly required for relay-assisted setup
- small integration-style tests if they remain simple and reviewable

---

## Non-goals

This task does **not** include:

- arbitrary multi-hop forwarding
- dynamic relay-path ranking
- scheduler refinement
- multi-WAN behavior
- weighted burst/flowlet-aware scheduling implementation
- path scoring
- relay discovery expansion
- generic encrypted data-plane behavior
- generic TCP data-plane behavior
- broad routing machinery
- full production relay hardening

Do not accidentally turn this task into “implement overlay routing.”

---

## Design constraints

This task must preserve these architectural rules:

- v1 data plane allows **single relay hop only**
- raw UDP still requires **zero in-band overhead**
- forwarding must remain **association-bound**
- relay-assisted carriage is not the same thing as arbitrary forwarding
- local target is not the same thing as local ingress
- service binding is not the same thing as local ingress binding
- relay participation must remain explicit
- relay-assisted carriage must not imply scheduler or multi-WAN behavior already exists

Especially important:

- do **not** add payload-visible shim headers to carried UDP packets
- do **not** allow arbitrary relay chains
- do **not** let relay behavior become uncontrolled free-form routing
- do **not** blur direct and relay-assisted carriage semantics
- do **not** lose the generic service model

---

## Expected outputs

This task should produce, at minimum:

1. A minimal single-relay-hop raw UDP carriage path
2. Explicit association-bound relay forwarding context
3. Clear distinction between direct and relay-assisted carriage
4. Clear reporting/errors for invalid or unavailable relay-assisted carriage cases
5. Focused tests for non-trivial single-relay-hop behavior
6. A safe base for later scheduler and multi-WAN work

---

## Acceptance criteria

This task is complete when all of the following are true:

1. a valid association can use a single relay hop for raw UDP carriage
2. relay-assisted carriage remains limited to exactly one relay hop
3. relay-assisted carriage remains association-bound
4. the implementation keeps distinct:
   - association
   - relay participant
   - direct carriage
   - relay-assisted carriage
   - local target
   - local ingress
5. the implementation does not claim scheduler, multi-WAN, or encrypted-carriage support
6. the implementation does not introduce arbitrary multi-hop forwarding
7. the implementation remains aligned with:
   - `spec/v1-data-plane.md`
   - `spec/v1-service-model.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
8. `go build ./...` succeeds
9. tests pass

---

## Files likely touched

Expected primary files:

- `internal/dataplane/...`
- `internal/node/...`
- `internal/coordinator/...`
- `internal/transport/...`
- `cmd/transitloom-node/main.go`
- possibly `cmd/transitloom-coordinator/main.go` if coordinator relay is the first slice

Possibly:
- `internal/controlplane/...` if a narrow relay-setup extension is truly needed
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. choose the narrowest honest first relay role
   - likely coordinator relay first
2. define a very small relay-assisted runtime model
3. define explicit one-hop forwarding context
4. implement relay-assisted carriage for a valid association
5. preserve clear distinction from direct carriage
6. expose clear placeholder reporting about what is and is not implemented
7. add focused tests
8. update task/context/memory files as needed

Keep the runtime model narrow and explicit.

Do **not** prematurely add:
- relay ranking
- scheduler logic
- path scoring
- relay chains
- broad routing abstractions
- generic transport plugin systems

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- manual or integration-style checks that:
  - a source node can send via a single relay hop
  - the relay forwards only in valid association context
  - the destination receives and delivers to the correct local target
  - invalid or missing relay/association context prevents carriage
  - the implementation does not accidentally allow a second relay hop

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they stay simple and reviewable

---

## Risks to watch

### Risk 1: accidental multi-hop drift
This is the biggest risk in this task.

Do not let “single relay hop” quietly become “general routed overlay.”

### Risk 2: violating zero in-band overhead
Do not add payload-visible shim headers to carried raw UDP packets.

### Risk 3: collapsing relay and forwarding freedom
Relay-assisted carriage must remain association-bound and explicitly constrained.

### Risk 4: premature scheduler/optimization expansion
Do not let this task drift into:
- scheduler logic
- multi-WAN behavior
- dynamic path selection
- relay scoring

### Risk 5: unclear relay role boundaries
Be explicit about whether the first relay role is:
- coordinator relay
- node relay

Do not blur those if only one is actually implemented first.

---

## Implementation notes

Coordinator relay was chosen as the first relay role (simpler first slice).

### Files created or modified

- `internal/dataplane/relay.go` (new): `RelayForwardingEntry`, `RelayForwardingTable`, `RelayCarrier` (coordinator); `RelayEgressEntry`, `RelayEgressTable`, `RelayEgressCarrier` (source node); builders `BuildRelayForwardingEntry`, `BuildRelayEgressEntry`; status reporting `ReportRelayCarriageStatus`
- `internal/dataplane/relay_test.go` (new): 17 tests including flagship end-to-end single-hop carriage test, structural single-hop constraint enforcement test, and direct-vs-relay distinction test
- `internal/node/relay_path.go` (new): `RelayPathRuntime`, `RelayEgressActivation`, `RelayPathResult`, `BuildRelayEgressActivationInputs`, `ActivateRelayEgressPaths`
- `internal/coordinator/relay.go` (new): `CoordinatorRelayRuntime`, `RelayActivation`, `RelayActivationResult`, `ActivateRelayForAssociation`
- `internal/config/common.go` (modified): added `RelayEndpoint` field to `AssociationConfig`
- `internal/dataplane/forwarding.go` (modified): updated DirectOnly error message to mention relay types; updated package doc to mention relay-assisted carriage scope

### Key architectural decisions made during implementation

- **Per-association relay listen port for zero overhead**: Coordinator allocates a distinct UDP port per association for relaying; the association is identified by which port received the packet, not by in-band content. This is the only option that preserves zero in-band overhead at the relay.
- **Destination delivery unchanged**: `DirectCarrier.StartDelivery` is reused for relay-assisted delivery at the destination side. Delivery behavior is identical regardless of whether packets arrived via direct or relay path. No separate "relay delivery" type was created.
- **Single-hop structurally enforced**: `RelayForwardingEntry` has only a `DestMeshAddr` (terminal endpoint) with no next-relay or chain field. Relay chains cannot be constructed; the constraint is structural, not a runtime check.
- **Separate types for direct vs relay**: `ForwardingEntry` (direct), `RelayForwardingEntry` (coordinator relay), and `RelayEgressEntry` (source egress) are distinct incompatible types. No architectural conflation is possible.
- **Relay and egress carriers are independent**: `RelayCarrier` and `RelayEgressCarrier` are separate runtime objects with separate handle maps. Each manages its own per-association goroutines.

### Verification

- `go build ./...`: success
- `go test ./...`: all pass (17 new relay tests + all existing tests)

---

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0011 — scheduler baseline and multi-WAN refinement`

unless implementation reveals that a narrower relay-hardening or ingress/runtime prerequisite should be split out first.

The important outcome is that Transitloom now has a real, constrained, single-relay-hop raw UDP carriage path that remains honest to the v1 architecture.

---
