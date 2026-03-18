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
- coordinator-distributed path candidates (distribution/consumption layer T-0018 done; refinement layer T-0020 done — candidates now enriched by endpoint freshness + quality before Decide(); active probe scheduling loop not yet wired)

---

## Active task

None. T-0031 has been completed. See queued tasks below for the next work.

## Recently completed

### T-0031 — project integration consolidation audit
**status:** completed
**task file:** `agents/tasks/T-0031-project-integration-consolidation-audit.md`

Performed a focused post-integration audit across all T-0002 through T-0030 deliverables.
Inspected runtime, control, config, and status surfaces from actual code. Key findings:

1. **EffectivePolicy gap (highest-priority):** `config.EffectivePolicy` from T-0028 is resolved in
   `tlctl node config` for display only. Runtime components (`FallbackConfig`, `MultiWANStickinessConfig`,
   `ProbeSchedulerConfig`) all use hardcoded defaults. `activateSingleScheduledEgress` does not call
   `ResolvePolicy()` per association. Profile/policy_overrides have no effect on runtime behavior.

2. **SecureControlListener not started:** `coordinator.SecureControlListener` (TCP+TLS 1.3 mTLS) is
   fully implemented and tested but is not started in `transitloom-coordinator/main.go`. All control
   sessions remain bootstrap-only HTTP at runtime. `ControlSessionRuntime` explicitly labels them
   `SessionAuthenticated=false` via `BootstrapOnlyTransportStatus()`.

3. **All other live features confirmed:** scheduler-to-carrier integration, fallback/recovery,
   stickiness/hysteresis, candidate refinement, probe loop, control session reconciliation, event
   history, status server, tlctl inspection — all wired and running in `main.go`.

4. **Per-packet striping gap is observable but correct:** Scheduler decides ModePerPacketStripe,
   but multi-carrier striping at the socket level is deferred. The gap is logged in ScheduledEgressSummary.

5. **Signal layering is clean:** EndpointRegistry, PathQualityStore, CandidateFreshnessStore,
   FallbackState, StickinessState, and chosen runtime path are all distinct and non-overlapping.

6. **Status/reporting is honest:** bootstrap-ready ≠ authorized, configured ≠ active, bootstrap-only
   transport is always labeled as such. One gap: TASKS.md was missing T-0027 and T-0028 — corrected.

Updated: `agents/TASKS.md`, `agents/CONTEXT.md`, `agents/MEMORY.md`, `agents/tasks/T-0031-*.md`.
`go build ./...` and `go test ./...` pass.

### T-0028 — config profile and policy bundling basics
**status:** completed
**task file:** `agents/tasks/T-0028-config-profile-and-policy-bundling-basics.md`

Introduced `ProfileConfig`, `PolicyBundle`, and `EffectivePolicy` in `internal/config/profile.go`.
`ResolvePolicy()` layers system defaults, profile settings, and inline overrides.
`tlctl node config` outputs fully resolved effective policy per association.
Tests added for resolution and override logic. **Note (T-0031 audit finding):** `EffectivePolicy`
is currently consumed only in `tlctl node config` display — runtime components still use
hardcoded defaults (`DefaultFallbackConfig`, `DefaultMultiWANStickinessConfig`,
`DefaultProbeSchedulerConfig`). Wiring EffectivePolicy into runtime is a high-priority follow-up.

### T-0027 — control-plane session resume and state reconciliation basics
**status:** completed
**task file:** `agents/tasks/T-0027-control-plane-session-resume-and-state-reconciliation-basics.md`

Added `ControlSessionRuntime` in `internal/node/session_reconcile.go` with bounded reconnect
loop (`ControlSessionResumeInterval=10s`), explicit phases (disconnected, transport-reconnected,
session-established, reconciling, reconciled, reconciliation-failed), and per-step outcomes
(service-refresh, association-refresh, path-candidate-refresh). Disconnect handling marks
coordinator-derived candidate freshness stale and marks endpoint registry stale. Status surface
in `internal/status` exposes `ControlReconciliationSummary`. Wired into `transitloom-node/main.go`
as a background goroutine. **Note:** runtime uses bootstrap-only HTTP transport; `SessionAuthenticated`
will be false until secure transport is active.

### T-0030 — live node probe-loop lifecycle integration basics
**status:** completed
**task file:** `agents/tasks/T-0030-live-node-probe-loop-lifecycle-integration-basics.md`

Implemented the first bounded live lifecycle integration for the active probe loop.
`cmd/transitloom-node/main.go` now starts the probe loop during live runtime after
scheduled egress activation and ties cancellation to the node runtime context.
`internal/node/scheduled_egress.go` now owns probe-loop lifecycle/status state with
explicit states (`disabled`, `blocked`, `active`, `waiting-prerequisites`, `stopped`)
and last-round counters/timestamp. `internal/node/probe_scheduler.go` now reports
zero-target rounds through `onRound`, keeping waiting state inspectable instead of
hidden idle behavior. `internal/status` now exposes probe-loop runtime summary in
`ScheduledEgressSummary` and status report output. Added focused tests for blocked
prerequisites, waiting/no-target lifecycle behavior, no-target callback delivery,
and probe-loop status reporting. `go build ./...` and `go test ./...` pass.

### T-0029 — active probe scheduling and path usability signal wiring basics
**status:** completed
**task file:** `agents/tasks/T-0029-active-probe-scheduling-and-path-usability-signal-wiring-basics.md`

Implemented the first bounded active probe scheduling loop and path-usability
signal wiring in `internal/node/probe_scheduler.go`. Added:
`ProbeSchedulerConfig` (ProbeInterval, MaxTargetsPerRound), `ProbeTarget`
(host:port + PathIDs for quality-store linkage), `ProbeRoundResult`/`ProbeRoundDetail`
for operator-visible observability, `BuildPathIDMap()` (maps "host:port" →
path-quality-store IDs from config-derived and distributed candidates),
`SelectProbeTargets()` (bounded targeted selection from EndpointRegistry;
unverified first, then stale/failed; deduplicates; enforces maxTargets),
`ExecuteProbeRound()` (wires results into EndpointRegistry.ApplyProbeResult
and PathQualityStore.RecordProbeResult as distinct layers; tracks absent vs
failed measurement; context-aware; executor-error handling),
`BuildDistributedProbeTargets()` (probe targets from CandidateStore for
coordinator-distributed candidates), `RunProbeLoop()` (bounded ticker-driven
goroutine helper with cold-start probe and onRound callback for observability).
Endpoint freshness and path quality remain distinct update layers throughout.
Probe scheduling is separate from fallback/switching policy (T-0024 scope).
22 focused tests added and passing: target selection, bounded fan-out, targeted-
first discipline, absent vs failed measurement, endpoint freshness update,
quality store update, executor error handling, context cancellation, path ID
mapping, and end-to-end integration test. `go build ./...` and `go test ./...` pass.

### T-0024 — multi-WAN policy and hysteresis basics
**status:** completed
**task file:** `agents/tasks/T-0024-multi-wan-policy-and-hysteresis-basics.md`

Implemented the first explicit multi-WAN stickiness and hysteresis layer.
Added `MultiWANStickinessPolicy` and `AssociationStickinessStore` in
`internal/node/stickiness_policy.go`. Added `PathGroup string` field to
`scheduler.PathCandidate` for uplink-group identity. Added exported
`ScoreCandidate()` to the scheduler package as the single authoritative
scoring formula for threshold comparisons. Integrated the stickiness layer
between the fallback filter and `Scheduler.Decide()` in
`activateSingleScheduledEgress()`: `AdjustCandidates()` filters the list
(current-only when suppressing, all candidates when allowing); `RecordSelection()`
updates state and starts hold-down after a switch. Added `StickinessReason`,
`SwitchOccurred`, `HoldDownActive` fields to `ScheduledEgressActivation` and
`status.ScheduledEgressEntry`. Updated `ReportLines()` to surface stickiness
state for operator visibility. Added `NewAssociationStickinessStore` creation in
`NewScheduledEgressRuntime`. 10 focused tests (first selection, trivial suppression,
clear improvement, hold-down, hold-down expiry, current path disappears, switch
detection, per-association isolation, threshold=0 disable, Remove reset).
`go build ./...` and `go test ./...` pass.

### T-0023 — direct-relay fallback and recovery basics
**status:** completed
**task file:** `agents/tasks/T-0023-direct-relay-fallback-and-recovery-basics.md`

Implemented the first explicit direct-to-relay fallback and recovery behavior.
Added `DirectRelayFallbackPolicy` (per-association three-state machine:
`prefer-direct` → `fallen-back-to-relay` → `recovering-to-direct`) and
`AssociationFallbackStore` (per-association policy map) in
`internal/node/fallback_policy.go`. The policy enforces: (1) `MinRelayDwell`
(30s) anti-flap gate — direct candidates filtered even if re-usable until dwell
expires; (2) `RecoveryConfirmWindow` (15s) — direct must be continuously usable
before the policy returns to prefer-direct; (3) relay disappearance while in
relay-fallback state triggers return to prefer-direct (not stuck); (4) direct
failure during recovery window resets dwell timer. `applyFallbackFilter()` removes
direct candidates when `FilterDirect=true`. Integration in `activateSingleScheduledEgress()`:
usability signals derived post-refinement, policy evaluated, filter applied, then
`Scheduler.Decide()` called. `FallbackState`/`FallbackReason` surfaced on
`ScheduledEgressActivation` and `status.ScheduledEgressEntry` for operator visibility.
21 focused tests (state machine, anti-flap, recovery, abort, isolation, filter helpers)
added and passing. `go build ./...` and `go test ./...` pass.

### T-0022 — candidate refresh and revalidation automation basics
**status:** completed
**task file:** `agents/tasks/T-0022-candidate-refresh-and-revalidation-automation-basics.md`

Implemented the first bounded automation layer for refreshing distributed path
candidates and revalidating their supporting endpoint/reachability assumptions.
Added `CandidateFreshnessStore` (per-association staleness tracking with explicit
`CandidateRefreshTrigger` values: endpoint-stale, endpoint-failed, quality-stale,
candidate-expired, path-unhealthy, explicit), `SelectCandidateRefreshTargets()`
(scans three freshness layers in priority order, deduplicates by association ID,
preserves root-cause trigger), `ExecuteCandidateRefresh()` (bounded refresh:
fetches updated candidates from coordinator for targets in the given list only,
calls `StoreCandidates()`, marks refreshed — does NOT call `Scheduler.Decide()`
or activate carriers), `CandidateRefreshResult` with `ReportLines()` for
observability. Three freshness layers remain explicitly distinct throughout:
`EndpointRegistry` (address-level reachability), `PathQualityStore` (RTT/jitter/loss
measurement freshness), `CandidateFreshnessStore` (coordinator-distribution freshness).
26 focused tests covering store lifecycle, trigger priority, deduplication, quality-stale
vs absent-quality distinction, architectural boundary enforcement. `go build ./...` and
`go test ./...` both pass.

### T-0026 — path change event history and audit basics
**status:** completed
**task file:** `agents/tasks/T-0026-path-change-event-history-and-audit-basics.md`

Implemented the first bounded path-change event history and audit baseline.
Added `EventHistory` and `Event` types in `internal/status/history.go` to record
explicit state changes like fallback-to-relay, recovery-to-direct, candidate-excluded,
and endpoint-stale. Added `RecentEvents` to `ScheduledEgressSummary` to expose this
history through the status API and `tlctl node status`. Modified `ScheduledEgressRuntime`
in `internal/node` to initialize and populate the bounded event history during path
activation (`ActivateScheduledEgress`), preserving the boundary between current state
and recent history. Added focused tests for history storage and event generation.
`go build ./...` and `go test ./...` both pass.

### T-0025 — operator path diagnostics and explainability basics
**status:** completed
**task file:** `agents/tasks/T-0025-operator-path-diagnostics-and-explainability-basics.md`

Implemented the first explicit operator-facing path diagnostics and
explainability baseline. Enhanced `internal/status` types to carry
`PathCandidateStatus` for all considered candidates, including
exclusion reasons, health degradation, endpoint freshness, and measured
quality. Updated `ScheduledEgressRuntime.Snapshot()` to include these
diagnostics for each association. Enhanced the scheduler to provide
detailed "why" reasons for burst vs stripe mode decisions, including
explicit mismatch reasons. Updated `tlctl node status` output to
surface these detailed "why chosen / why not chosen" diagnostics.
Added focused tests for the explainability logic. `go build ./...`
and `go test ./...` pass.

### T-0021 — control-plane transport security maturation
**status:** completed
**task file:** `agents/tasks/T-0021-control-plane-transport-security-maturation.md`

Implemented minimum useful maturation beyond bootstrap-only HTTP transport.
Added PKI generation primitives (`GenerateRootCA`, `GenerateCoordinatorIntermediate`,
`GenerateNodeCertificate`, `ParseCertificatePEM`, `ParseECPrivateKeyPEM`,
`ParseTLSCertificatePEM`, `NewCertPool`) in `internal/pki/issuance.go`.
Added TLS 1.3 config builders (`BuildCoordinatorTLSConfig`, `BuildNodeTLSConfig`)
in `internal/pki/tls.go`.
Added `SecureControlMode` type, `SecureTransportStatus`, `BootstrapOnlyTransportStatus()`,
`TLSMTCPFallbackTransportStatus()`, and `ReportLines()` in
`internal/controlplane/secure_transport.go` — making bootstrap-vs-secure distinction
explicit at the type level and in operator output.
Added `SecureControlListener` (TCP + TLS 1.3 mTLS) in
`internal/coordinator/secure_listener.go`, using the same application-layer handlers
as `BootstrapListener` but wrapping each listener with TLS. Includes `Run`, `ReportLines`,
`TransportStatus`, `BoundEndpoints`, `RuntimeSummaryLines`, `RegistrySnapshot`,
`AssociationSnapshot`.
Preserved bootstrap-only HTTP transport (`BootstrapListener`) unchanged; both types
coexist explicitly.
Added 14 focused tests across `internal/pki` and `internal/coordinator`.
Fixed TLS 1.3 client rejection test behavior: Go's TLS 1.3 defers the server's
certificate_required alert to the first client Read; tests updated to account for
this (check Read error, not DialWithDialer error).
`go build ./...` and `go test ./...` both pass.

### T-0020 — quality-aware path selection refinement
**status:** completed
**task file:** `agents/tasks/T-0020-quality-aware-path-selection-refinement.md`

Implemented the candidate refinement layer that bridges coordinator-distributed
path candidates, endpoint freshness, and live quality measurements into realistic
scheduler inputs. Added `RefinedCandidate`, `CandidateEndpointState`, and
`RefineCandidates` / `UsableSchedulerCandidates` in `internal/node/candidate_refinement.go`.
Wired `CandidateStore` and `EndpointRegistry` fields into `ScheduledEgressRuntime`;
`activateSingleScheduledEgress` now merges distributed (refined) and config-derived
candidates before `Scheduler.Decide()`. Endpoint freshness (Failed → exclude,
Stale → health degraded) and quality enrichment remain distinct inputs.
21 focused tests + 1 benchmark (~355 ns/op for 3 candidates), 5 integration tests
added to `scheduled_egress_test.go`. `go build ./...` and `go test ./...` both pass.

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

The next implementation tasks (from T-0031 audit findings, in priority order):
1. **Wire EffectivePolicy into runtime** — `FallbackConfig`, `MultiWANStickinessConfig`, and `ProbeSchedulerConfig` must read from `EffectivePolicy` (resolved per association) instead of hardcoded defaults. This is the biggest gap between T-0028's resolved-but-unconsumed policy fields and the live runtime.
2. **Start SecureControlListener in coordinator runtime** — `SecureControlListener` (TCP+TLS 1.3 mTLS) is fully implemented and tested but is not started in `transitloom-coordinator/main.go`. The coordinator still runs BootstrapListener only.
3. **Node enrollment flow** — certificate issuance; required before application-layer admission-token enforcement can be meaningful.
4. **Application-layer admission-token enforcement** on the secure control transport.
5. **QUIC+TLS 1.3 mTLS primary transport** (QUIC wrapper around existing PKI material).

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
