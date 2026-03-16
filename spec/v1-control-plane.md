# Transitloom v1 Control Plane Specification

## Status

Draft

This document specifies the Transitloom v1 control plane. It refines the architecture baseline and defines the control-plane responsibilities, trust model, control transports, session model, coordinator interactions, discovery behavior, relay behavior, and the high-level operation model used by nodes and coordinators.

This document focuses on the **control plane**, not the raw UDP data plane.

---

## 1. Purpose

The Transitloom control plane is responsible for coordinating the overlay mesh.

It exists to provide:

- authenticated identity and participation checks
- node admission enforcement
- service registration and discovery
- association setup and policy distribution
- coordinator discovery
- relay assistance
- route/path coordination
- path summary exchange where useful
- control-plane liveness and health management

The control plane is intentionally separate from the data plane. The data plane carries service traffic. The control plane establishes, authorizes, and manages the context in which data-plane traffic is allowed to exist.

---

## 2. v1 design goals

### 2.1 Primary goals

- Provide a secure, coordinator-managed control fabric
- Preserve clear separation between control and data planes
- Support multiple coordinators
- Support coordinator-assisted overlay discovery and setup
- Support relay-assisted control connectivity
- Support hard revocation through current admission checks
- Keep the control protocol generic rather than WireGuard-specific
- Support the flagship WireGuard-over-mesh use case without changing WireGuard itself

### 2.2 Secondary goals

- Support future evolution without replacing the core control model
- Support constrained coordinator-assisted route construction
- Support policy-aware discovery and relay eligibility
- Support stable operational behavior when coordinators or paths fail

---

## 3. Non-goals for v1

The v1 control plane does not aim to provide:

- a full unconstrained distributed routing protocol for all future mesh behavior
- complete global replication of all transport/path metrics
- a fully general service-mesh API surface from day one
- a dependency on external PKI services or public Internet CA infrastructure
- a requirement for plaintext or unauthenticated control transport
- arbitrary control-plane topology complexity before the primary use case is proven

---

## 4. Control-plane roles

Transitloom v1 control plane involves three infrastructure roles:

- **Root authority**
- **Coordinator**
- **Node**

## 4.1 Root authority role

The root authority is the trust anchor for the Transitloom PKI.

Its control-plane responsibilities are limited to trust-authority behavior, including:

- root trust anchor management
- coordinator intermediate lifecycle support
- special administrative trust operations

The root authority is not a normal end-user coordinator target.

The root authority is out of the normal end-user discovery path and should not act as a standard coordinator for ordinary node sessions.

## 4.2 Coordinator role

Coordinators are the primary control-plane infrastructure.

Coordinators are responsible for:

- authenticating node control sessions
- verifying node certificate identity
- verifying short-lived admission tokens
- enforcing current admission/revoke state
- issuing node certificates through coordinator intermediates
- distributing service and policy information
- assisting discovery
- assisting route/relay construction
- relaying control sessions where applicable
- handling administrative commands
- participating in the distributed coordinator control network

## 4.3 Node role

Nodes are control-plane clients and mesh participants.

Nodes are responsible for:

- establishing authenticated control sessions
- presenting identity credentials
- presenting current admission tokens
- registering local services
- learning coordinators and peers according to policy
- learning association and relay policy
- probing eligible paths
- reporting selected summaries where useful
- establishing control-plane sessions directly or through authorized relays

---

## 5. Trust model

Transitloom v1 separates **identity** from **current participation permission**.

## 5.1 Identity

Identity is based on node certificates and their associated key pairs.

A node certificate proves who the node is.

## 5.2 Participation permission

Participation permission is based on a short-lived signed **admission token**.

An admission token proves that the node is currently allowed to participate in the Transitloom network.

## 5.3 Mandatory requirements for node participation

A normal node control session requires both:

- a valid node certificate
- a valid admission token

A valid certificate alone is not sufficient.

## 5.4 Hard revoke model

Transitloom v1 uses hard revoke semantics.

If a node is revoked:

- coordinators must reject normal control sessions from that node
- coordinators must reject reconnect attempts from that node
- the node must not be treated as an active participant even if its identity certificate remains valid until a later expiry time

Revocation enforcement is operational and admission-state driven, not dependent on forcing extremely short certificate lifetimes.

---

## 6. Control transports

Transitloom v1 supports two control transports:

- **QUIC + TLS 1.3 mTLS** as the primary transport
- **TCP + TLS 1.3 mTLS** as the fallback transport

These are two transports for one control protocol, not two different control protocols.

## 6.1 Primary transport

The preferred control-plane transport is:

- QUIC
- TLS 1.3
- mutual TLS
- Transitloom private PKI identity verification
- admission-token enforcement at session establishment

## 6.2 Fallback transport

The fallback control-plane transport is:

- TCP
- TLS 1.3
- mutual TLS
- the same identity and admission model as QUIC

## 6.3 Transport equivalence rule

The application control protocol above transport must remain logically equivalent across QUIC and TCP.

The following must remain consistent across transports:

- message types
- authorization model
- session semantics
- service registration behavior
- admission enforcement
- policy distribution
- discovery behavior

---

## 7. Relayed control sessions

Transitloom v1 allows relayed control connectivity.

Control plane is intentionally more relay-flexible than data plane.

## 7.1 Allowed relay classes

Control-plane sessions may be established:

- directly to a coordinator
- directly between eligible nodes where applicable
- through a coordinator relay
- through a relay-capable node, subject to policy

## 7.2 End-to-end trust preservation

The trust relationship that matters is still between the actual session endpoints.

Relay involvement must not weaken endpoint authentication requirements.

Transitloom v1 may support different relay/proxy behaviors depending on transport and deployment mode, but endpoint trust must remain explicit and policy-controlled.

## 7.3 Relay vs direct preference

Control plane should prefer direct connectivity when healthy and allowed, but relay-assisted control sessions are valid and important for:

- bootstrap
- NAT/CGNAT reachability
- degraded network conditions
- coordinator access from constrained nodes

## 7.4 Relay depth

Control plane may be more flexible than data plane in relay usage, but v1 should still prioritize operational simplicity and bounded complexity.

This document leaves exact control-plane relay-depth and route-expansion details to more detailed route/relay specifications, while preserving the v1 principle that control is more flexible than data.

---

## 8. Session model

Transitloom control interactions occur through authenticated control sessions.

## 8.1 Session types

Transitloom v1 includes these logical control session classes:

- **node-to-coordinator sessions**
- **coordinator-to-coordinator sessions**
- **special administrative sessions**
- **optional relayed node-to-coordinator sessions**
- **optional node-to-node control interactions where policy and later specs allow**

## 8.2 Session establishment prerequisites

To establish a normal node-to-coordinator control session, a node must present:

- its certificate-backed identity
- its current admission token
- transport-level mTLS identity proof

The coordinator must verify:

- certificate validity
- issuer trust chain
- node identity
- admission-token validity
- current global admission state
- that the node is not revoked

## 8.3 Session outcome

A control session may result in one of:

- successful session establishment
- explicit rejection
- silent denial where policy requires concealment-like behavior
- relay-offered alternative path information
- transport fallback behavior

The exact external behavior depends on policy and deployment mode.

---

## 9. Coordinator network state model

Transitloom v1 narrows strongly global state to security-sensitive objects.

## 9.1 Strongly global security-sensitive state

This includes:

- node admission state
- node revoke state
- trust-related state
- related security-sensitive coordinator control information

## 9.2 Not-strongly-global state

The following are not required to be strongly global in v1:

- full live path metrics
- full detailed path-quality histories
- local transport adaptation
- most local forwarding decisions
- most ephemeral health observations

Only summary or aggregated health may be shared where useful.

## 9.3 Partition behavior

If a coordinator is partitioned from the rest of the coordinator network, it may accept a security-sensitive administrative write only as a **pending proposal**.

Pending proposals are not committed truth.

Nodes must not act on such proposals as if they are committed global state.

## 9.4 Security-sensitive operation model

Security-sensitive global changes should be modeled as ordered operations.

This includes operations such as:

- admit node
- revoke node
- trust updates
- related security-sensitive global state changes

Transitloom v1 should use an ordered security-sensitive admin operation model rather than weak overwrite semantics.

---

## 10. Administrative command model

Transitloom allows authorized admins to manage the coordinator network from any appropriate coordinator entry point.

## 10.1 Admin ingress

An authorized admin may issue commands through a coordinator that is allowed to accept administrative control actions.

## 10.2 Operation classes

Administrative control operations include, at minimum:

- node admission
- node revoke
- node re-admission
- trust-related changes
- coordinator-related control changes
- policy changes
- local coordinator serving policy changes where applicable

## 10.3 Pending vs committed operations

When coordinator network conditions do not allow an operation to become committed truth immediately, the operation may be recorded as pending.

Pending operations must not be treated as committed global truth by nodes.

## 10.4 Local vs global operations

Transitloom distinguishes between:

- **global security-sensitive operations**
- **coordinator-local serving-policy operations**

Global security-sensitive operations require proper ordered global handling.

Coordinator-local policy operations may be narrower in scope and need not necessarily become globally authoritative objects in the same way.

---

## 11. Admission lifecycle

Transitloom v1 supports controlled node lifecycle transitions.

## 11.1 Admission states

At a minimum, the control plane must be able to represent concepts equivalent to:

- pending
- admitted
- revoked

The exact schema is refined in the PKI/admission specification.

## 11.2 First admission

A new node must not become a normal participant until admitted through the proper administrative process.

## 11.3 Revoke

Revoke removes current participation permission.

Revoke must cause:

- rejection of normal new control sessions
- rejection of reconnect attempts
- inability to continue normal participation using only an old certificate

## 11.4 Re-admission

Transitloom v1 treats re-admission as a fresh authorization event, not an automatic restoration of prior transient session state.

A re-admitted node may participate again only after the proper admission process succeeds.

---

## 12. Node certificate issuance and renewal

## 12.1 Coordinator intermediate issuance role

Authorized coordinators issue node certificates through coordinator intermediates chained to the Transitloom root.

## 12.2 Renewal path

Node certificate renewal may occur through an authorized coordinator even when coordinator access is obtained through relay-assisted control connectivity, subject to policy and trust enforcement.

## 12.3 Renewal independence from prior issuer

Transitloom v1 should not require a node to renew only through the coordinator that previously issued its current certificate.

Any authorized coordinator intermediate under the Transitloom root may be used according to policy.

## 12.4 Certificate vs admission validity

A node certificate proves identity.

A valid admission token proves current authorization.

Transitloom control logic must not confuse these roles.

---

## 13. Admission token model

Transitloom v1 requires short-lived admission tokens for normal participation.

## 13.1 Purpose

Admission tokens exist to:

- support hard revocation
- separate identity from current authorization
- reduce pressure to use extremely short-lived certificates
- make current participation permission explicit

## 13.2 Control-plane requirement

Coordinators must require a valid admission token during normal node session establishment.

## 13.3 Token freshness

Admission tokens are intentionally short-lived relative to node identity certificates.

Exact formats and lifetimes are defined in the PKI/admission specification.

## 13.4 Session effect

If a node lacks a valid admission token, it must not be treated as a normal admitted participant even if its certificate remains valid.

---

## 14. Service registration

Transitloom is service-oriented.

## 14.1 Services as first-class objects

A service is a mesh-exposed local endpoint or capability that Transitloom can carry.

The control plane is responsible for handling service metadata and registration.

## 14.2 Registration responsibilities

Nodes register their services with coordinators according to policy.

A service registration may include:

- service identity
- service type
- local target metadata
- policy labels
- transport capability declarations
- future extensibility metadata

## 14.3 Genericity

The service model is generic.

WireGuard is a flagship use case, but the service registry must not be fundamentally WireGuard-specific.

---

## 15. Association control

Transitloom uses associations as first-class connectivity objects.

## 15.1 Purpose

Associations define the logical relationship between services that are permitted to communicate across the overlay mesh.

## 15.2 Control-plane responsibilities

The control plane is responsible for:

- distributing association definitions
- enforcing association policy
- exposing eligible relay/path class information
- distributing constraints relevant to route/path setup
- establishing the context in which data-plane forwarding becomes legal

## 15.3 Association scope

An association may carry policy such as:

- allowed source and destination
- allowed service pair
- relay permissions
- direct/intranet/relay eligibility
- scheduling policy
- health policy
- metering considerations
- future transport mode allowances

---

## 16. Discovery model

Transitloom v1 includes both coordinator discovery and node/service discovery.

## 16.1 Coordinator discovery sources

A node may learn coordinators from:

- preconfigured bootstrap coordinators
- coordinator-provided catalogs
- node-provided hints, where policy allows

## 16.2 Trust rule for discovery

Coordinator discovery learned from nodes is treated as a hint, not as trust truth.

Cryptographic trust and coordinator usability still depend on:

- certificate validation
- policy
- current coordinator admissibility and eligibility

## 16.3 Node/service discovery

Nodes may discover other nodes and services subject to policy.

Discovery does not automatically imply authorization to create associations.

## 16.4 Private address sharing

Private or intranet address information has exposure implications.

Its advertisement and use must remain policy-aware and may depend on local node configuration and related rules.

---

## 17. Route and relay coordination responsibilities

## 17.1 Control-plane route assistance

Transitloom v1 control plane assists route/path construction rather than leaving everything to unmanaged local behavior.

Coordinators may assist by distributing:

- eligible peer information
- relay eligibility information
- coordinator catalogs
- service and association metadata
- policy-filtered path candidate classes
- summary health where useful

## 17.2 Data-plane scheduling ownership

Even though coordinators assist with route and relay context, endpoint nodes remain responsible for data-plane scheduling decisions in v1.

The control plane should not own live end-to-end data split decisions for all traffic.

## 17.3 Relay distinction

Control-plane relay and data-plane relay are intentionally distinct policy categories.

Control relay may be easier to allow than data relay.

---

## 18. Keepalive and liveness

## 18.1 Control-plane liveness

Control sessions require liveness handling.

The transport and higher control protocol should support liveness and re-establishment behavior.

## 18.2 Mesh-owned operational liveness

Transitloom prefers mesh-owned operational liveness handling rather than relying entirely on upper application behavior.

## 18.3 Interaction with application keepalive

Application-level keepalive, including WireGuard keepalive, may exist, but Transitloom control-plane and data-plane liveness should not depend on application keepalive alone.

---

## 19. Control-plane state categories

For clarity, Transitloom v1 distinguishes these categories of control-plane state:

### 19.1 Trust state
Examples:
- root authority state
- coordinator intermediate state
- node certificate-related trust state
- admission token-related validity context

### 19.2 Global admission state
Examples:
- pending/admitted/revoked concepts
- related security-sensitive node participation control

### 19.3 Service state
Examples:
- service registrations
- service metadata
- service capability declarations

### 19.4 Association state
Examples:
- permitted service relationships
- relay permissions
- transport policy
- route/path eligibility constraints

### 19.5 Discovery state
Examples:
- coordinator catalogs
- policy-filtered discovery information
- node/service discovery metadata

### 19.6 Health summary state
Examples:
- summary health relevant to route/relay assistance
- not the full globally serialized transport adaptation state

---

## 20. v1 coordinator-local serving policy

Transitloom v1 allows coordinator-local serving policy distinct from global admission state.

This permits behavior such as:

- local concealment-like behavior
- silent drop-like behavior
- local draining or local restrictions
- limited or filtered coordinator visibility

These local serving behaviors must not be confused with global revoke.

Global revoke means the node is no longer admitted network-wide.

Coordinator-local serving policy means one coordinator behaves differently toward a node without changing the node’s global admission status.

---

## 21. Public-facing use case stance

Transitloom v1 keeps the control-plane model generic, but public-facing documentation should explicitly optimize around the flagship use case:

- WireGuard over mesh
- multi-WAN UDP aggregation
- direct + relay-assisted real-world connectivity

This is a documentation and adoption decision, not a control-plane model specialization.

---

## 22. Deferred and future work

The following control-plane-related items are intentionally left for later phases or more detailed specifications:

- exact message schema and wire framing
- exact token format and cryptographic details
- final coordinator operation log structure
- detailed route-object and relay-object definitions
- broader dynamic-routing sophistication beyond v1 needs
- fully general node-to-node control overlays beyond v1 priorities
- deeper service management APIs
- future encrypted generic service carriage implications

---

## 23. Related specifications

This control-plane specification depends on and should remain consistent with:

- `spec/v1-architecture.md`
- `spec/v1-data-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-pki-admission.md`
- `spec/v1-wireguard-over-mesh.md`

---
