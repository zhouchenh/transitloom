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
- scheduler-to-carrier integration (completed in T-0014; Decide() results now govern direct vs relay carrier activation)
- live path quality measurement (RTT/jitter/loss from real traffic)
- multi-path carrier load balancing at the socket level
- coordinator-distributed path candidates (now implemented at the distribution/consumption layer; runtime selection integration is future work)

---

## Active task

No active task.

---

## Recently completed

### T-0018 — path candidate distribution and consumption basics
**status:** completed
**task file:** `agents/tasks/T-0018-path-candidate-distribution-and-consumption-basics.md`

Implemented the first coordinator-mediated path-candidate distribution and
node-side consumption flow. Added `DistributedPathCandidate` wire model with
explicit relay/direct distinction in `internal/controlplane/path_candidate.go`.
Added `/v1/bootstrap/path-candidates` HTTP endpoint to the coordinator bootstrap
listener via `internal/coordinator/path_candidates.go` + updates to
`bootstrap_listener.go`. Added node-side `CandidateStore` + `StoreCandidates`
in `internal/node`. Coordinator generates relay candidates from configured relay
endpoints; direct candidates require node endpoint advertisement (future work,
noted via `Notes` field). `DistributedPathCandidate` is explicitly separate from
`scheduler.PathCandidate`, `ForwardingEntry`, and `SchedulerDecision`. 30+
focused tests across three packages, all passing. `go build ./...` clean.

### T-0019 — live path quality measurement basics
**status:** completed
**task file:** `agents/tasks/T-0019-live-path-quality-measurement-basics.md`

Implemented the first live path-quality measurement baseline.
`PathQualityStore` in `internal/scheduler`: thread-safe, freshness-aware store
with EWMA RTT/jitter/loss/confidence accumulation via `RecordProbeResult`,
direct quality injection via `Update`, explicit staleness via `FreshQuality`
(returns zero when older than `DefaultQualityMaxAge`=60s), and `ApplyCandidates`
which enriches PathCandidates before `Scheduler.Decide()`. `MeasuredPathQuality`
snapshot type for observability.
`PathQualitySummary` + `MakePathQualitySummary` added to `internal/status`
for operator-visible quality reporting with stale/fresh labeling.
`ScheduledEgressRuntime.QualityStore` added to `internal/node`; wired into
`activateSingleScheduledEgress` before `Decide()`. `QualitySnapshot()` method
exposes measurement state separately from carrier state. `NewScheduledEgressRuntime`
now initializes a `PathQualityStore` by default.
`ReportSchedulerStatus()` updated to reflect live measurement as implemented.
12 focused tests added in `internal/scheduler/quality_store_test.go`, 5 tests
in `internal/status/quality_test.go`, 4 integration tests added to
`internal/node/scheduled_egress_test.go`. 1 benchmark added
(`BenchmarkApplyCandidates`: ~222 ns/op for 4 candidates).
`go build ./...` and `go test ./...` both pass.

### T-0017 — targeted external endpoint probing and freshness revalidation basics
**status:** completed
**task file:** `agents/tasks/T-0017-targeted-external-endpoint-probing-and-freshness-revalidation-basics.md`

Implemented targeted external endpoint probing and freshness revalidation in
`internal/transport`. Added a probe wire protocol (13-byte magic+type+nonce
datagram), `ProbeCandidate` with `CandidateReason` enforcing targeted-first
discipline, `UDPProbeExecutor` for actual UDP challenge/response probing,
`ProbeResponder` / `HandleProbeDatagram` for integration into listeners,
`EndpointRegistry` for thread-safe endpoint collection with staleness/revalidation
management, `RevalidationTrigger` types. Added `CoordinatorProbeRequest` /
`CoordinatorProbeResponse` in `internal/controlplane` for coordinator-assisted
probing model. Added `EndpointFreshnessSummary` + `ReportLines()` in
`internal/status`. Added 35 focused tests covering: probe protocol, candidate
building (targeted, no broad scan), registry lifecycle, stale/revalidate cycle,
end-to-end UDP probe with local ProbeResponder, coordinator probe validation,
endpoint freshness summary construction and reporting.
`go build ./...` and `go test ./...` both pass.

### T-0016 — tlctl runtime inspection and operator workflows basics
**status:** completed
**task file:** `agents/tasks/T-0016-tlctl-runtime-inspection-and-operator-workflows-basics.md`

Replaced the `tlctl` stub with a practical read-oriented operator inspection CLI.
Implemented `internal/status/server.go` — a read-only HTTP status server
(`StatusServer` with `NewStatusServer`/`ListenAndServe`/`/status` GET endpoint,
text/plain output, no mutation surface). Wired the status server into
`cmd/transitloom-node/main.go` and `cmd/transitloom-coordinator/main.go` using
the existing `observability.status` config (enabled+listen address).
`cmd/tlctl/main.go` now provides six subcommands in `tlctl <role> <action>` form:
`node bootstrap` (node local readiness from disk, reuses `node.InspectBootstrap`),
`node config` (configured services/associations/endpoints from config, labels
"configured" vs "registered"/"active"/"verified"), `node status` (queries HTTP
status endpoint for scheduler/carrier state), `coordinator bootstrap` (trust
material from disk, reuses `pki.InspectCoordinatorBootstrap`), `coordinator config`
(configured transport/trust/relay), `coordinator status` (queries HTTP endpoint for
registry/association state). All output preserves architectural state boundaries:
configured ≠ registered, bootstrap-ready ≠ coordinator-authorized, DNAT external
and local ports kept separate. Added 12 focused tests in `cmd/tlctl/inspect_test.go`
covering configured-state labeling, service/association/external-endpoint semantics,
DNAT vs no-DNAT distinction, missing-endpoint "(none)" display, coordinator output
structure. `go build ./...` and `go test ./...` both pass.

### T-0015 — external endpoint advertisement and DNAT-aware reachability basics
**status:** completed
**task file:** `agents/tasks/T-0015-external-endpoint-advertisement-and-dnat-aware-reachability-basics.md`

Implemented explicit external-endpoint advertisement and DNAT-aware
reachability modeling in `internal/transport/endpoint.go`. Defined
`ExternalEndpoint`, `EndpointSource` (configured/router-discovered/
probe-discovered/coordinator-observed), `VerificationState`
(unverified/verified/stale/failed), `RouterDiscoveryHint`,
`ProbeResult`, `NewConfiguredEndpoint`, `ValidateAddrPort`, and state
transition methods (MarkStale/MarkVerified/MarkFailed). Added
`ExternalEndpointConfig` + `ForwardedPortConfig` to
`internal/config/common.go` with DNAT-aware external_port/local_port
separation, wired `external_endpoint` into `NodeConfig.Validate()`.
Added 11 focused test functions (with subtests) covering: Validate,
DNAT distinctions, state transitions, usability contract,
source-of-knowledge semantics, RouterDiscoveryHint conversion,
ProbeResult application, ValidateAddrPort, constructor, and
stale-after-down revalidation pattern.
`go build ./...` and `go test ./...` both pass.

### T-0013 — runtime observability and debugging basics
**status:** completed
**task file:** `agents/tasks/T-0013-runtime-observability-and-debugging-basics.md`

Implemented the first explicit runtime observability and debugging baseline.
Added `internal/status` package with narrow summary types: `BootstrapSummary`,
`ServiceRegistrySummary`, `AssociationStoreSummary`, `ScheduledEgressSummary`.
Each summary preserves architectural state distinctions (configured vs applied,
bootstrap/cached vs authorized, pending vs active). Added `ReportLines()` on
each summary type for operator-friendly logging. Added `ScheduledEgressRuntime.Snapshot()`
in `internal/node` combining stored activation results with live carrier counters
(direct `IngressStats` / relay `EgressStats`). Added `RuntimeSummaryLines()` to
coordinator `BootstrapListener` surfacing current registry and association state.
Added 13 focused tests covering key semantic distinctions (applied vs computed
carrier state, stripe-gap visibility, "ready ≠ authorized", "pending ≠ active",
bootstrap-placeholder labeling, zero-counter suppression, etc.).

### T-0014 — scheduler-to-carrier integration
**status:** completed
**task file:** `agents/tasks/T-0014-scheduler-to-carrier-integration.md`

Implemented `ScheduledEgressRuntime` + `ActivateScheduledEgress` in `internal/node`.
`Scheduler.Decide()` is called at the egress decision point for each association;
result governs whether `DirectCarrier` or `RelayEgressCarrier` is activated.
Direct paths preferred over relay via relay penalty; striping blocked for unmeasured
paths (confidence=0). `ScheduledEgressActivation.CarrierActivated` + `Decision`
fields are always aligned for operator observability. `cmd/transitloom-node/main.go`
now uses `BuildScheduledActivationInputs` + `ActivateScheduledEgress` instead of
`BuildAssociationActivationInputs` + `ActivateDirectPaths`. Added `relay_endpoint`
format validation to config. Updated `ReportSchedulerStatus()` to show integration
as implemented. Added 17 focused tests.
`go build ./...` and `go test ./...` both pass.

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
