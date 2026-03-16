# Transitloom v1 Architecture Baseline

## Status

Draft

This document defines the current v1 architecture baseline for Transitloom. It is intended to freeze the main product and system design decisions before substantial implementation begins.

This is an engineering specification, not a marketing document. It focuses on boundaries, responsibilities, object model, trust model, transport model, and v1 scope.

---

## 1. Purpose

Transitloom is a coordinator-managed overlay mesh transport platform for carrying services across direct paths, intranet paths, and relays in real-world networks with multi-WAN links, NAT/CGNAT, dynamic addressing, and mixed connectivity conditions.

Transitloom v1 is primarily optimized for:

- high-performance raw UDP service carriage
- multi-WAN bandwidth aggregation
- WireGuard-over-mesh deployments
- policy-controlled overlay connectivity
- practical operation across direct and relayed paths

Transitloom is designed around a generic service-carriage model rather than a protocol-specific tunnel, but WireGuard is the flagship v1 use case in documentation and examples.

---

## 2. Goals

### 2.1 Primary goals

- Provide high-performance raw UDP transport over an overlay mesh
- Preserve zero in-band overhead for raw UDP data plane
- Support multi-WAN path usage and practical bandwidth aggregation
- Support direct, intranet, coordinator-relayed, and node-relayed connectivity
- Provide coordinator-managed trust, admission, revocation, and policy control
- Provide a generic service model that can carry WireGuard and other UDP services
- Provide a workable foundation for future expansion without overcommitting v1

### 2.2 Secondary goals

- Support multiple coordinators for HA and operational flexibility
- Support stable local service bindings for applications using the mesh
- Support future extension to additional service types and transport modes
- Support future encrypted generic data carriage, without requiring it in v1

---

## 3. Non-goals for v1

Transitloom v1 does not aim to provide all possible overlay mesh capabilities.

### 3.1 Explicit non-goals

- Arbitrary multi-hop raw UDP data forwarding
- A general-purpose full service mesh for every protocol from day one
- Deep protocol-specific WireGuard behavior in the core transport model
- Public Internet PKI or external CA dependencies
- Mandatory kernel integration
- Finalized generic TCP carriage implementation
- Finalized generic encrypted data plane implementation
- Fully general dynamic routing sophistication beyond what is needed for v1 coordination and relay selection

### 3.2 Scope boundary for performance

Transitloom v1 prioritizes making multi-WAN raw UDP aggregation work well over supporting maximum routing flexibility.

---

## 4. Product model

Transitloom is built around four major concepts:

- **authorities**
- **coordinators**
- **nodes**
- **services**

The system is coordinator-managed, but traffic carriage is performed by nodes across authorized direct and relayed overlay paths.

### 4.1 Runtime roles

Transitloom has three major runtime infrastructure roles:

- **Root authority**
- **Coordinator**
- **Node**

### 4.2 Human roles

Transitloom has two human actor classes:

- **Authorized admins**
- **End users / operators of nodes**

Authorized admins manage coordinators, trust, policy, admission, and global control state.

End users or site operators run nodes and consume the overlay transport services.

---

## 5. Roles and responsibilities

## 5.1 Root authority

The root authority is a special trust role, not a normal end-user coordinator target.

The root authority is responsible for:

- acting as the trust anchor for the Transitloom PKI
- issuing or managing coordinator intermediate CA certificates
- supporting coordinator intermediate lifecycle events
- not serving ordinary coordinator traffic to end-user nodes in normal operation

### v1 root authority constraints

- The root authority is not exposed as a normal coordinator target to end-user nodes
- The root authority should not serve ordinary coordinator traffic for end users
- The root authority may be implemented as a distinct binary or distinct operational mode, but is logically separate from coordinators

## 5.2 Coordinators

Coordinators are admin-operated infrastructure components.

They are responsible for:

- trust and admission enforcement
- service and policy distribution
- coordinator-assisted discovery
- control-plane coordination
- relay assistance
- optional control-plane and data-plane relaying
- certificate issuance using coordinator intermediates
- verification of short-lived admission tokens
- participating in the distributed coordinator control network

### Coordinator properties

- centrally managed by authorized admins
- may exist in multiple locations or failure domains
- may be discovered by nodes
- may assist route construction and relay selection
- may carry relayed traffic when policy allows

## 5.3 Nodes

Nodes are participants that expose services to the mesh and carry traffic across overlay paths.

Nodes may represent:

- end-user devices
- servers
- routers
- gateways
- appliances
- other managed systems

Nodes are responsible for:

- registering and exposing services
- establishing authorized direct and relayed paths
- probing and measuring path quality
- participating in service associations
- performing endpoint-owned traffic scheduling for data plane
- carrying local service traffic over the mesh

---

## 6. Trust and admission model

Transitloom v1 separates **identity** from **participation permission**.

### 6.1 Identity

Identity is established using certificates and keys.

### 6.2 Participation permission

Participation permission is established using short-lived admission state and tokens.

This separation allows hard revocation without requiring extremely short-lived identity certificates.

## 6.3 PKI structure

Transitloom v1 uses:

- one root authority
- per-coordinator intermediates under the root
- node identity certificates issued through authorized coordinators

### PKI roles

- **Root CA**: trust anchor
- **Coordinator intermediate CA**: issues node certificates
- **Node certificate**: identifies a node
- **Admission token**: short-lived signed authorization proving current participation permission

## 6.4 Certificates vs admission tokens

### Certificates
Certificates represent node identity.

### Admission tokens
Admission tokens represent current permission to participate in the coordinator network and mesh.

A valid node certificate alone is not sufficient for normal participation.

### v1 requirement
Normal node participation requires:

- a valid node certificate
- a valid short-lived admission token

## 6.5 Revocation

Transitloom v1 uses **hard revoke** semantics for participation.

Hard revoke means:

- a revoked node must not successfully establish normal control sessions
- a revoked node must not successfully reconnect even if its identity certificate remains valid
- coordinators must reject revoked nodes based on current admission state and token validity

This behavior is enforced operationally by admission checks, not by forcing extremely short certificate lifetimes.

---

## 7. Coordinator network and state model

## 7.1 Global security-sensitive state

Transitloom v1 strongly globalizes only security-sensitive control state.

This includes:

- admission state
- revoke state
- trust state

## 7.2 Non-global or not-strongly-global state

Transitloom v1 does not require strongly global replication for high-churn or local transport state.

This includes:

- live path metrics
- forwarding adaptation
- most live path selection state
- most transport health measurements

Only summary or aggregated health may be shared where useful.

## 7.3 Coordinator partition behavior

A partitioned coordinator may accept global admin operations only as **pending proposals**, not as committed truth.

Pending proposals must not be treated as committed global state by nodes until the coordinator network converges and commits the change.

## 7.4 Security-sensitive operation model

Security-sensitive admin actions should use an ordered operation model rather than weak conflict behavior.

This includes operations such as:

- admit node
- revoke node
- trust changes
- related global security-sensitive state changes

### v1 intent
Security-sensitive objects should be modeled through ordered operations and deterministic application rather than through ad hoc overwrite semantics.

---

## 8. Service model

Transitloom is service-oriented.

## 8.1 Services

A **service** is a locally exposed endpoint that Transitloom can carry over the mesh.

In v1, the primary service type is raw UDP.

A service may include:

- service identity
- local target/bind information
- metadata
- policy
- capabilities
- future transport capability declarations

## 8.2 Associations

An **association** is the logical connectivity relationship between services across nodes.

Associations are first-class concepts in Transitloom.

An association defines:

- source and destination service relationship
- permitted connectivity class
- permitted relay usage
- transport policy
- scheduling behavior
- health policy

Associations may be bidirectional in behavior even when modeled through explicit service-to-service relationships.

## 8.3 Service registry

Transitloom v1 includes a service registry that serves both:

- human-facing discovery and naming purposes
- machine-facing association and policy purposes

The registry should be generic, not WireGuard-specific.

---

## 9. WireGuard-over-mesh position

WireGuard is the flagship use case for Transitloom v1 documentation and examples, but it is not privileged in the core service model.

Transitloom should support WireGuard without requiring WireGuard protocol changes.

## 9.1 WireGuard integration model

Transitloom’s intended WireGuard-over-mesh model is:

- WireGuard listens on its normal local listen port
- Transitloom provides stable local loopback ingress ports for remote peer associations
- WireGuard peers use those local loopback endpoints
- Transitloom handles the real mesh transport underneath

Transitloom is responsible for:

- path discovery
- direct path usage
- relay usage
- keepalive behavior
- path scheduling
- relay fallback
- NAT traversal assistance where applicable

WireGuard remains responsible for its own protocol-level encryption and semantics.

## 9.2 WireGuard in the core model

Transitloom v1 treats WireGuard as a generic UDP-carried service in the core model.

Transitloom v1 documentation and examples should still explicitly optimize around WireGuard as the flagship adoption path.

---

## 10. Path and link model

Transitloom operates over a set of path classes.

## 10.1 Path classes

Transitloom v1 may use combinations of:

- direct public paths
- direct intranet/private paths
- coordinator relay paths
- node relay paths

## 10.2 Data-plane hop constraint

Transitloom v1 limits raw UDP data-plane forwarding to:

- direct paths
- or single relay hop only

This constraint exists to preserve predictable aggregation behavior and reduce complexity for zero-overhead raw UDP transport.

### Not in v1
Raw UDP data plane does not support arbitrary multi-hop forwarding in v1.

## 10.3 Control-plane relay flexibility

Control plane may use broader relay flexibility than data plane where operationally useful.

Control-plane routing is still constrained in v1 and is not intended to become an unconstrained general routing system before the primary aggregation use case is proven.

---

## 11. Routing and route construction

## 11.1 v1 routing direction

Transitloom v1 uses a constrained, coordinator-assisted route construction model.

It does not aim to implement maximum routing flexibility in v1.

## 11.2 Route construction responsibilities

### Coordinators
Coordinators are responsible for:

- discovery assistance
- policy filtering
- relay eligibility knowledge
- route suggestion / assistance
- relayed path decision influence where appropriate

### Nodes
Nodes are responsible for:

- probing direct and available paths
- local path health measurement
- endpoint-owned data scheduling
- final adaptation among eligible data paths

## 11.3 Control vs data routing

Control and data routing are intentionally not equivalent in v1.

### Control plane
Control plane may be more flexible and relay-tolerant.

### Data plane
Data plane is more constrained because aggregation performance is a primary objective.

---

## 12. Data-plane transport model

## 12.1 Primary v1 transport focus

The primary v1 transport focus is raw UDP.

## 12.2 Zero-overhead requirement

Transitloom v1 requires zero in-band overhead for raw UDP data-plane transport.

This means:

- Transitloom should not add payload-visible per-packet shim headers in the raw UDP data path
- forwarding identity and association state must be maintained through control-plane-installed state and path binding state rather than through in-band per-packet metadata

## 12.3 Relay statefulness

Because raw UDP zero-overhead is required, relays must be stateful.

For v1 raw UDP forwarding, relays maintain explicit state sufficient to forward traffic without inserting packet headers.

## 12.4 Scheduling authority

For v1 data plane, scheduling authority is endpoint-owned.

Endpoints decide how traffic is split across eligible paths.

Relays should not be free to perform arbitrary independent hop-level scheduling behavior in ways that undermine end-to-end policy.

## 12.5 v1 default scheduler

Transitloom v1 uses:

- **weighted burst/flowlet-aware scheduling** as the default

Per-packet striping is allowed only when paths are closely matched according to policy and measured path conditions.

### Intent
This design preserves aggregation potential while reducing excessive reordering and instability on mismatched paths.

## 12.6 Relay usage for data plane

Data-plane relay is allowed in v1, but raw UDP data forwarding remains limited to:

- direct
- or single relay hop

Data relay may use:

- coordinator relay
- node relay

according to policy and path selection logic.

---

## 13. Path health and policy

## 13.1 Path health inputs

Path decisions may consider a combined score incorporating factors such as:

- latency
- loss
- jitter
- effective goodput
- path cost
- metered/unmetered status
- relay penalty
- policy

## 13.2 Metering model

Metered or unmetered classification is a per-path property.

## 13.3 Keepalive ownership

Transitloom prefers mesh-owned keepalive behavior.

Application-layer keepalive behavior, including WireGuard keepalive, may be tolerated but should not be the primary operational dependency when Transitloom is managing path liveness.

---

## 14. Discovery model

## 14.1 Coordinator discovery

Nodes may discover coordinators from:

- preconfigured coordinators
- authorized coordinator catalogs
- authorized node-provided hints

Coordinator discovery learned from nodes is treated as a hint, not as trust truth.

Cryptographic trust and policy still govern coordinator usability.

## 14.2 Node discovery

Node discovery may occur broadly according to policy, but discovery does not automatically imply authorization to create associations.

Automatic association behavior should remain policy-constrained.

## 14.3 Private address sharing

Private or intranet address sharing may be controlled by local node configuration and related policy.

This area should remain explicitly policy-aware because it has information exposure implications.

---

## 15. Relay policy

## 15.1 Control relay vs data relay

Transitloom v1 distinguishes between control relay and data relay.

Control relay may be broader and easier to permit than data relay.

Data relay should remain more carefully constrained because poor relay choices directly damage aggregation performance.

## 15.2 Relay eligibility

Relay eligibility is policy-controlled.

Node relay and coordinator relay are both valid concepts, but their operational roles may differ.

### v1 direction
Transitloom should be able to express different relay permissions for:

- control plane
- data plane
- specific nodes
- specific services
- specific associations

---

## 16. Product stance and documentation stance

## 16.1 Core model stance

The Transitloom core model remains generic.

It should not be deeply specialized around WireGuard behavior.

## 16.2 Public-facing adoption stance

Transitloom v1 documentation and examples should explicitly optimize around the flagship use case:

- WireGuard over mesh
- multi-WAN raw UDP aggregation
- real-world direct + relay overlay transport

This is a documentation and onboarding choice, not a core model limitation.

---

## 17. Implementation guidance for v1

## 17.1 General implementation priorities

Transitloom v1 should prioritize:

- aggregation performance
- predictable data-plane behavior
- clear trust and admission behavior
- maintainable generic service model
- constrained routing scope
- disciplined scope boundaries

## 17.2 Performance stance

Transitloom v1 is performance-first.

Where high-performance path adaptation conflicts with over-broad global coordination or excessive routing flexibility, performance for the primary use case should win.

## 17.3 Kernel integration

Kernel integration is not a requirement for v1 and should be avoided unless clearly necessary.

---

## 18. Deferred or future work

The following are intentionally left for later phases or later design work:

- arbitrary multi-hop raw UDP data forwarding
- generalized dynamic multi-hop service mesh behavior
- finalized generic TCP carriage
- generic encrypted data plane
- deeper service-specific helpers beyond flagship documentation
- more advanced routing sophistication beyond v1 needs
- broader application service mesh ambitions beyond the primary aggregation use case

---

## 19. Open questions for deeper specs

This architecture baseline fixes the major v1 boundaries, but additional detailed specifications are still required for:

- control-plane protocol
- service registry schema
- association schema
- route and relay object model
- path measurement model
- exact scheduler rules and thresholds
- PKI lifecycle
- admission token format and lifecycle
- coordinator operation log model
- WireGuard-over-mesh operational workflow

These should be defined in more specific documents under `spec/`.

---

## 20. Related v1 spec documents

The following documents should refine this architecture baseline:

- `spec/v1-control-plane.md`
- `spec/v1-data-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-pki-admission.md`
- `spec/v1-wireguard-over-mesh.md`

---
