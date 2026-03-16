# Transitloom v1 WireGuard-over-Mesh Specification

## Status

Draft

This document specifies the Transitloom v1 WireGuard-over-mesh model. It defines how standard WireGuard deployments are mapped onto Transitloom services, associations, local bindings, control-plane policy, and raw UDP data-plane carriage.

This document does **not** redefine the WireGuard protocol. Transitloom carries WireGuard traffic over the overlay mesh without requiring WireGuard protocol changes.

---

## 1. Purpose

Transitloom v1 uses WireGuard-over-mesh as its flagship deployment model.

The purpose of this document is to define:

- how a WireGuard instance maps to a Transitloom service
- how WireGuard peers map to Transitloom associations
- how local listen ports and local mesh ingress ports are separated
- how Transitloom carries WireGuard UDP traffic across direct and relayed overlay paths
- how keepalive responsibility is divided
- how this flagship use case fits into the generic Transitloom service model

---

## 2. Goals

### 2.1 Primary goals

- Support standard WireGuard without protocol changes
- Keep Transitloom generic in the core model
- Make WireGuard-over-mesh operationally practical
- Preserve zero in-band overhead for raw UDP carriage
- Support multi-WAN aggregation for WireGuard traffic
- Support direct, intranet, and single-relay-hop data carriage
- Provide stable local loopback endpoints for WireGuard peer configuration

### 2.2 Secondary goals

- Make WireGuard the flagship documented v1 use case
- Allow multiple WireGuard services per node
- Leave room for future helper tooling without making it required in v1
- Keep the WireGuard model compatible with broader generic UDP service carriage

---

## 3. Non-goals for v1

Transitloom v1 WireGuard-over-mesh does not aim to provide:

- WireGuard protocol modifications
- kernel patches or WireGuard implementation changes
- a requirement to rewrite WireGuard configs automatically
- special core semantics that make WireGuard the only supported service model
- arbitrary multi-hop raw UDP forwarding for WireGuard traffic
- a requirement that WireGuard keepalive be the primary liveness mechanism

---

## 4. High-level model

Transitloom treats a WireGuard instance as a generic UDP-carried service.

At a high level:

- the local WireGuard instance listens on its normal local UDP `ListenPort`
- Transitloom exposes one stable local ingress endpoint per remote WireGuard peer/service association
- WireGuard sends peer traffic to those Transitloom-provided local endpoints
- Transitloom carries the packets over the mesh to the remote WireGuard service
- inbound remote packets are delivered by Transitloom to the local WireGuard `ListenPort`

WireGuard remains responsible for its own protocol-level encryption and peer semantics.

Transitloom remains responsible for the overlay transport underneath.

---

## 5. Core mapping

## 5.1 WireGuard interface to Transitloom service

Each WireGuard interface intended for Transitloom carriage maps to a Transitloom service instance.

That service instance includes at least:

- node identity
- service identity
- service type
- local target UDP endpoint
- policy metadata
- association eligibility

For WireGuard, the local target is the local WireGuard `ListenPort`.

## 5.2 WireGuard peer to Transitloom association

Each remote WireGuard peer relationship maps to a Transitloom association between:

- the local WireGuard service
- the remote WireGuard service

Transitloom uses the association to define:

- legal carriage
- eligible path classes
- eligible relay classes
- scheduling behavior
- local ingress port assignment
- policy and health behavior

## 5.3 Multiple peers

A single local WireGuard service may participate in multiple associations, one for each remote peer/service relationship.

---

## 6. Local endpoint model

The most important implementation detail is the separation between:

- the **real local WireGuard listen port**
- the **Transitloom local ingress ports** used as peer endpoints

These are not the same thing.

## 6.1 Real local WireGuard listen port

The WireGuard `[Interface] ListenPort` is the actual local UDP port on which the local WireGuard service listens.

Example:

- node A WireGuard service real local listen port: `51820`

Transitloom must deliver inbound remote WireGuard traffic for that service to this local target.

## 6.2 Transitloom local ingress port

For each remote WireGuard peer association, Transitloom provides a stable local ingress endpoint that WireGuard can send traffic to.

Example on node A:

- local ingress for remote peer/service on node B: `127.0.0.1:41002`
- local ingress for remote peer/service on node C: `127.0.0.1:41003`

WireGuard uses these ingress ports as peer `Endpoint` values.

Transitloom receives packets on these local ingress ports and carries them to the appropriate remote WireGuard service.

## 6.3 Stability requirement

Transitloom v1 should provide deterministic and stable local ingress ports across restarts by default, subject to persisted configuration/state.

This matters because WireGuard peer configuration is typically static or semi-static.

---

## 7. Example conceptual mapping

Consider a three-node deployment: node A, node B, node C.

### Node A
- WireGuard service listen port: `<node-a-listen-port>`
- Transitloom ingress for peer B: `<mesh-port-a-to-b>`
- Transitloom ingress for peer C: `<mesh-port-a-to-c>`

### Node B
- WireGuard service listen port: `<node-b-listen-port>`
- Transitloom ingress for peer A: `<mesh-port-b-to-a>`
- Transitloom ingress for peer C: `<mesh-port-b-to-c>`

### Node C
- WireGuard service listen port: `<node-c-listen-port>`
- Transitloom ingress for peer A: `<mesh-port-c-to-a>`
- Transitloom ingress for peer B: `<mesh-port-c-to-b>`

WireGuard configuration then points each peer `Endpoint` to the appropriate local loopback ingress port, while Transitloom carries the traffic over the mesh.

---

## 8. WireGuard configuration model

The intended v1 usage model is that WireGuard remains standard and Transitloom adapts around it.

## 8.1 Interface section

A WireGuard interface should keep its own normal fields, including:

- `PrivateKey`
- `Address`
- `ListenPort`

Transitloom does not replace the local WireGuard service itself.

## 8.2 Peer section

For each peer, the WireGuard `Endpoint` points to a Transitloom local ingress endpoint, typically on loopback.

Examples:

- `127.0.0.1:<mesh-port>`
- `[::1]:<mesh-port>`

Transitloom uses that ingress endpoint to map the peer’s traffic into the correct remote association.

## 8.3 Local loopback preference

Transitloom v1 should prefer local loopback ingress endpoints for this flagship usage model.

This keeps WireGuard isolated from the underlay details such as:

- public IPs
- dynamic endpoint changes
- relay choices
- path switching
- intranet discovery
- multi-WAN logic

---

## 9. Send path behavior

## 9.1 Local send flow

When the local WireGuard instance sends a packet to a peer endpoint configured as a Transitloom local ingress port:

1. WireGuard sends UDP to the Transitloom local ingress endpoint
2. Transitloom identifies the relevant association
3. Transitloom selects among eligible direct/relay paths according to scheduler and policy
4. Transitloom carries the raw UDP packet over the overlay mesh
5. The remote Transitloom instance delivers the packet to the remote WireGuard service local target

## 9.2 Endpoint-owned scheduling

Transitloom endpoint nodes remain responsible for scheduling/splitting traffic across eligible paths.

Relay nodes and coordinator relays do not own the end-to-end scheduling policy for WireGuard traffic.

---

## 10. Receive path behavior

## 10.1 Remote receive flow

When a remote WireGuard packet arrives at the destination node through Transitloom:

1. Transitloom identifies the association and target local WireGuard service
2. Transitloom delivers the packet to the local WireGuard `ListenPort`
3. The local WireGuard instance processes the packet normally

Transitloom does not require WireGuard to know or care whether the packet arrived via:

- direct public path
- intranet path
- coordinator relay
- node relay

---

## 11. Reverse initiation behavior

Transitloom must support bidirectional initiation in practical terms.

WireGuard traffic does not have to originate only from one side.

If node B’s WireGuard service sends first toward node A’s service:

1. node B WireGuard sends to node B’s local Transitloom ingress for node A
2. Transitloom carries the packet over the overlay mesh
3. node A Transitloom delivers it to node A’s local WireGuard `ListenPort`

This works regardless of which side sends first, as long as the relevant Transitloom association and path context exist and are legal.

---

## 12. Keepalive model

## 12.1 Transitloom-owned liveness preference

Transitloom prefers mesh-owned keepalive and liveness handling.

For WireGuard-over-mesh, this means Transitloom should manage:

- overlay path liveness
- relay liveness
- NAT freshness where relevant
- path probing and fallback behavior

## 12.2 WireGuard PersistentKeepalive

WireGuard `PersistentKeepalive` may still be tolerated, but Transitloom v1 should not require it for correct overlay operation when Transitloom itself is configured to maintain liveness.

In other words:

- WireGuard keepalive may exist
- Transitloom keepalive and path management are the primary mechanism

## 12.3 Practical implication

Transitloom should be able to support WireGuard-over-mesh even when `PersistentKeepalive` is unset, provided Transitloom is configured to maintain overlay continuity.

---

## 13. Data-plane transport rules for WireGuard

WireGuard traffic carried by Transitloom follows the raw UDP v1 data-plane rules.

## 13.1 Included in v1

WireGuard carriage may use:

- direct public paths
- direct intranet/private paths
- single coordinator relay hop
- single node relay hop

## 13.2 Not included in v1

WireGuard carriage does not use:

- arbitrary multi-hop raw UDP forwarding
- in-band Transitloom shim headers in zero-overhead mode
- generic encrypted Transitloom data carriage in v1

WireGuard already provides its own encryption at the application/protocol layer.

---

## 14. Zero-overhead requirement for WireGuard carriage

WireGuard-over-mesh is one of the main reasons Transitloom preserves zero in-band overhead for raw UDP.

Transitloom v1 should carry WireGuard UDP packets without adding Transitloom-visible payload shim headers in the raw UDP path.

This helps preserve MTU behavior relative to the Transitloom raw UDP transport itself, while still allowing Transitloom to perform overlay forwarding using stateful bindings and control-plane-installed context.

---

## 15. Multiple WireGuard services per node

Transitloom v1 supports multiple WireGuard services on the same node.

## 15.1 Requirement

Each WireGuard service instance must have:

- its own service identity
- its own local target binding
- its own association set
- its own stable ingress bindings where needed

## 15.2 Reason

This allows one node to host multiple distinct WireGuard interfaces or service contexts without confusing their identities or bindings.

---

## 16. Local address family behavior

Transitloom local ingress endpoints used by WireGuard peers may be exposed on:

- IPv4 loopback
- IPv6 loopback

Transitloom v1 should support at least a clear, deterministic configuration or default behavior for local loopback addressing.

Examples:

- `127.0.0.1:<port>`
- `[::1]:<port>`

The exact default may be implementation-specific, but the behavior must be consistent and predictable.

---

## 17. Association and service legality

WireGuard traffic is not forwarded merely because a UDP packet appears on a local ingress port.

Transitloom must forward WireGuard traffic only when:

- the corresponding service exists
- the corresponding remote service exists
- an association exists or is otherwise legal under current control-plane state
- relay/path use is allowed
- the node remains admitted and authorized

This preserves the generic Transitloom legality model.

---

## 18. Discovery and peer knowledge

Transitloom may discover nodes, services, and coordinators according to policy, but discovery does not automatically create WireGuard associations.

WireGuard-over-mesh still depends on legal and authorized Transitloom service associations.

The WireGuard configuration itself remains separate from Transitloom discovery unless future helper tooling is added.

---

## 19. Tooling stance for v1

Transitloom v1 does not require automatic WireGuard configuration rewriting.

### v1 stance
- automatic WireGuard config rewriting is not required
- optional snippet generation may exist later
- the main requirement is that the local ingress model be stable and understandable

Transitloom documentation should still provide WireGuard-oriented examples and guidance because WireGuard is the flagship use case.

---

## 20. Relationship to generic service model

Transitloom must keep the WireGuard model compatible with the generic service model.

This means:

- WireGuard is modeled as a service, not as a special network primitive
- WireGuard peer carriage is modeled through associations
- local ingress bindings are generic enough to be reused by other UDP services later
- Transitloom core architecture should not depend on WireGuard-specific packet semantics

---

## 21. Operator-facing expectations

For an operator deploying WireGuard-over-mesh in Transitloom v1, the practical mental model should be:

- WireGuard still runs normally on each node
- Transitloom provides stable local peer endpoints
- Transitloom handles real transport across the mesh
- Transitloom manages direct vs relay use, path health, and multi-WAN behavior
- WireGuard does not need to know the real remote public endpoint details

This is the intended operational simplification.

---

## 22. Example conceptual configuration

A conceptual WireGuard-over-mesh configuration on node A may look like this:

- local WireGuard service listens on `<node-a-listen-port>`
- peer B endpoint is `127.0.0.1:<mesh-port-a-to-b>`
- peer C endpoint is `127.0.0.1:<mesh-port-a-to-c>`

Transitloom then maps:

- local ingress `<mesh-port-a-to-b>` -> remote WireGuard service on node B
- local ingress `<mesh-port-a-to-c>` -> remote WireGuard service on node C

with overlay path selection and scheduling performed by Transitloom.

The exact user-facing config examples may be documented separately, but this conceptual mapping is fixed.

---

## 23. Observability for WireGuard carriage

Transitloom v1 should prioritize generic association/path/service observability rather than deep WireGuard-specific observability in the core.

Useful generic visibility includes:

- service identity
- association identity
- local ingress mapping
- active path set
- relay use
- throughput
- health score
- scheduler mode

Transitloom does not need to make WireGuard-specific counters first-class in the core data-plane model.

---

## 24. v1 constraints summary

Transitloom v1 WireGuard-over-mesh includes:

- standard WireGuard without protocol changes
- one Transitloom service per WireGuard instance
- one association per remote peer/service relationship
- stable local loopback ingress ports
- separation of local WireGuard `ListenPort` and Transitloom local ingress ports
- endpoint-owned scheduling
- zero-overhead raw UDP carriage
- direct and single-relay-hop data paths
- Transitloom-owned liveness preference

Transitloom v1 WireGuard-over-mesh does not include:

- WireGuard protocol modification
- mandatory automatic config rewriting
- arbitrary multi-hop raw UDP forwarding
- special core semantics that make WireGuard the only meaningful service type

---

## 25. Future directions

The following are intentionally deferred:

- automatic WireGuard config helper modes
- richer WireGuard-oriented UX tooling
- deeper WireGuard-specific observability helpers
- integration helpers for managed deployments
- future encrypted non-WireGuard service carriage
- future TCP-based service carriage

---

## 26. Related specifications

This WireGuard-over-mesh specification depends on and should remain consistent with:

- `spec/v1-architecture.md`
- `spec/v1-control-plane.md`
- `spec/v1-data-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-pki-admission.md`

---
