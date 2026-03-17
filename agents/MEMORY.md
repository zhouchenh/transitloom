# agents/MEMORY.md

## Purpose

This file stores **durable project memory** for Transitloom.

Unlike `agents/CONTEXT.md`, which captures the current working state, this file should record decisions, invariants, and lessons that should persist across tasks and sessions.

This file exists because coding agents are context-limited. If an important decision is not written down here or in `agents/memory/*.md`, future agents may forget it, rediscover it slowly, or accidentally violate it.

---

## What belongs here

Use this file for information that should remain useful across multiple tasks, such as:

- settled architectural decisions
- stable naming and terminology
- v1 invariants
- important rejected approaches
- persistent implementation rules
- project-level tradeoff decisions
- working assumptions that should not be re-debated casually

Do **not** use this file as a daily work log.  
Use `agents/CONTEXT.md`, `agents/TASKS.md`, `agents/tasks/*.md`, and `agents/logs/` for active-state and session-specific tracking.

---

## Durable project identity

- Project name is **Transitloom**
- Transitloom is a **coordinator-managed overlay mesh transport platform**
- Transitloom v1 is focused first on:
  - high-performance raw UDP service carriage
  - practical multi-WAN aggregation
  - WireGuard-over-mesh as the flagship documented use case
- Transitloom core should remain **generic**
- WireGuard is the **flagship use case**, not the sole product identity
- Transitloom is not meant to be a WireGuard protocol fork or WireGuard replacement

---

## Durable v1 product stance

- Transitloom v1 is **not** a full unconstrained service mesh
- Transitloom v1 is **not** a general arbitrary-hop raw UDP routed overlay
- Transitloom v1 is intentionally constrained so that the primary use case can work well
- The project should optimize for:
  - correctness
  - maintainability
  - practical transport value
  - end-to-end usefulness
- The project should not optimize for:
  - feature count
  - broad abstraction for its own sake
  - speculative future capability before the flagship path works

---

## Durable trust and admission decisions

These decisions are settled unless deliberately changed through specs.

- Node identity and current participation permission are separate
- A valid node certificate alone is **not** enough for normal participation
- Normal participation requires:
  - valid node certificate
  - valid admission token
- Revoke is **hard in operational effect**
- A revoked node must not successfully continue normal participation just because its identity certificate is still valid
- Root authority is not a normal node-facing coordinator target
- Root authority should not serve ordinary end-user coordinator traffic
- Per-coordinator intermediates under one root are the chosen PKI direction
- Routine node certificate renewal should not require the root to be online if coordinators already hold valid intermediates
- Relay-assisted renewal is allowed when policy permits it
- Trust-material file references resolve relative to `storage.data_dir` when config uses local relative paths
- Root bootstrap may treat missing root cert/key material as coherent only when both are absent and `trust.generate_key=true`
- Coordinator bootstrap requires a present root trust anchor; coordinator intermediate cert/key may both be absent as an explicit awaiting-issuance bootstrap state, but partial presence is invalid
- Node bootstrap keeps persisted identity references under `node_identity` and cached current admission-token references under `admission`; those paths also resolve relative to `storage.data_dir` when configured as local relative paths
- Node bootstrap treats `private_key present + certificate absent` as a coherent awaiting-certificate state for later enrollment work, but not as identity readiness
- A cached current admission token without ready node identity material is an invalid local bootstrap combination
- The cached current admission-token placeholder file is local JSON metadata (`token_id`, `node_id`, `issuer_coordinator_id`, `issued_at`, `expires_at`) and is only a readiness signal, not authoritative admission truth

---

## Durable data-plane decisions

These are among the most important v1 boundaries.

- Raw UDP is the primary v1 data-plane transport
- Raw UDP v1 requires **zero in-band overhead**
- v1 raw UDP data plane allows:
  - direct public paths
  - direct intranet/private paths
  - single coordinator relay hop
  - single node relay hop
- v1 raw UDP data plane does **not** allow arbitrary multi-hop forwarding
- Data-plane scheduling is **endpoint-owned**
- Relay nodes/coordinators must not become unconstrained end-to-end scheduling authorities
- v1 default scheduler is **weighted burst/flowlet-aware**
- Per-packet striping is allowed only when paths are **closely matched**
- Multi-WAN aggregation is still a primary practical target and should influence design choices
- `PathCandidate` is the scheduler's input type for deciding which paths to use; it is explicitly distinct from `RelayCandidate`, `ForwardingEntry`, `RelayForwardingEntry`, and `SchedulerDecision`
- `RelayCandidate` represents the relay intermediate itself; `PathCandidate` represents a resolved path that may use a relay; these must not be collapsed (spec/v1-object-model.md sections 16-17)
- `StripeMatchThresholds` defines the conservative gate for per-packet striping: RTT spread, jitter spread, loss spread, minimum confidence; all must be within thresholds and all paths must be measured for striping to activate
- `Scheduler.Decide()` is the scheduling authority; it runs at the source endpoint only; relays do not run Decide(); this preserves endpoint-owned scheduling
- `SchedulerDecision.Reason` is always non-empty; it is required for observability and must explain the decision in plain text
- `AssociationCounters` (atomic) tracks per-association decision/mode/striping counts; `SchedulerStatus` exposes a snapshot for operator review
- Relay paths are scored with a penalty (`relayPenalty = 10`) to preserve direct-path preference when quality is comparable; this matches the spec requirement that direct paths are preferred when they are healthy enough and competitively useful

---

## Durable control-plane decisions

- QUIC + TLS 1.3 mTLS is the primary control transport
- TCP + TLS 1.3 mTLS is the fallback control transport
- Control-plane semantics should stay logically consistent across QUIC and TCP
- Control plane is more flexible than data plane
- Security-sensitive global state should use ordered operations rather than weak overwrite semantics
- Partitioned coordinators may accept security-sensitive changes only as **pending proposals**
- Nodes must not treat pending proposals as committed truth
- All bootstrap transport timeout, retry, and body-limit constants live in `internal/controlplane/transport.go` and are named; magic literals are not acceptable
- `TransportErrorKind` + `TransportError` + `ClassifyTransportError` in `internal/controlplane/errors.go` are the canonical way to classify raw transport errors; callers must not parse error strings to make retry/skip decisions
- Retry is bounded: only `TransportErrorKindTimeout` is retryable (up to `BootstrapRetryMaxAttempts`); `TransportErrorKindConnectionRefused` and `TransportErrorKindContextCanceled` are never retried
- The coordinator bootstrap listener's HTTP server must have `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes`, and per-handler `http.MaxBytesReader` set; omitting these creates attack surface
- The current bootstrap control transport is HTTP without TLS; the `BootstrapProtocolVersion` and `BootstrapOnly` response fields explicitly declare that this is not the final QUIC+mTLS/TCP+TLS transport; future work must replace the transport without removing the semantic distinction
- `BootstrapEndpointAttempt.ErrorKind` records the normalized error kind for each failed attempt; report lines must include `kind=` for operator observability
- `SecureControlMode` in `internal/controlplane/secure_transport.go` is the explicit type for transport security mode; values: `bootstrap-only-http`, `tls-1.3-mtls-tcp-fallback`, `quic-tls-1.3-mtls-primary`; callers must never use raw strings
- `SecureTransportStatus` (with `Mode`, `Authenticated`, `Description`) must be used in operator-facing log output for any control listener or session; never omit the authentication state
- `BootstrapOnlyTransportStatus()` returns the explicit declaration for the bootstrap HTTP transport; every bootstrap-session log line should use it so operators cannot mistake the transport as authenticated
- `TLSMTCPFallbackTransportStatus()` returns the status for active TCP+TLS 1.3 mTLS; `Authenticated=true`
- `SecureControlListener` (in `internal/coordinator`) is the TCP+TLS 1.3 mTLS fallback control listener; it is a separate type from `BootstrapListener`; both exist explicitly in the codebase; the distinction is at the type level, not in a runtime flag
- `NewSecureControlListener` rejects a nil `*tls.Config` with a clear error; passing nil would silently create a plaintext listener indistinguishable from `BootstrapListener`
- `pki.BuildCoordinatorTLSConfig` is the canonical constructor for the coordinator's TLS config; it always enforces `MinVersion: tls.VersionTLS13` and `ClientAuth: tls.RequireAndVerifyClientCert`
- `pki.BuildNodeTLSConfig` is the canonical constructor for a node's TLS client config; always `MinVersion: tls.VersionTLS13`; `ServerName` must be set (not empty) for hostname-based coordinator deployments
- PKI generation primitives (`GenerateRootCA`, `GenerateCoordinatorIntermediate`, `GenerateNodeCertificate`) are in `internal/pki/issuance.go`; they produce PEM-encoded material in `RootCAMaterial`, `IntermediateMaterial`, `NodeCertMaterial`
- Root CA: `MaxPathLen=1` (one level of intermediates allowed), no ExtKeyUsage (root is not a transport cert)
- Coordinator intermediate: `MaxPathLen=0` / `MaxPathLenZero=true` (prevents sub-intermediates), no ExtKeyUsage restriction (restricting to ServerAuth only would break Go's x509 chain verification for ClientAuth node certs)
- Node cert: leaf (`IsCA=false`), `ExtKeyUsage: ClientAuth` (required for mTLS node-to-coordinator sessions); a valid node cert is necessary but NOT sufficient for participation — admission token still required
- Private keys are stored as PKCS#8 PEM (`PRIVATE KEY` header) via `x509.MarshalPKCS8PrivateKey`; SEC1 (`EC PRIVATE KEY`) is also parsed in `ParseECPrivateKeyPEM` for compatibility
- The bootstrap-only flag (`BootstrapOnly=true`) in session responses must remain true even over TLS 1.3 mTLS; transport authentication is separate from application-layer admission-token enforcement
- In Go's TLS 1.3 implementation, the server's `certificate_required` rejection alert surfaces on the **first Read** after the client-side handshake completes, not during `tls.DialWithDialer`; tests that expect rejection must do a Read after dial and check for a non-nil read error, not rely on a non-nil dial error
- Application-layer admission-token enforcement on `SecureControlListener` is NOT yet implemented (T-0021 scope); connections with valid certificates but invalid/missing tokens are not yet rejected at the application layer

---

## Durable service-model decisions

These distinctions are important and must not be casually collapsed.

- Service is not the same thing as service binding
- Service binding is not the same thing as local ingress binding
- Local target is not the same thing as local ingress
- Relay candidate is not the same thing as path candidate
- Discovery hints are not authoritative truth
- Config is not the same thing as distributed state
- Multiple services per node are supported
- Multiple WireGuard services per node are supported
- Stable local ingress bindings matter for the flagship use case
- WireGuard should remain generic in the core model
- Bootstrap-phase service registration stores requested local ingress intent separately from the service binding/local target; it does not allocate a `LocalIngressBinding`
- Bootstrap-only service registration does not imply authenticated service ownership, service discovery completeness, or association authorization
- Association creation is strictly distinct from service registration; a registered service does not automatically have associations
- Bootstrap-only association records are logical connectivity placeholders; they do not imply path selection, relay eligibility, forwarding-state installation, or that traffic can flow
- Association creation validates that both source and destination services are registered in the coordinator's service registry
- Self-associations (same node, same service) are rejected
- Node config carries optional `associations` entries with `source_service`, `destination_node`, `destination_service`; source service type is resolved from local services config; destination service type defaults to raw-udp for v1
- Direct raw UDP carriage is association-bound: a ForwardingEntry must be installed in the ForwardingTable before carriage can start; the DirectCarrier rejects carriage attempts for unknown associations
- The ForwardingEntry bridges control-plane association records to data-plane forwarding state; it is not the association itself but the installed forwarding context
- Local ingress (where app sends into mesh) and local target (where mesh delivers to service) are kept as separate `*net.UDPAddr` fields in the ForwardingEntry and must never be the same address
- `AssociationConfig` now carries an optional `direct_endpoint` field for bootstrap-only direct-path testing; in the full system, peer endpoints will come from coordinator-distributed path candidates
- `AssociationConfig` now carries an optional `mesh_listen_port` field for per-association inbound delivery; because zero in-band overhead means no association header, the association is identified by which mesh listener port received the packet
- `DirectPathRuntime` (in `internal/node`) combines a `ForwardingTable` and `DirectCarrier` into the minimum node-runtime integration needed for direct-path WireGuard-over-mesh
- Node startup (`cmd/transitloom-node/main.go`) now wires direct-path carriage into the bootstrap flow: after association creation, it builds activation inputs from config + coordinator results, activates direct paths, and stays running if carriage is active
- `AssociationConfig` now carries an optional `relay_endpoint` field for bootstrap-only relay-assisted egress; both `DirectEndpoint` and `RelayEndpoint` may be set simultaneously so that the scheduler can choose between them; `relay_endpoint` is validated as a valid `host:port` in `validateAssociation`
- Relay-assisted carriage uses a per-association listen port on the coordinator to identify associations without in-band headers; this is the only mechanism compatible with zero in-band overhead at the relay hop
- `RelayForwardingEntry` (coordinator relay) and `RelayEgressEntry` (source node egress) are separate incompatible types from `ForwardingEntry` (direct carriage); they must never be conflated
- `RelayForwardingEntry` has only a `DestMeshAddr` terminal field and no next-relay or chain field; this structurally prevents relay chains and enforces the v1 single-hop constraint without runtime checks
- Destination-side delivery for relay-assisted carriage is handled by the existing `DirectCarrier.StartDelivery` with an existing `ForwardingEntry`; the delivery path is identical for direct and relay-assisted traffic; no separate "relay delivery" type was created
- `RelayPathRuntime` (in `internal/node`) manages source-node relay egress: `RelayEgressTable` + `RelayEgressCarrier`
- `CoordinatorRelayRuntime` (in `internal/coordinator`) manages coordinator relay forwarding: `RelayForwardingTable` + `RelayCarrier`
- `ScheduledEgressRuntime` (in `internal/node`) is the scheduler-guided egress runtime: `Scheduler` + `DirectPathRuntime` + `RelayPathRuntime`; this is the correct integration type for node startup, replacing direct `DirectPathRuntime` alone
- `ActivateScheduledEgress` calls `Scheduler.Decide()` at the egress decision point for each association; it activates `DirectCarrier` for direct-class paths and `RelayEgressCarrier` for relay-class paths; the two carriers remain architecturally distinct
- `ScheduledEgressActivation.CarrierActivated` ("direct"/"relay"/"none") must always be aligned with `Decision.Mode` and `Decision.ChosenPaths[0].Class`; an operator can verify this alignment without reading code
- Per-packet striping at the carrier level is not yet implemented; when `ModePerPacketStripe` is decided, `ActivateScheduledEgress` activates the best single path and records the mode so the gap is observable; this is intentional and not a bug
- `cmd/transitloom-node/main.go` uses `BuildScheduledActivationInputs` + `ActivateScheduledEgress`; do not revert to `BuildAssociationActivationInputs` + `ActivateDirectPaths` for the scheduler-integrated path

---

## Durable external endpoint and DNAT-aware reachability decisions

These decisions are settled unless deliberately changed through specs.

- `ExternalEndpoint` in `internal/transport` is the explicit type for externally reachable endpoints; it must not be confused with local target, local ingress, or mesh/runtime port
- `EndpointSource` has four values in precedence order: configured > router-discovered > probe-discovered > coordinator-observed; configured is the highest-confidence source
- `VerificationState` has four values: unverified (usable, represents operator intent), verified (confirmed reachable), stale (must revalidate), failed (must revalidate)
- Stale and failed endpoints must not be used for direct-path decisions without revalidation; endpoint knowledge must not be treated as timeless truth
- Endpoint knowledge must become stale after unhealthy/down events; this is enforced via `MarkStale()` + `IsUsable()` contract
- `ExternalEndpoint.Port` (external) and `ExternalEndpoint.LocalPort` (local mesh listener) must never be collapsed into one undifferentiated field; in DNAT deployments they differ and conflating them silently breaks inbound reachability
- `HasDNAT()` and `EffectiveLocalPort()` are the correct accessors for DNAT-aware logic; callers should not inspect `Port` and `LocalPort` directly
- `ExternalEndpointConfig` and `ForwardedPortConfig` are in `internal/config/common.go`; `NodeConfig` carries `ExternalEndpoint ExternalEndpointConfig`
- `RouterDiscoveryHint` and `ProbeResult` are placeholder types in `internal/transport`; they reserve semantic space for future UPnP/PCP/NAT-PMP and probe-verification code so those implementations use the correct source categories rather than overloading local service binding fields
- Full UPnP/PCP/NAT-PMP implementation and blind full-range port probing are explicitly out of scope; targeted probing and router-protocol discovery are future work
- `NewConfiguredEndpoint()` is the canonical constructor for operator-configured external endpoints; it sets source=configured, verification=unverified
- Coordinator observation of a source address (public IP) is insufficient to infer usable inbound ports; the coordinator cannot observe DNAT rules on the router

## Durable targeted probing and freshness revalidation decisions

These decisions are settled unless deliberately changed through specs.

- The Transitloom probe wire protocol is 13 bytes: 4-byte magic "TLPR" + 1 type byte + 8-byte nonce (little-endian). `ProbeTypeRequest = 0x01`, `ProbeTypeResponse = 0x02`. The responder echoes the nonce with ProbeTypeResponse.
- `IsProbeDatagram()` in `internal/transport` is the canonical way to identify probe datagrams for multiplexed listeners.
- `CandidateReason` has four values: configured, router-discovered, coordinator-observed, previously-verified. Every `ProbeCandidate` must carry a reason to enforce targeted-first discipline.
- `BuildCandidatesFromEndpoints()` must never return more candidates than the input endpoint count; it never invents new host:port combinations. This is the structural guard against blind port scanning.
- `BuildCoordinatorObservedCandidates()` takes a coordinator-observed IP + explicitly configured port list. It must never be called with a speculative port range.
- `UDPProbeExecutor` is the operational probe executor; it uses crypto/rand nonces, a connected UDP socket, and respects context deadlines.
- `ProbeResponder` and `HandleProbeDatagram()` are for the remote side; existing mesh listeners should integrate `HandleProbeDatagram()` rather than running a separate responder socket.
- `EndpointRegistry` in `internal/transport` is the thread-safe runtime store for ExternalEndpoints; it is the authoritative source for usable/stale/failed endpoint knowledge during node runtime.
- `EndpointRegistry.MarkAllStale()` is the correct way to bulk-invalidate endpoint knowledge after path-down/IP-change events; it leaves already-stale/failed endpoints unchanged.
- `EndpointRegistry.SelectForRevalidation()` returns stale+failed endpoints; `SelectForInitialVerification()` returns unverified endpoints. These are the two targeted candidate selection points.
- `EndpointRegistry.ApplyProbeResult()` updates all matching host:port endpoints; multiple source records for the same address are all updated.
- `CoordinatorProbeRequest` in `internal/controlplane` preserves the DNAT distinction: `TargetPort` (external, on router) and `EffectiveLocalPort` (local mesh listener after DNAT) must never be collapsed.
- `EndpointFreshnessSummary` in `internal/status` is the operator-facing freshness surface; it imports `internal/transport` (leaf package, no cycle risk).
- `ReportLines()` on `EndpointFreshnessSummary` labels stale/failed endpoints as `[needs-revalidation]` explicitly so operators know which endpoints cannot be used for direct-path decisions.
- Blind full-range probing (0–65535) is explicitly not implemented and must not become the default. All probe candidates must trace to a known deliberate source.
- Full UPnP/PCP/NAT-PMP and STUN/TURN/ICE are explicitly out of scope for T-0017 and remain future work.

---

## Durable distributed path-candidate decisions

These decisions are settled unless deliberately changed through specs.

- `DistributedPathCandidate` (in `internal/controlplane`) is the wire format for coordinator-distributed path candidates; it is explicitly distinct from `scheduler.PathCandidate` (local runtime scheduler input), `ForwardingEntry`, `RelayForwardingEntry`, and `SchedulerDecision`
- `DistributedPathCandidate` carries both a `Class` field (`direct-public`, `direct-intranet`, `coordinator-relay`, `node-relay`) AND an explicit `IsRelayAssisted bool` flag; both must remain consistent; using both is a deliberate belt-and-suspenders architectural enforcement, not redundancy
- `PathCandidatePath = "/v1/bootstrap/path-candidates"` is the canonical coordinator endpoint path for path-candidate distribution
- `PathCandidateSet` is association-bound: every candidate in a set must carry the same `AssociationID` as the set itself; this is enforced by `PathCandidateSet.Validate()`
- The coordinator currently generates only relay candidates (from `CoordinatorRelayConfig.ListenEndpoints` when `DataEnabled && !DrainMode`); direct candidates require node endpoint advertisement (future work); the absence is always explained via the `Notes` field on `PathCandidateSet`
- When `DrainMode` is true, the coordinator must not generate relay candidates even if `DataEnabled` is true; the drain state must be noted in `PathCandidateSet.Notes`
- `CandidateStore` (in `internal/node`) is the node-side store for coordinator-distributed candidates; it holds only `DistributedPathCandidate` values — not scheduler decisions, not forwarding entries, not activation state
- `CandidateStore` provides copy isolation: `Store()` copies the input slice, `Lookup()` returns a copy; mutating the caller's slice does not affect stored state and vice versa
- `CandidateStore.Snapshot()` returns all association sets sorted by `AssociationID`; the order must be deterministic
- `StoreCandidates()` is deliberately separate from carrier activation; it updates only the `CandidateStore` and does not touch scheduler state, forwarding entries, or carriers
- `FetchPathCandidates()` requires a prior accepted bootstrap session; the fetch/store boundary is explicit: fetch returns the raw response, the caller calls `StoreCandidates()` as a separate step
- Candidate presence is NOT proof of runtime success; `IsUsable()` means `RemoteEndpoint` is set — it means the coordinator provided an endpoint to try, not that a `ForwardingEntry` is installed or traffic will flow
- `PathCandidateResponse.BootstrapOnly = true` is always set; the response format explicitly declares non-final semantics matching the bootstrap-only control transport

---

## Durable live path quality measurement decisions

These decisions are settled unless deliberately changed through specs.

- `PathQualityStore` in `internal/scheduler` is the live measurement input layer; it is NOT scheduling, NOT candidate existence, NOT applied runtime behavior
- `PathQualityStore.RecordProbeResult` uses EWMA for RTT (alpha=0.125), jitter (alpha=0.25 deviation), and loss fraction (alpha=0.1); confidence increases by 0.1 per successful probe, decreases by 0.2 per failure, clamped to [0,1]
- `PathQualityStore.Update` replaces stored quality directly (no EWMA blending); for use with passive observation or external measurement sources
- `PathQualityStore.FreshQuality` returns (zero, false) when the measurement is older than `maxAge`; this is the explicit staleness contract — stale measurements must not silently appear as current
- `DefaultQualityMaxAge` = 60s; measurements older than this are treated as stale by the scheduler
- `PathQualityStore.ApplyCandidates` returns a new slice with Quality fields enriched from fresh measurements; only Quality is updated, not ID/Class/Health/AssociationID
- `ScheduledEgressRuntime.QualityStore` is set to a new `PathQualityStore` by default in `NewScheduledEgressRuntime`; applied in `activateSingleScheduledEgress` before `Scheduler.Decide()`
- `ScheduledEgressRuntime.QualitySnapshot()` returns `PathQualitySummary` (from `internal/status`); this is a separate method from `Snapshot()` because measurement inputs and applied carrier behavior are different concerns
- `PathQualitySummary` + `MakePathQualitySummary` in `internal/status` provides operator-visible freshness reporting; stale entries labeled `[stale/needs-remeasurement]`
- Active probe scheduling loop (calling `RecordProbeResult` on a timer) is NOT yet implemented; the store accepts probe results but callers must drive the probe lifecycle
- Measurement state (PathQualityStore) is intentionally NOT the same as `EndpointRegistry` (T-0017); EndpointRegistry tracks address/port reachability for external endpoints; PathQualityStore tracks per-path scheduler quality inputs for association-bound candidates

---

## Durable candidate refinement decisions

These decisions are settled unless deliberately changed through specs.

- `RefinedCandidate` in `internal/node` is the intermediate form between `DistributedPathCandidate` (coordinator wire format) and `scheduler.PathCandidate` (local runtime scheduler input); it is always produced by `RefineCandidates()` and consumed by `UsableSchedulerCandidates()` before `Scheduler.Decide()`
- `CandidateEndpointState` has four values: `Unknown` (no registry entry), `Usable` (unverified or verified), `Stale` (was valid, needs revalidation), `Failed` (probe confirmed unreachable); these are distinct from `transport.VerificationState` — conversion is explicit via `verificationToEndpointState()`
- `RefineCandidates()` applies a strict three-step pipeline per candidate: (1) usability check — missing `RemoteEndpoint` → excluded; (2) endpoint-freshness check — Failed → excluded, Stale → `HealthStateDegraded` but Usable=true; (3) quality enrichment — `FreshQuality()` applied when available; endpoint state and quality enrichment are always kept as distinct inputs
- Endpoint freshness and measured path quality must never be collapsed into one score; `EndpointState` (from `EndpointRegistry`) reflects address-level reachability; `QualityFresh`/`Candidate.Quality` (from `PathQualityStore`) reflect RTT/jitter/loss; the two inform refinement through separate steps
- Stale endpoints are degraded (not excluded): stale means the endpoint was valid but needs revalidation; stale candidates remain usable as last-resort fallbacks while revalidation is pending; failed endpoints are excluded because they are confirmed unreachable
- `checkEndpointFreshness()` returns the most severe state across all registry entries matching a host:port; severity order: Failed (3) > Stale (2) > Usable (1) > Unknown (0); most-severe wins to prevent a usable record from hiding a stale/failed one for the same address
- Quality enrichment is applied inside `RefineCandidates()` (not by a separate `ApplyCandidates` call on the result); this makes `QualityFresh` visible on `RefinedCandidate` for diagnostics; callers must not call `ApplyCandidates` on `UsableSchedulerCandidates()` output — it would double-apply quality or zero out already-enriched fields
- `ScheduledEgressRuntime` carries `CandidateStore *CandidateStore` and `EndpointRegistry *transport.EndpointRegistry`; both are nil by default (no regression when unused); `activateSingleScheduledEgress` merges refined distributed candidates with config-derived candidates before `Scheduler.Decide()`
- Config-derived candidates have quality applied via `runtime.QualityStore.ApplyCandidates(input.PathCandidates)` separately from distributed candidates; the copy target is `candidates[:len(input.PathCandidates)]` to avoid touching the distributed suffix of the merged slice
- `RefinedCandidate.ReportLine()` produces a single human-readable line per candidate; excluded candidates show `[excluded]: <reason>`; usable candidates show class, endpoint state, quality label, and optional degraded reason — this is the primary operator-facing diagnostic surface for refinement behavior
- `convertDistributedClass()` maps `DistributedPathCandidateClass` → `scheduler.PathClass` with explicit case handling; it must never collapse relay and direct classes

---

## Durable candidate refresh and revalidation decisions

These decisions are settled unless deliberately changed through specs.

- `CandidateFreshnessStore` in `internal/node` is the per-association store for coordinator-distribution freshness; it is explicitly distinct from `EndpointRegistry` (address-level reachability) and `PathQualityStore` (RTT/jitter/loss measurement freshness); the three freshness layers must never be collapsed
- `CandidateRefreshTrigger` has six values: `endpoint-stale`, `endpoint-failed`, `quality-stale`, `candidate-expired`, `path-unhealthy`, `explicit`; every refresh target must carry the trigger that caused it
- `DefaultCandidateMaxAge` = 5 minutes; candidates older than this without an explicit refresh are treated as stale by `FreshnessState()`
- `CandidateFreshnessState` has three values: `fresh` (recently refreshed), `stale` (explicitly marked or age-expired), `unknown` (never tracked or never refreshed); stale state must not silently look current
- `MarkStale()` preserves the first trigger that fires (root-cause visibility); subsequent `MarkStale()` calls do not overwrite `StaleReason` once a stale record exists
- `FreshnessState()` performs lazy age-based staleness check (`time.Since(r.LastRefreshedAt) > s.maxAge`); no background goroutines are needed; the check is performed on every read
- `SelectCandidateRefreshTargets()` scans three freshness layers in priority order: (1) freshness store stale/expired, (2) endpoint registry stale/failed endpoints matching known candidates, (3) quality store stale measurements for candidate IDs; a `seen` map deduplicates by association ID so the first (highest-priority) trigger wins
- Quality-stale trigger fires only when `len(staleQualityIDs) > 0` AND a candidate ID is in that set; absent quality (never measured) does NOT trigger a quality-stale refresh; this distinction prevents spurious re-fetches for paths that simply have no measurements yet
- `ExecuteCandidateRefresh()` is bounded: it only processes targets in the provided list, never scans the whole network, and skips all targets if the bootstrap session is not accepted; skipped targets record an explicit skip reason in `CandidateRefreshDetail`
- `ExecuteCandidateRefresh()` calls `FetchPathCandidates()` + `StoreCandidates()` per target; it never calls `Scheduler.Decide()` or activates carriers; refresh updates `CandidateStore` only — the scheduler decision point is deliberately separate
- `CandidateRefreshResult` carries `Attempted`, `Refreshed`, `Failed`, `Skipped` counts plus per-association `Details`; `ReportLines()` surfaces this for operator review
- Active probe scheduling loop is now implemented in `internal/node/probe_scheduler.go` (T-0029): `RunProbeLoop()` drives `SelectProbeTargets()` + `ExecuteProbeRound()` on a ticker; callers start it as a goroutine for background operation

---

## Durable active probe scheduling decisions

These decisions are settled unless deliberately changed through specs.

- `ProbeSchedulerConfig` in `internal/node` configures the bounded active probe loop: `ProbeInterval` (default 30s) and `MaxTargetsPerRound` (default 10); both bounds are explicit and structural — they prevent uncontrolled fan-out
- `ProbeTarget` is the ephemeral probe work item: `Host`, `Port`, `Reason` (targeted-first discipline), `PathIDs` (quality-store linkage); it bridges `EndpointRegistry` (address reachability) and `PathQualityStore` (scheduling quality inputs)
- `SelectProbeTargets()` builds a bounded targeted probe candidate list from `EndpointRegistry`: unverified endpoints first (priority), then stale/failed (revalidation); deduplicates by host:port; enforces maxTargets; uses `BuildCandidatesFromEndpoints` to preserve targeted-first discipline (no port-range guessing)
- `ExecuteProbeRound()` wires probe results into two distinct layers: (1) `registry.ApplyProbeResult()` — endpoint freshness/verification state; (2) `qualityStore.RecordProbeResult()` per PathID — RTT/jitter/loss for scheduling quality; the two layers serve different downstream consumers and must not be merged
- Absent measurement vs failed measurement is explicit: empty `PathIDs` → endpoint freshness updated but quality store not touched (path is "unmeasured", not "failed"); non-empty `PathIDs` with `Reachable=false` → quality store records a failure (confidence decreases, loss rises) — "measured and failed"
- `BuildPathIDMap()` builds "host:port" → `[]pathID` linkage from two sources: (1) config-derived inputs (`assocID + ":direct"` for direct endpoints); (2) coordinator-distributed candidates (`CandidateID` for each `RemoteEndpoint`); both are included when they reference the same host:port
- `RunProbeLoop()` is the bounded ticker-driven goroutine helper: fires immediately on start (cold-start probe), then every `ProbeInterval`; `onRound` callback provides observability; context cancellation stops cleanly; the loop does NOT call `Scheduler.Decide()` or activate carriers — it only drives probe execution and wires results
- Live runtime lifecycle integration for the probe loop is handled by `ScheduledEgressRuntime.StartProbeLoop()` (T-0030): startup is prerequisite-gated, shutdown is tied to runtime context cancellation, and runtime status is explicit with states `disabled` / `blocked` / `active` / `waiting-prerequisites` / `stopped`
- `BuildDistributedProbeTargets()` builds probe targets from `CandidateStore` distributed candidates that have a known `RemoteEndpoint`; distinct from `SelectProbeTargets()` which reads `EndpointRegistry`; both are targeted-first
- Probe scheduling is explicitly separate from: hysteresis/switching policy (`DirectRelayFallbackPolicy`, T-0024 scope), candidate refresh automation (`ExecuteCandidateRefresh`), and scheduler path decisions (`Scheduler.Decide`)
- `ProbeRoundResult.ReportLines()` is the primary operator-facing observability surface for one probe round; it shows selected/reachable/unreachable/errors counts and per-target details including RTT and quality path IDs

---

## Durable multi-WAN stickiness and hysteresis decisions

These decisions are settled unless deliberately changed through specs.

- `MultiWANStickinessPolicy` in `internal/node/stickiness_policy.go` is the per-association state machine for path-switching stability; it is explicitly distinct from `DirectRelayFallbackPolicy` (T-0023) which governs direct↔relay class transitions; stickiness governs within-class and cross-class quality-based switching when multiple eligible candidates exist
- The stickiness layer sits between the fallback filter and `Scheduler.Decide()` in the decision stack: candidate generation → refinement → fallback policy → stickiness policy → scheduler → carrier; this order is intentional and must not be collapsed
- `AdjustCandidates()` returns a filtered candidate list: current-only when suppressing (forces scheduler to pick current path via `ModeSinglePath`), all candidates when allowing; the scheduler sees either a single-element or full list and picks naturally — the stickiness policy does not modify scores
- `RecordSelection()` must be called after `Scheduler.Decide()` with the chosen path ID; it updates `currentID`, starts the hold-down timer when a switch occurs, and returns a `StickinessEval` with `SwitchOccurred=true` on a switch
- Score comparison uses `scheduler.ScoreCandidate()` — the single authoritative scoring formula; the stickiness policy must never reimplement or duplicate the scoring logic; `ScoreCandidate()` was exported from the scheduler package specifically for this use
- `StickinessThreshold` default=3: an alternative path must score strictly more than 3 points above the current path before a switch is allowed; this blocks trivial oscillation on measurement noise while permitting genuine quality improvements; a value of 0 disables threshold checking entirely
- `HoldDownDuration` default=30s: after any switch, all further switches are suppressed for 30 seconds regardless of quality; hold-down prevents rapid re-switching after a change; the current path must be absent from the candidate list to bypass hold-down
- `StickinessEval.Reason` is always non-empty; switch suppression must never be opaque; every evaluation produces a human-readable explanation including: whether hold-down was active, elapsed/total hold-down time, score comparison, and whether suppression occurred
- `AssociationStickinessStore` manages per-association `MultiWANStickinessPolicy` instances created lazily; `Remove(associationID)` reclaims memory when an association is torn down; `Snapshot()` returns `[]StickinessSnapshot` for operator observability
- `NewScheduledEgressRuntime` initializes `StickinessStore: NewAssociationStickinessStore(DefaultMultiWANStickinessConfig())` automatically; a nil `StickinessStore` is allowed (bypasses stickiness policy entirely) and must not panic
- `StickinessReason`, `SwitchOccurred`, `HoldDownActive` are surfaced on both `ScheduledEgressActivation` (runtime) and `status.ScheduledEgressEntry` (observability); `ReportLines()` emits a `stickiness` line with `[SWITCH]` and `[hold-down]` labels when present
- `PathGroup string` field on `scheduler.PathCandidate` reserves semantic space for uplink/WAN group identity (e.g., "wan0", "fiber", "lte-backup"); group-based scheduling policy is future work; the field must not be used for per-packet routing decisions in v1
- The score-comparison approach (exposing `ScoreCandidate()`) was chosen over a `HysteresisBonus` field on `PathCandidate` because any positive bonus would cause the current path to always win when it and the alternative both score 100, making stickiness effectively permanent rather than threshold-bounded
- The stickiness policy does not generate, exclude, or re-order candidates in the scheduling sense; it only removes non-current candidates when suppressing; this preserves the existing candidate pipeline boundaries

---

## Durable WireGuard-over-mesh decisions

- WireGuard is the flagship documented v1 use case
- Transitloom should support WireGuard without WireGuard protocol changes
- A WireGuard service maps to a generic UDP-carried service in the Transitloom model
- The local WireGuard `ListenPort` is the local target for inbound delivery
- Transitloom local ingress ports used as WireGuard peer endpoints are separate from the local target
- Transitloom should prefer mesh-owned liveness behavior
- WireGuard `PersistentKeepalive` may be tolerated but should not be the primary overlay-liveness dependency

---

## Durable observability decisions

- `internal/status` is the canonical package for runtime status summaries; it contains narrow, explicit types, not a generic telemetry framework
- `BootstrapSummary` reflects local material coherence only; it must never be labeled as coordinator authorization
- `ServiceRegistrySummary` and `AssociationStoreSummary` expose coordinator state; bootstrap-only records must always be labeled as placeholders, not as final authenticated state
- `ScheduledEgressSummary` carries both `SchedulerMode` (computed decision) and `CarrierActivated` (what actually started); these two fields must always appear together so operators can verify alignment; misalignment between them is intentional/observable (e.g., per-packet-stripe decided but direct carrier activated because multi-carrier striping not yet implemented)
- `ScheduledEgressRuntime.Snapshot()` provides the live observability surface for applied carrier behavior; it merges stored activation results with `DirectCarrier.IngressStats` / `RelayEgressCarrier.EgressStats` counters
- `BootstrapListener.RuntimeSummaryLines()` exposes the coordinator's current service registry and association state for logging; service registration and association are kept as separate summary sections
- Status package imports `internal/service` (leaf) only; it must not import `internal/node` or `internal/coordinator` to avoid cycles; callers convert their internal types and pass data to the status constructors
- ReportLines output must label bootstrap-only / placeholder / local-readiness-only state explicitly — not doing so creates misleading operator output

---

## Durable implementation-order decisions

Transitloom should be implemented in this order unless a task explicitly justifies a deviation:

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

This order is important. It should not be casually ignored.

---

## Durable first milestones

The first meaningful milestone is:

**two admitted nodes, one coordinator, one UDP service per node, one legal association, direct raw UDP carriage working**

The first flagship validation milestone after that is:

**WireGuard-over-mesh over a direct path, using Transitloom local ingress ports**

These milestones should continue to shape implementation choices.

---

## Durable rejected or constrained directions

These are not necessarily rejected forever, but they are outside v1 or should not be implemented casually.

- arbitrary multi-hop raw UDP data forwarding
- treating certificates as sufficient proof of current participation
- making root authority a normal end-user coordinator target
- making WireGuard-specific semantics foundational in the core model
- allowing relays to independently reshape end-to-end scheduling policy in v1
- treating discovery as authorization
- broad service-mesh ambition ahead of the flagship transport path
- speculative generic encrypted data plane as though it already exists
- speculative generic TCP data plane as though it already exists

---

## Durable naming and structure decisions

- Root workspace for coding agents is:
  - `AGENTS.md`
  - `agents/`
- The agent workspace directory name is **`agents/`**, not `agent/`
- When content itself contains triple backticks, copy-paste markdown blocks should use `~~~markdown` outer fences instead of triple backticks
- Specs live under `spec/`
- Human-facing docs live under `docs/`
- Agent operational context lives under `agents/`

---

## Durable coding-agent workflow decisions

- Agents must read the required agent workspace files before substantial work
- Agents must treat the `agents/` workspace as persistent operational memory, not optional documentation
- Agents should update `agents/CONTEXT.md`, `agents/MEMORY.md`, `agents/TASKS.md`, and related files whenever meaningful progress or learning occurs
- Small unrecorded facts are dangerous because context-limited agents may forget them later
- If a future agent would benefit from knowing it, and it is not already clearly captured, it should be written down
- Agents should follow `agents/CODING.md` for coding behavior
- Agents should follow `agents/REPORTING.md` for end-of-run reporting

---

## Durable repository workflow policy

Transitloom uses a staged repository workflow policy.

### Before v1.0.0
Before the first stable `v1.0.0` release, the repository uses **Model A**:

- agents may commit directly
- agents may push directly to `master`

but only when:
- the change is coherent and task-aligned
- verification has been run appropriately
- reporting is complete
- relevant `agents/` files have been updated
- the commit message accurately reflects the work
- the repo is not being left in a confusing or partially broken state unless the checkpoint is intentional and clearly documented

### At and after v1.0.0
Starting at `v1.0.0`, the repository must switch to **Model B**:

- agents work on task/feature branches
- agents push branches, not direct pushes to `master`
- integration proceeds through review workflow

Agents must not assume the pre-`v1.0.0` direct-push policy still applies after that milestone.

### Durable rule
This transition from Model A to Model B is a deliberate project policy, not a casual preference.

---

## Durable decision philosophy

When tradeoffs are unclear, Transitloom should generally prefer:

- performance over unnecessary routing freedom
- clarity over cleverness
- explicit state over hidden magic
- maintainability over abstraction vanity
- generic core over protocol-specific hacks
- real operational control over optimistic assumptions
- end-to-end usefulness over local optimization

This is part of the project’s durable philosophy and should guide implementation decisions.

---

## What to add here later

Add to this file when a decision becomes durable enough that future agents should not have to rediscover or renegotiate it.

Good candidates:
- stable naming choices
- architectural decisions that survived review
- implementation constraints that keep recurring
- explicit “do not do this again” lessons
- settled defaults and boundaries

If a decision is still temporary or task-specific, put it in `agents/CONTEXT.md` or a task file instead.

---
