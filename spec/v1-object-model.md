# Transitloom v1 Object Model Specification

## Status

Draft

This document defines the core object model for Transitloom v1. It translates the architecture, control-plane, data-plane, service-model, PKI/admission, and WireGuard-over-mesh specifications into concrete logical objects and relationships.

This is a conceptual and structural specification. It does not define final wire encoding, final API payload shape, or final Go struct syntax. Its purpose is to freeze the core entities, their responsibilities, their relationships, and their lifecycle expectations before implementation expands.

---

## 1. Purpose

Transitloom needs a stable object model so that:

- specifications use consistent terms
- control-plane and data-plane logic attach to the same conceptual objects
- future configuration and API design stay aligned
- implementation can be split into packages without semantic drift

The object model is the shared vocabulary and structure for the system.

---

## 2. Design principles

The v1 object model follows these principles:

- keep identity separate from current authorization
- keep control-plane objects separate from data-plane runtime state
- make services and associations first-class
- keep the model generic rather than WireGuard-specific
- represent relay and path candidates explicitly
- keep security-sensitive global objects distinct from local operational state
- allow stable local bindings for application-facing use cases
- support future transport/service expansion without redesigning the core entity model

---

## 3. Object categories

Transitloom v1 objects fall into these broad categories:

- **identity and trust objects**
- **global authority objects**
- **infrastructure objects**
- **service and association objects**
- **path and relay objects**
- **local runtime and binding objects**
- **operational state and observation objects**

---

## 4. Top-level object inventory

The core v1 objects are:

- `RootAuthority`
- `Coordinator`
- `CoordinatorIntermediate`
- `Node`
- `NodeCertificate`
- `AdmissionToken`
- `GlobalOperation`
- `Service`
- `ServiceBinding`
- `LocalIngressBinding`
- `Association`
- `PathCandidate`
- `RelayCandidate`
- `CoordinatorCatalogEntry`
- `DiscoveryHint`
- `HealthSummary`

Additional implementation-specific runtime objects may exist later, but these are the minimum conceptual objects that the design should preserve.

---

## 5. RootAuthority

## 5.1 Purpose

`RootAuthority` represents the root trust anchor role for a Transitloom deployment.

It is responsible for:

- anchoring the Transitloom private PKI
- issuing or managing coordinator intermediates
- supporting special trust lifecycle operations
- staying outside the normal end-user coordinator path

## 5.2 Conceptual fields

A `RootAuthority` should conceptually include:

- stable root authority identity
- trust anchor certificate metadata
- lifecycle state
- administrative metadata
- issuance capability metadata
- validity horizon metadata

## 5.3 Notes

A `RootAuthority` is not a normal end-user coordinator target and should not be modeled as interchangeable with `Coordinator`.

---

## 6. Coordinator

## 6.1 Purpose

`Coordinator` represents an admin-operated control-plane infrastructure participant.

It is responsible for:

- authenticating node sessions
- enforcing admission and revoke state
- issuing node certificates through its intermediate
- validating admission tokens
- distributing discovery, policy, and association context
- assisting route and relay coordination
- relaying control traffic where allowed
- optionally relaying data traffic where allowed

## 6.2 Conceptual identity

A `Coordinator` should have a stable coordinator identity distinct from its transient network addresses.

## 6.3 Conceptual fields

A `Coordinator` should conceptually include:

- coordinator ID
- display name or label
- region / location metadata
- failure-domain metadata
- capability metadata
- relay capability metadata
- control transport endpoints
- discovery visibility metadata
- lifecycle / administrative state
- reference to issuing/trust objects
- policy metadata

## 6.4 Relationship notes

A `Coordinator` may have:

- one current `CoordinatorIntermediate`
- zero or more advertised transport endpoints
- zero or more `CoordinatorCatalogEntry` records as seen by consumers
- control-plane relationships with nodes and other coordinators

---

## 7. CoordinatorIntermediate

## 7.1 Purpose

`CoordinatorIntermediate` represents an intermediate CA issued under the Transitloom root and used by a coordinator for node certificate issuance.

## 7.2 Responsibilities

It exists to:

- issue node certificates
- decouple routine node issuance from root availability
- support intermediate overlap and lifecycle management

## 7.3 Conceptual fields

A `CoordinatorIntermediate` should conceptually include:

- intermediate ID
- owning coordinator ID
- issuer root identity
- certificate metadata
- validity window
- lifecycle state
- replacement/overlap metadata
- issuance capability state

## 7.4 Relationship notes

A `CoordinatorIntermediate` belongs to one coordinator role at a time, but historical rollover may produce multiple intermediate objects over time.

---

## 8. Node

## 8.1 Purpose

`Node` represents a Transitloom participant that may expose one or more services and participate in the overlay mesh.

A node may correspond to:

- an end-user device
- a server
- a router
- a gateway
- an appliance
- another managed participant

## 8.2 Identity model

A node has a stable node identity independent of its transient network addresses.

## 8.3 Conceptual fields

A `Node` should conceptually include:

- node ID
- display name or label
- identity anchor metadata
- administrative metadata
- local policy metadata
- discovery metadata
- capability metadata
- lifecycle/admission summary
- zero or more services
- zero or more local addresses / candidate transport attachments
- group/tag metadata where applicable

## 8.4 Important distinction

A `Node` is not the same thing as:

- a `Service`
- a specific local network address
- a single transport endpoint
- a certificate
- an admission token

Those are separate objects.

---

## 9. NodeCertificate

## 9.1 Purpose

`NodeCertificate` represents the certificate-backed identity object for a node.

It answers:

- who the node is

It does not answer:

- whether the node is currently allowed to participate

## 9.2 Conceptual fields

A `NodeCertificate` should conceptually include:

- node ID
- certificate identity metadata
- issuer intermediate ID
- root chain metadata
- validity window
- key usage / role metadata
- lifecycle state
- issuance metadata

## 9.3 Relationship notes

A node may accumulate a history of `NodeCertificate` objects over time due to renewal.

---

## 10. AdmissionToken

## 10.1 Purpose

`AdmissionToken` represents a short-lived authorization object proving current participation permission.

It answers:

- is this node currently allowed to participate

## 10.2 Conceptual fields

An `AdmissionToken` should conceptually include:

- token ID
- node ID
- issuer coordinator identity
- issuance timestamp
- expiry timestamp
- token lifecycle state
- admission-state linkage metadata
- policy scope metadata where needed

## 10.3 Relationship notes

A node may receive many admission tokens over time. Tokens are intentionally short-lived relative to node certificates.

---

## 11. GlobalOperation

## 11.1 Purpose

`GlobalOperation` represents an ordered, security-sensitive administrative operation in the coordinator network.

This object is critical for:

- admission
- revoke
- trust-related changes
- other globally significant state changes

## 11.2 Why it exists

Transitloom v1 avoids weak ad hoc overwrite semantics for security-sensitive global state.

A `GlobalOperation` makes those changes explicit and orderable.

## 11.3 Conceptual fields

A `GlobalOperation` should conceptually include:

- operation ID
- operation type
- target object reference
- actor/admin identity
- submit coordinator identity
- creation timestamp
- ordering/version metadata
- state (`pending`, `committed`, `rejected`, etc.)
- reason / note metadata
- conflict or reconciliation metadata where needed

## 11.4 Examples

Examples of operation types:

- admit node
- revoke node
- re-admit node
- trust change
- coordinator security-sensitive state update

---

## 12. Service

## 12.1 Purpose

`Service` represents a mesh-visible capability exposed by a node.

Examples include:

- a WireGuard UDP listener
- a generic raw UDP endpoint
- a future TCP-capable service declaration

## 12.2 Service identity

A service has its own identity and is not reducible to only a port number.

## 12.3 Conceptual fields

A `Service` should conceptually include:

- service ID
- owning node ID
- service name
- service type
- labels/tags
- capability declarations
- discovery metadata
- policy metadata
- administrative metadata
- lifecycle state

## 12.4 Relationship notes

A node may own zero or more services.

A service may have one or more bindings and zero or more associations.

---

## 13. ServiceBinding

## 13.1 Purpose

`ServiceBinding` maps a logical service to its concrete local runtime endpoint.

It answers:

- where does Transitloom deliver inbound traffic for this service?

## 13.2 Conceptual fields

A `ServiceBinding` should conceptually include:

- binding ID
- service ID
- owning node ID
- local target address
- local target port
- local target family/protocol metadata
- binding lifecycle state
- local policy metadata
- notes about transport assumptions where needed

## 13.3 Notes

A service binding is not the same thing as a local ingress binding.

`ServiceBinding` is about inbound delivery to the local service target.

---

## 14. LocalIngressBinding

## 14.1 Purpose

`LocalIngressBinding` represents a local application-facing endpoint used to send traffic into the mesh toward a remote service association.

This object is especially important for WireGuard-over-mesh.

## 14.2 Why it exists

Transitloom must distinguish:

- where local inbound service traffic is delivered
- where local applications send outbound peer traffic into the mesh

Those are different roles.

## 14.3 Conceptual fields

A `LocalIngressBinding` should conceptually include:

- ingress binding ID
- owning node ID
- local listen address
- local listen port
- protocol/family metadata
- associated local service ID
- associated remote service or association reference
- stability/determinism metadata
- lifecycle state

## 14.4 Relationship notes

For WireGuard-over-mesh, one service may have multiple `LocalIngressBinding` objects, one per remote peer/service association.

---

## 15. Association

## 15.1 Purpose

`Association` is the core connectivity object between services.

It defines the legal, policy-controlled relationship through which Transitloom is allowed to carry traffic.

## 15.2 Responsibilities

An association is responsible for expressing:

- source and destination service relationship
- allowed path classes
- allowed relay classes
- scheduling policy
- health policy
- metering constraints
- local binding context
- operational state references

## 15.3 Conceptual fields

An `Association` should conceptually include:

- association ID
- source service ID
- destination service ID
- association directionality metadata
- allowed path classes
- allowed relay classes
- scheduler profile
- health policy
- metering policy
- administrative state
- association lifecycle state
- discovery/policy references
- relevant binding references where applicable

## 15.4 Relationship notes

An association is not itself a packet path. It is the policy/legal object under which one or more path candidates may be used.

---

## 16. PathCandidate

## 16.1 Purpose

`PathCandidate` represents a candidate data-plane or control-relevant path between association endpoints.

It is a generic path object, not limited to one transport implementation detail.

## 16.2 Path classes

A `PathCandidate` may represent:

- direct public path
- direct intranet/private path
- coordinator relay path
- node relay path

## 16.3 Conceptual fields

A `PathCandidate` should conceptually include:

- path ID
- associated association ID or eligible association scope
- path class
- source node/service context
- destination node/service context
- relay involvement metadata
- local attachment/bind metadata
- remote tuple metadata
- health summary metadata
- policy metadata
- metering metadata
- scheduler eligibility metadata
- lifecycle state

## 16.4 Important distinction

A `PathCandidate` is a candidate/usable path, not a full historical telemetry stream. High-churn path telemetry does not have to be modeled as globally serialized object state.

---

## 17. RelayCandidate

## 17.1 Purpose

`RelayCandidate` represents a relay-capable intermediate participant that may be used for an association or path class.

It is separate from `PathCandidate` because a relay is not the same thing as the fully resolved path that uses it.

## 17.2 Relay classes

A `RelayCandidate` may be:

- coordinator relay
- node relay

## 17.3 Conceptual fields

A `RelayCandidate` should conceptually include:

- relay candidate ID
- relay class
- underlying coordinator ID or node ID
- eligible source/destination scope
- relay capability metadata
- policy metadata
- health/capacity summary metadata
- administrative state
- lifecycle state

## 17.4 Relationship notes

A `PathCandidate` may reference a `RelayCandidate` when the path class is relayed.

---

## 18. CoordinatorCatalogEntry

## 18.1 Purpose

`CoordinatorCatalogEntry` represents a discoverable description of a coordinator as presented through discovery mechanisms.

It is useful because discovery-facing representation is not always identical to the full internal `Coordinator` object.

## 18.2 Conceptual fields

A `CoordinatorCatalogEntry` should conceptually include:

- coordinator ID
- discovery-visible name/label
- region/location metadata
- advertised transport endpoints
- capability metadata
- relay capability metadata
- trust-chain reference metadata
- discovery visibility scope
- freshness metadata

## 18.3 Notes

A node may learn a `CoordinatorCatalogEntry` from:
- configured bootstrap
- coordinator-provided catalog
- node-provided hint

But discovery does not equal trust; trust still depends on valid PKI and policy.

---

## 19. DiscoveryHint

## 19.1 Purpose

`DiscoveryHint` represents non-authoritative discovered information learned from nodes or other indirect channels.

This allows the model to distinguish:
- authoritative registry/catalog objects
- hint-level information

## 19.2 Examples

Examples include hints about:

- coordinators
- nodes
- services
- private addresses
- relay opportunities

## 19.3 Conceptual fields

A `DiscoveryHint` should conceptually include:

- hint ID
- hint type
- source object or source actor
- hinted target reference or metadata
- freshness metadata
- confidence metadata
- policy visibility metadata
- validation state

## 19.4 Notes

Hints are useful but do not override trust or legality.

---

## 20. HealthSummary

## 20.1 Purpose

`HealthSummary` represents summarized health or quality information about a path, relay, or other operational object.

It exists because Transitloom wants to share useful summaries without making all detailed telemetry strongly global.

## 20.2 Possible usages

A `HealthSummary` may be attached conceptually to:

- a path candidate
- a relay candidate
- a coordinator reachability view
- an association summary

## 20.3 Conceptual fields

A `HealthSummary` should conceptually include:

- summary ID
- subject reference
- latency summary
- loss summary
- jitter summary
- goodput summary
- metering/cost summary
- confidence metadata
- freshness timestamp
- health class/state

## 20.4 Notes

This object is intentionally summary-oriented. It is not a complete telemetry history object.

---

## 21. Object relationships

The most important relationships are:

- `RootAuthority` issues or anchors trust for `CoordinatorIntermediate`
- `Coordinator` owns or uses a `CoordinatorIntermediate`
- `CoordinatorIntermediate` issues `NodeCertificate`
- `Node` presents a `NodeCertificate`
- `Node` obtains an `AdmissionToken`
- `GlobalOperation` changes global security-sensitive state affecting `Node` participation
- `Node` owns one or more `Service`
- `Service` has one or more `ServiceBinding`
- `Service` may have zero or more `LocalIngressBinding`
- `Association` connects services
- `Association` may reference one or more `PathCandidate`
- `PathCandidate` may reference zero or one `RelayCandidate`
- `CoordinatorCatalogEntry` represents discoverable coordinator information
- `DiscoveryHint` may suggest coordinator/node/service/path information
- `HealthSummary` may summarize operational quality for paths, relays, coordinators, or associations

---

## 22. Lifecycle-oriented grouping

For implementation and reasoning purposes, the objects can be grouped as follows.

### 22.1 Slow-changing trust objects
- `RootAuthority`
- `CoordinatorIntermediate`
- `NodeCertificate`

### 22.2 Security-sensitive global authority objects
- `GlobalOperation`
- admission/revoke state attached to `Node`
- `AdmissionToken`

### 22.3 Service model objects
- `Service`
- `ServiceBinding`
- `LocalIngressBinding`
- `Association`

### 22.4 Operational candidate/summary objects
- `PathCandidate`
- `RelayCandidate`
- `CoordinatorCatalogEntry`
- `DiscoveryHint`
- `HealthSummary`

This grouping is useful because not every object belongs in the same replication, persistence, or API layer.

---

## 23. Global vs local object expectations

Transitloom v1 should not treat every object as globally authoritative in the same way.

## 23.1 Strongly global / security-sensitive
These are the most global objects:

- admission/revoke effects on `Node`
- trust roles (`RootAuthority`, `CoordinatorIntermediate`)
- ordered security-sensitive `GlobalOperation`

## 23.2 Locally observed / operationally dynamic
These are more local or dynamic:

- `PathCandidate` detailed health
- `RelayCandidate` capacity/health detail
- `DiscoveryHint`
- some `HealthSummary` instances
- local ingress runtime state

## 23.3 Mixed objects
Some objects are globally meaningful but contain locally relevant details:

- `Coordinator`
- `Service`
- `Association`

These should be handled carefully in implementation so that high-churn runtime state does not force inappropriate global coordination.

---

## 24. WireGuard-over-mesh mapping to the object model

WireGuard remains generic in the object model.

### 24.1 A WireGuard interface maps to:
- `Service`
- `ServiceBinding`

### 24.2 A WireGuard peer relationship maps to:
- `Association`
- `LocalIngressBinding` on the local node
- eligible `PathCandidate` objects

### 24.3 The local WireGuard listen port maps to:
- `ServiceBinding.local target`

### 24.4 The local loopback peer endpoint maps to:
- `LocalIngressBinding.local listen endpoint`

This keeps WireGuard compatible with the generic model while preserving its flagship use-case importance.

---

## 25. Suggested object-level invariants

The implementation should aim to preserve these invariants.

### 25.1 Identity vs authorization invariant
A `NodeCertificate` never implies current participation permission by itself.

### 25.2 Service identity invariant
A `Service` is not identified only by a local port number.

### 25.3 Association legality invariant
Traffic forwarding is legal only inside valid `Association` context.

### 25.4 Binding distinction invariant
`ServiceBinding` and `LocalIngressBinding` are not interchangeable.

### 25.5 Relay distinction invariant
A `RelayCandidate` is not the same thing as a `PathCandidate`.

### 25.6 Discovery hint invariant
A `DiscoveryHint` is not the same thing as trusted or committed registry truth.

---

## 26. Implementation guidance

This document does not mandate exact Go type definitions, but it strongly suggests that implementation should preserve object boundaries clearly.

### 26.1 Avoid collapsing concepts too early

Do not collapse the following into one generic struct too early:

- service vs binding
- node certificate vs admission token
- relay candidate vs path candidate
- coordinator catalog entry vs full coordinator object
- discovery hint vs authoritative object

### 26.2 Keep identifiers explicit

Each top-level object should have an explicit stable identifier appropriate to its scope.

### 26.3 Keep references explicit

Relationships between objects should be representable explicitly rather than inferred only from naming conventions.

---

## 27. Future extensions

The v1 object model should leave room for future objects such as:

- richer route objects
- explicit scheduler profile objects
- TCP service variants
- encrypted carriage profiles
- policy objects with TTL/temporal scope
- richer audit/event objects
- relay accounting objects
- node class/profile objects if later needed

The v1 core object model should remain stable enough that such additions do not require redesigning its foundations.

---

## 28. Related specifications

This object-model specification depends on and should remain consistent with:

- `spec/v1-architecture.md`
- `spec/v1-control-plane.md`
- `spec/v1-data-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-pki-admission.md`
- `spec/v1-wireguard-over-mesh.md`

---
