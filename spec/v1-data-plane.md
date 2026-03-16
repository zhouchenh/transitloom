# Transitloom v1 Data Plane Specification

## Status

Draft

This document specifies the Transitloom v1 data plane. It defines the raw UDP transport model, forwarding model, path classes, relay constraints, association state, scheduling behavior, health inputs, and WireGuard-over-mesh carriage assumptions relevant to v1.

This document focuses on the **data plane**, not the control-plane trust and coordination protocol.

---

## 1. Purpose

The Transitloom data plane is responsible for carrying service traffic across the overlay mesh once the control plane has:

- authenticated participants
- enforced admission state
- distributed service and association context
- distributed route and relay eligibility
- installed the state required for legal forwarding

Transitloom v1 data plane is primarily optimized for:

- high-performance raw UDP carriage
- practical multi-WAN bandwidth aggregation
- stable behavior across direct and relayed paths
- zero in-band overhead for raw UDP traffic
- WireGuard-over-mesh as the flagship workload

---

## 2. v1 design goals

### 2.1 Primary goals

- Carry raw UDP traffic across the mesh with zero in-band overhead
- Support practical multi-WAN aggregation for real deployments
- Support direct, intranet, and single-relay-hop data forwarding
- Keep scheduling endpoint-owned
- Preserve predictable performance under mixed path conditions
- Support relay fallback where useful without turning v1 into a general arbitrary-hop routed data mesh
- Work well for WireGuard carriage without changing WireGuard itself

### 2.2 Secondary goals

- Leave room for future encrypted UDP carriage
- Leave room for future TCP carriage
- Keep the service model generic rather than protocol-specific
- Support future extension of relay/path classes without redesigning the core association model

---

## 3. Non-goals for v1

Transitloom v1 data plane does not aim to provide:

- arbitrary multi-hop raw UDP forwarding
- general-purpose data-plane routing across unconstrained relay chains
- zero-copy or kernel-bypass optimization as a requirement for v1
- in-band raw UDP shim headers for v1 zero-overhead mode
- generic encrypted data carriage in v1
- generic TCP data carriage in v1
- unconstrained hop-local route/scheduling autonomy that breaks endpoint-owned traffic policy

---

## 4. Transport focus

## 4.1 Primary transport

The Transitloom v1 data plane focuses on **raw UDP**.

This is the primary and required service-carriage mode for v1.

## 4.2 Future transport modes

Transitloom is expected to support additional transport modes later, but they are outside the v1 data-plane scope:

- encrypted UDP carriage
- TCP carriage
- other service-specific transport profiles

### v1 requirement
The v1 raw UDP path must stand on its own and must not depend on future encrypted/TCP modes for correctness.

---

## 5. Core data-plane principles

Transitloom v1 data plane is built on these principles:

- **zero in-band overhead for raw UDP**
- **endpoint-owned scheduling**
- **direct paths preferred**
- **single relay hop maximum**
- **practical aggregation over theoretical routing generality**
- **stateful forwarding**
- **generic service carriage**
- **control-plane-installed legality**

---

## 6. Zero-overhead raw UDP model

## 6.1 Requirement

Transitloom v1 requires **zero in-band overhead** for raw UDP data-plane transport.

This means Transitloom must not add raw-UDP-visible shim headers inside the carried packet payload path for v1 zero-overhead mode.

## 6.2 Consequence

Because Transitloom must not add in-band per-packet metadata for raw UDP v1, forwarding identity must come from state and binding context, not packet headers.

That means the data plane must rely on:

- association state
- local listener identity
- configured local service bindings
- path binding state
- relay state
- control-plane-installed forwarding context

## 6.3 Forwarding implication

Transitloom raw UDP forwarding is therefore **stateful**.

Receivers and relays must know enough state to answer:

- what association this packet belongs to
- what local service it targets
- what path or relay context it arrived on
- what forwarding behavior is legal for it

without relying on in-band Transitloom packet headers.

---

## 7. Data-plane hop model

## 7.1 Allowed v1 data-plane path classes

A v1 data-plane association may use:

- **direct public path**
- **direct intranet/private path**
- **single coordinator relay hop**
- **single node relay hop**

## 7.2 Maximum data-plane relay depth

Transitloom v1 limits raw UDP data-plane forwarding to:

- direct
- or one relay hop only

### Examples allowed
- node A -> node B
- node A -> coordinator relay -> node B
- node A -> relay node -> node B

### Examples not allowed in v1
- node A -> relay node -> relay node -> node B
- node A -> coordinator relay -> relay node -> node B
- arbitrary data-plane route chains

## 7.3 Why this exists

This limit exists because Transitloom v1 prioritizes:

- multi-WAN aggregation quality
- scheduling predictability
- reduced reordering amplification
- simpler observability
- simpler legality and relay-state handling

over maximum routing flexibility.

---

## 8. Direct vs relayed data paths

## 8.1 Direct path preference

Transitloom v1 should prefer direct paths when they are:

- legal
- healthy enough
- competitively useful relative to relay paths

Direct includes both public direct and intranet/private direct paths.

## 8.2 Relay usage

Relay paths are valid when:

- direct paths are unavailable
- direct paths are clearly worse
- policy requires or permits relay usage
- relay is operationally useful for continuity or reachability

## 8.3 Relay classes

Transitloom v1 recognizes at least these relay classes:

- **coordinator relay**
- **node relay**

The data plane may use either class according to policy and endpoint scheduling logic.

## 8.4 Direct vs relay is a scored choice

Transitloom should not treat relay purely as a binary emergency-only mechanism.

Instead, eligible direct and relayed paths should be compared using policy-aware scoring.

That said, v1 should remain conservative and operationally predictable rather than aggressively switching without hysteresis.

---

## 9. Service and association model in the data plane

## 9.1 Services

A service is the local endpoint that Transitloom carries over the mesh.

For v1 data plane, the primary service type is raw UDP.

## 9.2 Associations

An association is the data-plane context that makes forwarding legal and meaningful.

The data plane must not invent raw forwarding behavior outside association context.

A data-plane association defines at least:

- source service
- destination service
- legal path classes
- legal relay classes
- scheduling policy
- health policy
- metering relevance
- local ingress/egress binding state

## 9.3 Bidirectional behavior

A practical association may behave bidirectionally, but explicit state on each endpoint must remain clear.

Transitloom should not rely on vague “session learned from first packet” behavior alone for correctness.

---

## 10. Local port and service-binding model

## 10.1 Local service target

Each service has a local target endpoint on a node.

For raw UDP services this is typically a local UDP destination such as:

- `127.0.0.1:<port>`
- `[::1]:<port>`
- a local non-loopback UDP bind, where policy allows

## 10.2 Stable local ingress bindings

Transitloom v1 should provide stable local ingress bindings for application-facing use cases where needed.

For the flagship WireGuard-over-mesh model, this means:

- one real local WireGuard listen port per WireGuard service
- one stable local mesh ingress port per remote peer/service association

## 10.3 Determinism

Local mesh ingress ports should be deterministic and stable across restarts by default, subject to persisted configuration/state.

---

## 11. WireGuard-over-mesh data-plane model

Transitloom v1 treats WireGuard as a generic UDP-carried service in the core model, but it is the flagship workload.

## 11.1 WireGuard local model

For a WireGuard service on a node:

- WireGuard itself listens on its normal local UDP listen port
- Transitloom exposes one stable local loopback ingress port for each remote WireGuard peer association
- WireGuard sends peer traffic to that local loopback ingress
- Transitloom carries packets across the mesh to the remote WireGuard service
- inbound carried packets are delivered to the local WireGuard listen port

## 11.2 Separation of roles

Transitloom must not confuse:

- the application’s own real local listen port
- the Transitloom-provided local ingress port used for remote peer carriage

These are separate roles.

## 11.3 Keepalive ownership

Transitloom prefers mesh-owned liveness and NAT-maintenance behavior.

WireGuard keepalive may be tolerated, but WireGuard keepalive must not be the primary data-plane liveness dependency when Transitloom is managing path health and underlay continuity.

---

## 12. Forwarding model

## 12.1 General forwarding rule

Transitloom forwards raw UDP packets only within installed association and path context.

Packets are not self-describing at the Transitloom level in v1 zero-overhead mode.

## 12.2 Packet interpretation inputs

The system may identify forwarding context using information such as:

- local listener identity
- local ingress port
- local node/service binding
- source tuple
- destination tuple
- path binding state
- relay binding state
- association state

The exact implementation detail may vary, but the principle is the same: forwarding identity comes from state, not in-band shim headers.

## 12.3 Relay forwarding rule

A relay may forward only when:

- the relevant association/path context has been installed
- the relay is authorized to relay for that association
- relay use is legal under current policy
- the path class is within v1 hop constraints

---

## 13. Scheduling authority

## 13.1 Endpoint-owned scheduling

Transitloom v1 assigns data-plane scheduling authority to the **association endpoints**, not to arbitrary intermediate relays.

This means endpoints are responsible for deciding:

- which eligible paths to use
- how traffic should be split
- when to shift traffic away from unhealthy paths
- when relay should be used or reduced

## 13.2 Relay role in scheduling

Relays provide forwarding and may expose health or capacity information, but they must not behave as unconstrained independent packet schedulers that undermine endpoint policy.

### v1 principle
Relays follow installed forwarding context; endpoints own end-to-end split decisions.

---

## 14. Default v1 scheduler

## 14.1 Default scheduler choice

Transitloom v1 uses:

- **weighted burst/flowlet-aware scheduling** as the default data-plane scheduler

## 14.2 Why this is the default

This default exists to balance:

- aggregation potential
- path diversity
- low avoidable reordering
- operational predictability
- relay/direct path mixtures

It is a safer default than unconstrained per-packet striping across all eligible paths.

## 14.3 Burst/flowlet behavior

The scheduler should prefer to keep short packet bursts or flowlets on the same path when doing so improves effective delivery quality and reduces unnecessary reordering.

The exact flowlet/burst detection logic may be implementation-specific, but the design goal is fixed.

---

## 15. Conditional per-packet striping

## 15.1 Allowed but not universal

Transitloom v1 allows per-packet striping only when eligible paths are **closely matched**.

Per-packet striping is not the universal default.

## 15.2 Closely matched concept

“Closely matched” is a policy- and measurement-based concept that may consider factors such as:

- RTT spread
- jitter spread
- loss spread
- recent delivery confidence
- relay penalties
- measured effective goodput stability

## 15.3 Fallback behavior

If paths stop being closely matched:

- the scheduler should fall back toward weighted burst/flowlet-aware behavior
- path usage should remain stable and conservative rather than continuing aggressive per-packet spraying

---

## 16. Path health model

## 16.1 Inputs

Path decisions may use a combined health/quality score based on inputs such as:

- latency
- jitter
- recent loss
- effective goodput
- relay penalty
- metered/unmetered status
- administrative weight
- policy eligibility
- health confidence

## 16.2 Health categories

A path may conceptually move between states such as:

- candidate
- active
- degraded
- standby
- probe-only
- failed
- admin-disabled

The exact schema is an implementation/detail matter, but the behavioral categories are useful.

## 16.3 Unhealthy paths

When a path becomes unhealthy for carrying live data:

- normal live traffic should stop using it
- path monitoring/probing may continue
- the path may return to active use once health improves sufficiently

Transitloom v1 is not required to keep trickle live data on unhealthy paths just to “see what happens.”

---

## 17. Probing and measurement

## 17.1 Passive measurement

Transitloom should use passive measurement whenever possible from real traffic behavior.

## 17.2 Lightweight active probes

Transitloom should support lightweight active probes for:

- liveness
- latency estimation
- loss estimation
- NAT/path freshness

## 17.3 Capacity probing

Capacity probing should be conservative.

### v1 intent
- active capacity tests may run on unmetered paths
- metered paths should avoid wasteful active burn
- tiny probes may still be allowed where policy says so

## 17.4 Metered paths

Metered/unmetered is a per-path property, not purely a node property.

For metered paths, Transitloom should prefer:

- passive observations
- lightweight probes
- historically informed estimates

over aggressive active capacity tests.

---

## 18. Direct/intranet path handling

## 18.1 Intranet path value

Transitloom should treat direct intranet/private paths as high-value candidates when policy allows them to be shared and used.

These paths may provide:

- lower latency
- better stability
- higher throughput
- no relay cost

## 18.2 Policy awareness

Private/intranet path advertisement and use must remain policy-aware.

Transitloom must not blindly assume that every discovered private address is shareable or desirable.

---

## 19. Relay path handling

## 19.1 Relay legality

A relay path may be used only if:

- the association permits it
- relay class is allowed
- the chosen relay is eligible
- current policy and admission state allow it

## 19.2 Relay path scoring

Relay paths should be scored and compared like other path candidates, with appropriate penalties or weights reflecting:

- extra hop cost
- extra latency/jitter risk
- relay health
- relay capacity
- policy or administrative preference

## 19.3 Control relay vs data relay

Control relay and data relay are not the same thing.

Data relay should be more conservative because it directly affects service quality and aggregation performance.

---

## 20. Rebinding and failover

## 20.1 General rule

Transitloom must tolerate underlay changes such as:

- public IP changes
- path loss
- relay loss
- direct path restoration
- intranet path appearance/disappearance

## 20.2 Failover behavior

Failover should be guided by:

- path health
- policy
- hysteresis
- scheduler stability

Failover should not be hair-trigger if it causes avoidable oscillation.

## 20.3 Direct restoration preference

When a usable direct path becomes available again, Transitloom may prefer it over relay, but restoration should be controlled with hysteresis and stability rules.

---

## 21. Data-plane statefulness

## 21.1 Why statefulness is required

Because v1 raw UDP carries no Transitloom shim header, the data plane requires explicit state.

State is needed at:

- endpoints
- relays
- local service ingress bindings

## 21.2 State installed by control plane

The control plane is responsible for establishing the legality and context in which data-plane forwarding can occur.

The data plane is not allowed to act as an unconstrained learning fabric.

## 21.3 Relay state

Each relay used by v1 raw UDP forwarding must maintain enough association/path state to forward packets correctly without adding in-band metadata.

---

## 22. Observability

## 22.1 Required observability stance

Transitloom v1 should prioritize:

- structured metrics
- logs
- status visibility
- generic association/path statistics

## 22.2 Generic over protocol-specific

Transitloom v1 should expose generic service/association/path statistics rather than deeply WireGuard-specific data-plane counters in the core model.

Examples of useful generic visibility include:

- association-level throughput
- path-level throughput
- path health score
- packet counts
- relay usage
- active scheduler mode
- current selected path set

---

## 23. v1 constraints summary

Transitloom v1 raw UDP data plane is intentionally constrained.

### Included in v1
- raw UDP
- zero in-band overhead
- direct public path
- direct intranet path
- single coordinator relay hop
- single node relay hop
- endpoint-owned scheduling
- weighted burst/flowlet-aware default scheduling
- conditional per-packet striping only for closely matched paths
- practical path probing and health logic
- generic service model
- WireGuard-over-mesh as flagship use case

### Not included in v1
- arbitrary multi-hop raw UDP forwarding
- unconstrained hop-local independent rescheduling
- generic encrypted data plane
- generic TCP data plane
- data-plane shim headers for raw UDP mode
- routing flexibility beyond what is useful for the primary aggregation use case

---

## 24. Future directions

The following are intentionally left for future work:

- generic encrypted UDP carriage
- generic TCP carriage
- richer relay/path classes
- more advanced scheduler families
- broader service-mesh behavior
- arbitrary multi-hop data forwarding
- more advanced path-coding/FEC behaviors
- more sophisticated route diversity logic

---

## 25. Related specifications

This data-plane specification depends on and should remain consistent with:

- `spec/v1-architecture.md`
- `spec/v1-control-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-pki-admission.md`
- `spec/v1-wireguard-over-mesh.md`

---
