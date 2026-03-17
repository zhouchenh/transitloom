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

---

## Durable control-plane decisions

- QUIC + TLS 1.3 mTLS is the primary control transport
- TCP + TLS 1.3 mTLS is the fallback control transport
- Control-plane semantics should stay logically consistent across QUIC and TCP
- Control plane is more flexible than data plane
- Security-sensitive global state should use ordered operations rather than weak overwrite semantics
- Partitioned coordinators may accept security-sensitive changes only as **pending proposals**
- Nodes must not treat pending proposals as committed truth

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
