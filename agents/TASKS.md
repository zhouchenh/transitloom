# agents/TASKS.md

## Purpose

This file is the compact task index for Transitloom.

It is intentionally short. Detailed task definitions, progress notes, and acceptance criteria should live under:

- `agents/tasks/`

Use this file to answer, quickly:

- what is active now
- what is next
- what is blocked
- what was recently completed

If a task needs more than a short summary, it belongs in its own task file.

---

## Current phase

**implementation bootstrap**

Transitloom currently has:
- architecture/spec baseline
- docs baseline
- object model
- config model
- implementation plan
- initial Go module and code skeleton
- partially built `agents/` workspace
- role-specific config loading and validation scaffolding
- root/coordinator trust-bootstrap inspection scaffolding
- node identity and admission bootstrap inspection scaffolding

Transitloom does **not** yet have meaningful implementation of:
- node enrollment
- node certificate issuance
- admission-token issuance or refresh
- coordinator-side admission-token validation
- service discovery
- scheduler-to-carrier integration (Decide() results not yet wired into carriers)
- live path quality measurement (RTT/jitter/loss from real traffic)
- multi-path carrier load balancing at the socket level
- coordinator-distributed path candidates

---

## Active task

No task is currently active.

The next task to pick up is **T-0013 — scheduler-to-carrier integration** (wiring
`Scheduler.Decide()` results into `DirectCarrier` and `RelayEgressCarrier`) or a
transport-security maturation task (QUIC+TLS 1.3 mTLS, TCP+TLS 1.3 fallback).

---

## Recently completed

### T-0012 — control-plane transport hardening
**status:** completed
**task file:** `agents/tasks/T-0012-control-plane-transport-hardening.md`

Added `internal/controlplane/transport.go` with named timeout/retry/body-limit
constants, `internal/controlplane/errors.go` with `TransportError`/`TransportErrorKind`/
`ClassifyTransportError`. Updated coordinator bootstrap listener with full HTTP server
timeouts and `http.MaxBytesReader` body limiting. Updated node bootstrap session
with bounded retry/backoff for timeout errors and immediate skip for non-retryable
errors (connection-refused, context-canceled). Added `ErrorKind` field to
`BootstrapEndpointAttempt`. Added 12 focused tests.
`go build ./...` and `go test ./...` both pass.

### T-0011 — scheduler baseline and multi-WAN refinement
**status:** completed
**task file:** `agents/tasks/T-0011-scheduler-baseline-and-multi-wan-refinement.md`

Implemented the first endpoint-owned scheduler baseline. Defined `PathCandidate`,
`RelayCandidate`, `PathQuality`, `PathClass`, `HealthState` (distinct from
ForwardingEntry), `SchedulerDecision`, `Mode`, `ChosenPath`. Implemented
`Scheduler.Decide()`: filters by association ID + health, scores by AdminWeight +
relay penalty + quality, selects ModeWeightedBurstFlowlet as default,
ModePerPacketStripe only when all paths are within `StripeMatchThresholds` (RTT,
jitter, loss spread, confidence). Added observable `AssociationCounters` and
`SchedulerStatus`. Added 25 tests and 2 benchmarks (~709 ns/op for 2 candidates).
`go build ./...` and `go test ./...` both pass.

### T-0010 — single relay hop basics
**status:** completed
**task file:** `agents/tasks/T-0010-single-relay-hop-basics.md`

Implemented the first single-relay-hop raw UDP carriage path. Defined
`RelayForwardingEntry`/`RelayForwardingTable` and `RelayCarrier` for the
coordinator relay role, `RelayEgressEntry`/`RelayEgressTable` and
`RelayEgressCarrier` for the source node relay egress role. Added
`CoordinatorRelayRuntime` and `RelayPathRuntime` integration types.
Destination-side delivery reuses existing `DirectCarrier.StartDelivery`
unchanged. Added `RelayEndpoint` to `AssociationConfig`. Added 17 focused tests
including a flagship end-to-end single-hop carriage test (local app → relay
egress → coordinator relay → mesh delivery → local target) and structural
enforcement of the single-hop constraint. `go build ./...` and `go test ./...`
both pass.

### T-0009 — WireGuard-over-mesh direct-path validation
**status:** completed
**task file:** `agents/tasks/T-0009-wireguard-over-mesh-direct-path-validation.md`

Implemented WireGuard-over-mesh direct-path validation. Wired direct raw UDP
carriage primitives into node runtime via `DirectPathRuntime`, made Transitloom
local ingress endpoints usable as WireGuard peer endpoints, validated end-to-end
direct-path delivery including zero in-band overhead, and preserved all
service-model/data-plane architecture boundaries. Added `MeshListenPort` to
association config for per-association inbound delivery. Added 10 focused tests
including a flagship end-to-end WireGuard-over-mesh validation, bidirectional
traffic, and multiple-packet delivery.

### T-0002 — config loading scaffolding
**status:** completed  
**task file:** `agents/tasks/T-0002-config-loading-scaffolding.md`

Implemented strict YAML config loading and validation scaffolding for root, coordinator, and node roles, plus `-config` startup wiring and tests.

### T-0003 — root/coordinator bootstrap scaffolding
**status:** completed  
**task file:** `agents/tasks/T-0003-root-coordinator-bootstrap.md`

Implemented explicit root/coordinator trust-bootstrap inspection, trust-material presence checks, role-specific startup reporting, and tests for valid and invalid bootstrap states.

### T-0004 — node identity and admission-token scaffolding
**status:** completed
**task file:** `agents/tasks/T-0004-node-identity-and-admission-token-scaffolding.md`

Implemented explicit node-identity and cached-admission-token bootstrap inspection, distinct persisted-state config sections, `transitloom-node` readiness reporting, and tests for valid and invalid local state combinations.

### T-0005 — minimal node-to-coordinator control session
**status:** completed
**task file:** `agents/tasks/T-0005-minimal-node-to-coordinator-control-session.md`

Implemented a bootstrap-only node-to-coordinator control-session exchange over
the coordinator TCP listener, with explicit readiness snapshots, structured
accept/reject results, clear placeholder reporting, and focused listener/client
tests.

### T-0006 — service registration basics
**status:** completed
**task file:** `agents/tasks/T-0006-service-registration-basics.md`

Implemented bootstrap-only service registration over the existing coordinator
TCP listener, with explicit service declaration mapping, a placeholder
coordinator-side in-memory registry, per-service accept/reject results, clear
separation between service binding/local target and requested local ingress
intent, and focused node/coordinator/service tests.

### T-0007 — association basics
**status:** completed
**task file:** `agents/tasks/T-0007-association-basics.md`

Implemented bootstrap-only association creation over the existing coordinator
TCP listener. Nodes can request associations between registered services;
the coordinator validates service existence, rejects self-associations and
duplicates, stores narrow placeholder association records, and returns
structured per-association results. Association is kept strictly distinct from
service registration, path selection, relay eligibility, and forwarding state.

### T-0008 — direct raw UDP carriage basics
**status:** completed
**task file:** `agents/tasks/T-0008-direct-raw-udp-carriage-basics.md`

Implemented the first direct raw UDP carriage path. `internal/dataplane` now
contains a ForwardingTable for association-bound forwarding lookup, a
DirectCarrier that manages local ingress listeners (source side) and local
target delivery (destination side), and a builder that bridges control-plane
association records to data-plane forwarding entries. Carriage preserves zero
in-band overhead, stays association-bound, keeps local ingress and local
target separate, and is explicitly direct-only with no relay, scheduler,
multi-WAN, or encrypted carriage support.

---

## Queued tasks

The next implementation tasks are:
- `T-0013 — scheduler-to-carrier integration` (wiring `Decide()` into carriers)
- transport-security maturation (QUIC+TLS 1.3 mTLS, TCP+TLS 1.3 fallback)

---

## Planned sequence

Unless deliberately changed, the intended order remains:

1. T-0001 — agents workspace baseline
2. T-0002 — config loading scaffolding
3. T-0003 — root/coordinator bootstrap scaffolding
4. T-0004 — node identity and admission-token scaffolding
5. T-0005 — minimal node-to-coordinator control session
6. T-0006 — service registration basics
7. T-0007 — association basics
8. T-0008 — direct raw UDP carriage
9. T-0009 — WireGuard-over-mesh direct-path validation
10. T-0010 — single relay hop
11. T-0011 — scheduler baseline and multi-WAN refinement

---

## Current blockers

No hard technical blocker is currently recorded.

The main risk right now is **architecture drift during early implementation**, not lack of ideas.

---

## Immediate priority rules

Right now, prioritize:

1. keeping specs, docs, and agent context consistent
2. building on the completed config, trust-bootstrap, node bootstrap, control-session, service-registration, association, direct raw UDP carriage, WireGuard-over-mesh direct-path validation, and single relay hop foundation
3. avoiding premature networking/transport complexity
4. preserving the v1 boundaries already chosen
5. continuing `agents/` workspace maintenance as implementation progresses

---

## Updating rule

Whenever task state changes, update:

- this file
- the relevant file under `agents/tasks/`
- `agents/CONTEXT.md` if the current phase, priorities, or blockers changed

If a future agent would need to know it, write it down.

---
