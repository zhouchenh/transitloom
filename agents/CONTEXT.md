# agents/CONTEXT.md

## Purpose

This file captures the **current working context** of the Transitloom repository.

Unlike `agents/IDENTITY.md` and `agents/SOUL.md`, which are meant to stay relatively stable, this file should be updated whenever the project’s active phase, immediate priorities, implementation status, or known blockers change.

This file exists because coding agents are context-limited and should not rely on remembering recent repository state across sessions.

---

## Current project phase

Transitloom is currently in the:

**implementation bootstrap phase**

That means:

- the project has moved beyond broad architecture brainstorming
- the initial v1 spec set has been drafted
- the initial docs set has been drafted
- the initial Go module and repository skeleton have been created
- the `agents/` workspace baseline is being established
- the next step is to begin the first disciplined implementation slice

The project is **not** yet in feature-development mode for advanced networking behavior.

It is currently in the stage where:
- architecture must remain consistent
- object boundaries must remain clean
- implementation sequencing matters a lot
- foundational mistakes are more dangerous than slow progress

---

## Current repository status

At the time this file is written, the repository already contains:

### Top-level documents
- `README.md`
- `LICENSE`
- `AGENTS.md`

### Specs
- `spec/v1-architecture.md`
- `spec/v1-control-plane.md`
- `spec/v1-data-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-pki-admission.md`
- `spec/v1-wireguard-over-mesh.md`
- `spec/v1-object-model.md`
- `spec/v1-config.md`
- `spec/implementation-plan-v1.md`

### Docs
- `docs/vision.md`
- `docs/concepts.md`
- `docs/roadmap.md`
- `docs/glossary.md`

### Agent workspace files drafted so far
- `AGENTS.md`
- `agents/README.md`
- `agents/BOOTSTRAP.md`
- `agents/IDENTITY.md`
- `agents/SOUL.md`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`
- `agents/CODING.md`
- `agents/REPORTING.md`

### Agent task files drafted so far
- `agents/tasks/T-0001-agents-workspace-baseline.md`
- `agents/tasks/T-0002-config-loading-scaffolding.md`
- `agents/tasks/T-0003-root-coordinator-bootstrap.md`
- `agents/tasks/T-0004-node-identity-and-admission-token-scaffolding.md`
- `agents/tasks/T-0005-minimal-node-to-coordinator-control-session.md`

### Agent workspace directories
- `agents/tasks/`
- `agents/context/`
- `agents/memory/`
- `agents/logs/`

### Code skeleton
- `go.mod`
- `cmd/transitloom-root/main.go`
- `cmd/transitloom-coordinator/main.go`
- `cmd/transitloom-node/main.go`
- `cmd/tlctl/main.go`

### Internal package skeleton
- `internal/admission/`
- `internal/config/`
- `internal/controlplane/`
- `internal/coordinator/`
- `internal/dataplane/`
- `internal/identity/`
- `internal/node/`
- `internal/objectmodel/`
- `internal/pki/`
- `internal/scheduler/`
- `internal/service/`
- `internal/status/`
- `internal/transport/`

The code is no longer entirely placeholder-level. The first real
implementation slices now exist for role-specific config loading, trust
bootstrap, node identity/admission bootstrap inspection, a bootstrap-only
node-to-coordinator control-session path, and a first bootstrap-only service
registration path.

---

## Current implementation state

### What is already done
- project naming is settled: **Transitloom**
- license choice is settled: **GPL-3.0**
- v1 architecture direction is documented
- v1 control-plane direction is documented
- v1 data-plane direction is documented
- service model is documented
- PKI/admission model is documented
- WireGuard-over-mesh model is documented
- object model is documented
- config model is documented
- implementation plan is drafted
- repository and command/package skeleton exist
- agent workspace baseline has been largely drafted
- coding standards and reporting standards have dedicated agent files
- role-specific YAML config structs exist for root, coordinator, and node
- `internal/config` now loads YAML with strict known-field checking
- root/coordinator/node startup now accepts `-config`, loads config, validates it, and starts placeholder runtime output
- config validation tests and sample YAML fixtures now exist for root/coordinator/node
- `internal/pki` now contains explicit root and coordinator trust-bootstrap inspection helpers
- root trust-material references now resolve relative to `storage.data_dir` when configured as local relative paths
- root startup now reports bootstrap state and rejects inconsistent or missing root material unless `trust.generate_key=true`
- coordinator startup now requires a present root trust anchor, reports coordinator intermediate bootstrap state, and rejects partial intermediate material
- node config now carries distinct `node_identity` and `admission` sections for persisted local identity material and cached current admission-token state
- `internal/identity` now inspects node certificate/key presence and distinguishes bootstrap-required, awaiting-certificate, and ready identity states
- `internal/admission` now inspects cached current admission-token metadata and distinguishes missing, usable, and expired local token state without treating that cache as authoritative truth
- `internal/node` now combines identity and admission inspection into explicit bootstrap readiness reporting for `transitloom-node`
- `transitloom-node` now rejects the incoherent local state where a cached current admission token exists but ready node identity material does not
- identity/admission bootstrap tests now cover valid and invalid local state combinations plus command-level startup verification
- `internal/controlplane` now contains a minimal bootstrap-session request/response model that carries only node-local readiness summary data and a structured bootstrap-only result
- `internal/coordinator` now exposes a bootstrap-only HTTP JSON endpoint on the configured TCP control listener(s), evaluates coordinator bootstrap state plus the node-reported readiness phase, and returns explicit accept/reject reasons without claiming final authentication
- `internal/node` now builds bootstrap-session requests from the existing identity/admission readiness inspection, retries bootstrap coordinator endpoints until one returns a structured result, and reports transport failures separately from coordinator rejection
- `transitloom-coordinator` now starts a minimal bootstrap control listener and stays running until signaled
- `transitloom-node` now attempts the bootstrap control session after local readiness inspection and exits clearly on success vs rejection/failure
- focused control-session tests now cover coordinator acceptance/rejection plus node-side endpoint fallback and structured rejection handling
- `internal/service` now maps configured services into explicit registration declarations with separate service identity, service binding/local target, and requested local-ingress intent
- `internal/controlplane` now contains a bootstrap-only service-registration request/response model with per-service results and explicit placeholder semantics
- `internal/coordinator` now exposes a bootstrap-only service-registration endpoint on the same minimal TCP listener, validates service declarations individually, and stores bootstrap-only placeholder service records in an in-memory registry
- `internal/node` now builds service-registration requests from configured services and submits them to the coordinator endpoint that accepted the bootstrap control session
- `transitloom-node` now attempts bootstrap-only service registration after bootstrap control-session success and exits clearly on full success vs partial or failed registration
- focused service-registration tests now cover request mapping, coordinator-side stored registry state, partial rejection of invalid service declarations, and node-side registration attempts

### What is not done yet
- no real object model implementation in Go
- no PKI issuance logic
- no node certificate issuance
- no node enrollment flow
- no live admission-token issuance or refresh logic
- no coordinator-side admission-token validation logic
- no final QUIC + TLS 1.3 mTLS control transport implementation
- no final TCP + TLS 1.3 fallback transport implementation
- no live certificate-chain validation during sessions
- no service discovery implementation
- no live association lifecycle management or policy evaluation
- no live path quality measurement (RTT/jitter/loss from real traffic or probes) — scheduler uses static zero-quality candidates from config
- no multi-path carrier load balancing at the socket level
- no coordinator-distributed path candidates

The first WireGuard-over-mesh direct-path validation now works end-to-end. Direct raw UDP carriage is wired into the node startup flow via `DirectPathRuntime`. Standard WireGuard can use Transitloom local ingress ports as peer endpoints on a direct path with zero in-band overhead.

Single relay hop basics (T-0010) are implemented. `RelayCarrier` (coordinator relay), `RelayEgressCarrier` (source node egress), and associated forwarding tables exist in `internal/dataplane`. `CoordinatorRelayRuntime` and `RelayPathRuntime` exist for integration. The single-hop constraint is structurally enforced; destination delivery reuses the existing `DirectCarrier.StartDelivery` path.

Scheduler baseline and multi-WAN refinement (T-0011) are now implemented. `internal/scheduler` now contains the first endpoint-owned scheduler: `PathCandidate`, `RelayCandidate`, `PathQuality`, `PathClass`, `HealthState`, `SchedulerDecision`, `Mode`, `ChosenPath`, `Scheduler`, `StripeMatchThresholds`, `AssociationCounters`, `SchedulerStatus`. The scheduler filters candidates by association ID + health, scores by AdminWeight + relay penalty + quality, defaults to `ModeWeightedBurstFlowlet`, and activates `ModePerPacketStripe` only when all paths are within configured thresholds. 25 tests and 2 benchmarks pass.

Scheduler-to-carrier integration (T-0014) is now implemented. `ScheduledEgressRuntime` combines `Scheduler` + `DirectPathRuntime` + `RelayPathRuntime`. `ActivateScheduledEgress` calls `Scheduler.Decide()` at the egress decision point for each association, then activates the chosen carrier: `DirectCarrier` for direct-class paths, `RelayEgressCarrier` for relay-class paths. Direct paths are preferred over relay via relay penalty. Striping is not activated for unmeasured candidates (confidence=0). `ScheduledEgressActivation.CarrierActivated` and `Decision` fields are always aligned for operator observability. `cmd/transitloom-node/main.go` now uses `BuildScheduledActivationInputs` + `ActivateScheduledEgress`. `relay_endpoint` format validation added to `validateAssociation`. 17 focused tests pass.

Control-plane transport hardening (T-0012) is now implemented. `internal/controlplane/transport.go` defines named constants for all bootstrap transport timeouts, retry limits, and body size limits. `internal/controlplane/errors.go` defines `TransportErrorKind`, `TransportError`, and `ClassifyTransportError()`. The coordinator bootstrap listener now has full HTTP server timeouts (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes`) and `http.MaxBytesReader` body limiting on all handler paths. Node bootstrap session now performs bounded exponential backoff retry for timeout errors only (up to `BootstrapRetryMaxAttempts`), skips immediately for connection-refused, and aborts immediately for context cancellation. `BootstrapEndpointAttempt` now carries `ErrorKind` for structured observability. 12 focused tests added and passing.

Runtime observability and debugging basics (T-0013) are now implemented. `internal/status` package now provides narrow, explicit summary types: `BootstrapSummary` (node local readiness — not coordinator authorization), `ServiceRegistrySummary` (coordinator service registry snapshot), `AssociationStoreSummary` (coordinator association snapshot), `ScheduledEgressSummary` (applied scheduler/carrier state with live traffic counters). Each type has `ReportLines()` for operator-friendly logging. `ScheduledEgressRuntime.Snapshot()` in `internal/node` returns a live `ScheduledEgressSummary` by combining stored activation results with live carrier counters (`DirectCarrier.IngressStats` / `RelayEgressCarrier.EgressStats`). `BootstrapListener.RuntimeSummaryLines()` in `internal/coordinator` surfaces current service registry and association state. 13 focused tests cover key semantic distinctions (applied vs computed state, stripe-gap visibility, "ready ≠ authorized", "pending ≠ active", bootstrap-placeholder labeling). `go build ./...` and `go test ./...` pass.

tlctl runtime inspection and operator workflows basics (T-0016) are now implemented. `cmd/tlctl/main.go` (previously a stub) now provides six read-oriented subcommands: `node bootstrap` (local identity/admission readiness from disk), `node config` (configured services/associations/external endpoints, labeled as configured-state-only), `node status` (runtime scheduler/carrier state via HTTP status endpoint), `coordinator bootstrap` (trust material readiness from disk), `coordinator config` (configured transport/trust/relay), `coordinator status` (runtime service registry + association state via HTTP endpoint). `internal/status/server.go` adds `StatusServer` (read-only GET /status, text/plain, no mutation surface). Both `transitloom-node` and `transitloom-coordinator` now start the status server if `observability.status.enabled: true` and `observability.status.listen` is set. All output preserves architectural state boundaries: configured ≠ registered/active/verified, bootstrap-ready ≠ coordinator-authorized, DNAT external/local ports preserved separately, service registry and association state remain distinct sections. 12 focused tests in `cmd/tlctl/inspect_test.go`.

External endpoint advertisement and DNAT-aware reachability basics (T-0015) are now implemented. `internal/transport/endpoint.go` defines `ExternalEndpoint`, `EndpointSource` (configured/router-discovered/probe-discovered/coordinator-observed), `VerificationState` (unverified/verified/stale/failed), `RouterDiscoveryHint`, `ProbeResult`, `NewConfiguredEndpoint`, `ValidateAddrPort`, and MarkStale/MarkVerified/MarkFailed state transitions. `internal/config/common.go` now carries `ExternalEndpointConfig` (with `PublicHost` and `ForwardedPorts`) and `ForwardedPortConfig` (with separate `ExternalPort` and `LocalPort` to preserve DNAT-awareness). `NodeConfig` now carries `ExternalEndpoint ExternalEndpointConfig` and validates it. The model explicitly separates local target, local ingress, mesh/runtime port, and external advertised endpoint. Narrow placeholder types for future UPnP/PCP/NAT-PMP discovery and targeted probing are defined. 11 focused test functions covering all modeling behavior pass.

Targeted external endpoint probing and freshness revalidation basics (T-0017) are now implemented. `internal/transport/probe.go` defines the 13-byte TLPR probe wire protocol (magic+type+nonce), `CandidateReason` (configured/router-discovered/coordinator-observed/previously-verified), `ProbeCandidate` with `Validate()`, `BuildCandidatesFromEndpoints()` (targeted only — no blind port scanning), `BuildCoordinatorObservedCandidates()`, `ProbeExecutor` interface, and `UDPProbeExecutor` (actual UDP challenge/response with crypto/rand nonce and context deadline). `internal/transport/responder.go` adds `ProbeResponder` (standalone UDP listener that echoes probe nonces) and `HandleProbeDatagram()` (for integrating probe handling into existing listeners). `internal/transport/registry.go` adds `EndpointRegistry` (thread-safe collection with Add, MarkAllStale, SelectForRevalidation, SelectForInitialVerification, UsableEndpoints, Snapshot, ApplyProbeResult) and `RevalidationTrigger` constants. `internal/controlplane/probe_assist.go` adds `CoordinatorProbeRequest` / `CoordinatorProbeResponse` / `ValidateCoordinatorProbeRequest` for the coordinator-assisted probing model. `internal/status/endpoint.go` adds `EndpointFreshnessSummary`, `MakeEndpointFreshnessSummary()`, and `ReportLines()` for operator-visible freshness reporting. 35 focused tests added across transport, controlplane, and status packages.

---

## Current v1 architectural boundaries

These boundaries are already chosen and should be treated as active constraints.

### Data plane
- raw UDP is the primary v1 data-plane transport
- zero in-band overhead is required for raw UDP
- v1 raw UDP data plane supports:
  - direct public paths
  - direct intranet/private paths
  - single coordinator relay hop
  - single node relay hop
- v1 raw UDP data plane does **not** support arbitrary multi-hop forwarding
- data-plane scheduling is endpoint-owned
- default scheduler is weighted burst/flowlet-aware
- per-packet striping is allowed only when paths are closely matched

### Control plane
- control plane is more flexible than data plane
- QUIC + TLS 1.3 mTLS is primary
- TCP + TLS 1.3 mTLS is fallback
- control semantics should stay logically consistent across both transports

### Trust and admission
- node identity and participation permission are separate
- a valid certificate alone is not enough for normal participation
- normal participation requires:
  - valid node certificate
  - valid admission token
- revoke is hard in operational effect
- root authority is not a normal node-facing coordinator target

### Service model
- core model remains generic
- WireGuard is the flagship v1 use case in docs and examples
- service, service binding, local target, and local ingress are distinct concepts
- multiple services per node are supported
- multiple WireGuard services per node are supported

### Product scope
- Transitloom v1 is not trying to be a full unconstrained service mesh
- Transitloom v1 is trying to make the flagship raw-UDP transport path work well first
- multi-WAN aggregation is still a primary practical target

---

## Current implementation priorities

The current implementation priorities, in order, are:

1. preserve architectural consistency
2. preserve object model boundaries
3. finish the usable `agents/` workspace baseline
4. start implementation in the order defined by `spec/implementation-plan-v1.md`
5. avoid premature feature expansion
6. prove the first real vertical slice as early as possible

The intended implementation order is:

1. config and object-model-aligned scaffolding
2. root/coordinator bootstrap
3. node identity and admission-token flow
4. minimal node-to-coordinator control session
5. service registration
6. association creation/distribution
7. direct raw UDP carriage
8. WireGuard-over-mesh direct path
9. single relay hop
10. scheduler and multi-WAN refinement

---

## Immediate next tasks

The immediate next tasks are:

### Agent workspace completion and normalization
The `agents/` workspace now has a solid baseline, but it should continue to be normalized and kept consistent as work begins.

Near-term agent-workspace work includes:
- keeping `AGENTS.md` and the `agents/` files consistent
- ensuring `agents/TASKS.md` stays a compact index
- using `agents/tasks/*.md` for detailed task tracking
- updating `agents/CONTEXT.md`, `agents/MEMORY.md`, and task files as progress is made

### Implementation bootstrap
The first real implementation work has begun with config loading scaffolding, trust bootstrap scaffolding, node identity/admission bootstrap scaffolding, and a bootstrap-only node-to-coordinator control-session path.
The next implementation work should continue with:

- association basics built on the new service-registration foundation
- live enrollment, certificate issuance, and admission-token refresh work after association basics or as a deliberately split prerequisite if needed

### Current active implementation-oriented task
The completed implementation tasks are:

- `T-0002 — config loading scaffolding`
- `T-0003 — root/coordinator bootstrap scaffolding`
- `T-0004 — node identity and admission-token scaffolding`
- `T-0005 — minimal node-to-coordinator control session`
- `T-0006 — service registration basics`
- `T-0007 — association basics`
- `T-0008 — direct raw UDP carriage basics`
- `T-0009 — WireGuard-over-mesh direct-path validation`
- `T-0010 — single relay hop basics`
- `T-0011 — scheduler baseline and multi-WAN refinement`
- `T-0012 — control-plane transport hardening`
- `T-0013 — runtime observability and debugging basics`
- `T-0014 — scheduler-to-carrier integration`
- `T-0015 — external endpoint advertisement and DNAT-aware reachability basics`
- `T-0016 — tlctl runtime inspection and operator workflows basics`
- `T-0017 — targeted external endpoint probing and freshness revalidation basics`

The next practical implementation task is:

- transport-security maturation (QUIC+TLS 1.3 mTLS, TCP+TLS 1.3 fallback), or
- path-candidate distribution refinement building on the new external endpoint model.

---

## First target milestone

The first meaningful milestone remains:

**two admitted nodes, one coordinator, one UDP service per node, one legal association, direct raw UDP carriage working**

The first flagship validation milestone after that remains:

**WireGuard-over-mesh over a direct path, using Transitloom local ingress ports**

These milestones should guide what gets built first.

---

## Current risks

The biggest current risks are architectural and sequencing risks, not low-level code bugs.

### Risk 1: architecture drift
Now that code skeleton and agent workspace exist, it is easy for implementation to drift away from the specs if agents start coding from intuition instead of reading.

### Risk 2: collapsing important concepts
The following distinctions are easy to accidentally collapse:
- identity vs admission
- service vs service binding
- local target vs local ingress
- relay candidate vs path candidate
- config vs distributed state

These distinctions must be preserved.

### Risk 3: premature abstraction
It would be easy to build:
- a broad routing framework
- a broad transport abstraction layer
- a broad policy engine
- a broad service-mesh API shape

before the first direct raw UDP vertical slice works.

That would likely slow the project and weaken the architecture.

### Risk 4: implementation in the wrong order
If coding starts with:
- advanced scheduler logic
- advanced discovery/routing
- broad relay behavior
- WireGuard helpers
- speculative encrypted transport

before the trust/control/service/direct-path foundation exists, progress will look larger than it really is.

### Risk 5: poor continuity discipline
Because agents are context-limited, failing to update the `agents/` workspace when meaningful progress or learning occurs is a real project risk, not merely a documentation lapse.

---

## Current practical guidance

At this stage, agents should optimize for:

- simple, clean package boundaries
- object-model fidelity
- correct trust/admission separation
- config clarity
- minimal viable vertical slices
- good status/observability scaffolding
- recording progress in `agents/`
- honest reporting using `agents/REPORTING.md`
- coding discipline using `agents/CODING.md`

Agents should **not** optimize for:
- broad feature counts
- speculative future transport types
- elaborate routing machinery
- local code elegance that breaks the current architecture

---

## Current task-system state

The task system is intended to work like this:

- `agents/TASKS.md` = compact task index
- `agents/tasks/*.md` = detailed task files
- `agents/REPORTING.md` = end-of-run reporting standard
- `agents/CODING.md` = coding standards

This should remain the working model unless deliberately changed.

---

## What should be updated next

This file should be updated when:

- the agent workspace baseline is fully stabilized
- the first implementation package content becomes real
- the project moves from bootstrap into actual config/trust/control implementation
- the active task changes materially
- the first milestone changes
- a serious blocker appears
- the immediate next tasks change

---

## Current summary

Transitloom is currently a **well-specified and now meaningfully implemented** project with:

- strong v1 specs
- a clear flagship use case
- a clear implementation order
- a repo/code skeleton
- a mostly established `agents/` workspace
- explicit coding and reporting standards
- verified config loading/validation scaffolding
- verified root/coordinator trust bootstrap validation and placeholder reporting
- verified node identity and admission bootstrap validation, readiness reporting, and invalid-local-state rejection
- verified bootstrap-only node-to-coordinator control-session scaffolding over the coordinator TCP listener, with explicit non-final-auth semantics
- verified bootstrap-only association creation scaffolding with explicit intent validation, coordinator-side in-memory association store, per-association accept/reject results, and clear separation from service registration and path/forwarding behavior
- verified direct raw UDP carriage: ForwardingTable with association-bound lookup, DirectCarrier with local ingress listeners and local target delivery, zero in-band overhead, and explicit direct-only scope
- verified WireGuard-over-mesh direct-path validation: `DirectPathRuntime` wires carriage into node startup, end-to-end delivery works with zero in-band overhead, local ingress and local target remain distinct, standard WireGuard can use Transitloom local ingress ports as peer endpoints
- verified single relay hop basics: `RelayForwardingEntry`/`RelayForwardingTable`/`RelayCarrier` (coordinator relay), `RelayEgressEntry`/`RelayEgressTable`/`RelayEgressCarrier` (source node relay egress), `CoordinatorRelayRuntime`, `RelayPathRuntime`, end-to-end single-hop carriage test proves local app → relay egress → coordinator relay → mesh delivery → local target with zero overhead; single-hop constraint structurally enforced; direct vs relay carriage kept architecturally distinct
- no substantive issuance code yet

tlctl operator inspection baseline (T-0016) is now implemented. `cmd/tlctl/main.go`
provides six read-oriented subcommands: `node bootstrap`, `node config`, `node status`,
`coordinator bootstrap`, `coordinator config`, `coordinator status`. Bootstrap and config
commands read from disk (no running process needed). Status commands query the
`observability.status` HTTP endpoint exposed by running processes. `internal/status/server.go`
adds `StatusServer` (read-only, GET /status only, text/plain). The status server is wired
into both `transitloom-node` and `transitloom-coordinator` using the existing
`observability.status` config section. All output preserves architectural boundaries:
configured ≠ registered/active/verified, bootstrap-ready ≠ coordinator-authorized, DNAT
external and local ports kept separate, service registration and association state remain
distinct sections. 12 focused tests added and passing.

The correct next move is to keep the `agents/` workspace accurate and continue
the staged implementation order. T-0017 is complete; next is transport-security
maturation (QUIC+TLS 1.3 mTLS, TCP+TLS 1.3 fallback) or path-candidate
distribution refinement building on the now-operational endpoint probing model.

---
