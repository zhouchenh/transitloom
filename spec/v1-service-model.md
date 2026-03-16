# Transitloom v1 Service Model Specification

## Status

Draft

This document specifies the Transitloom v1 service model. It defines the core object model for services, service instances, associations, local bindings, policy attachment points, discovery semantics, and the relationship between the service model and the control plane and data plane.

This document is intentionally generic. WireGuard is the flagship v1 workload, but the service model must not be fundamentally WireGuard-specific.

---

## 1. Purpose

Transitloom is service-oriented.

The service model exists to provide a stable abstraction above the data plane and below user-facing application workflows. It defines how local application endpoints become mesh-exposed capabilities, how those capabilities are described, how associations are formed between them, and how policy is attached.

The service model is responsible for answering questions such as:

- What is a service?
- How is a service identified?
- What is the difference between a service definition and a running local endpoint?
- How are remote peers connected to a local service?
- How do discovery and policy interact with services?
- How do services map to local ports and data-plane associations?

---

## 2. v1 design goals

### 2.1 Primary goals

- Provide a generic model for UDP-carried services
- Support stable local bindings for real applications
- Support multiple service instances per node
- Support named services and machine-readable service identity
- Support policy and discovery without hard-coding one protocol
- Support WireGuard-over-mesh cleanly without making WireGuard special in the core model

### 2.2 Secondary goals

- Leave room for future TCP-capable services
- Leave room for future encrypted service transport profiles
- Support both human-facing and machine-facing registry use
- Keep the service model stable even if routing and transport evolve later

---

## 3. Non-goals for v1

The v1 service model does not aim to provide:

- a full service mesh API ecosystem
- advanced L7 service semantics
- protocol-specific service grammars for every supported application
- arbitrary service-type auto-discovery without policy
- fully dynamic application reconfiguration for every consumer from day one

---

## 4. Core concepts

Transitloom v1 service model is built around these concepts:

- **node**
- **service**
- **service instance**
- **service binding**
- **association**
- **local ingress**
- **local target**
- **capability**
- **policy**

---

## 5. Nodes and services

## 5.1 Node

A node is a Transitloom participant that can expose one or more services to the mesh.

A node may host zero, one, or multiple services.

## 5.2 Service

A service is a mesh-visible capability defined on a node.

A service is the logical thing other nodes may discover, authorize, and connect to through associations.

Examples of service concepts in v1 include:

- a WireGuard UDP listener
- a generic raw UDP endpoint
- a future service that is not yet implemented in transport but may exist in schema

## 5.3 Service instance

A service instance is the concrete locally configured realization of a service on a node.

A service instance includes the local runtime details that make the service usable on that node.

### Distinction
- **Service** is the logical object
- **Service instance** is the concrete local realization

In many deployments there may be a 1:1 mapping, but the distinction is still useful.

---

## 6. Service identity

## 6.1 Service identity requirements

Each service must have a stable identity within the node scope.

The identity must be sufficient to distinguish:

- one service from another on the same node
- one service instance from another where multiple instances exist
- one remote association target from another

## 6.2 Recommended identity shape

Transitloom v1 should treat service identity as a structured identifier including at least:

- node identity or node scope
- service name or service ID
- service type

The exact serialization format is left to implementation and API schemas.

## 6.3 Multiple services per node

Transitloom v1 explicitly supports multiple services per node.

This includes multiple WireGuard service instances on one node.

Each such service instance must have its own independent service identity.

---

## 7. Service types

## 7.1 Primary v1 service type

The primary v1 service type is:

- **raw UDP**

## 7.2 WireGuard position

WireGuard is the flagship v1 use case, but in the service model it is represented as a generic UDP-carried service.

Transitloom v1 should not require that the service model encode deep WireGuard-specific protocol semantics.

## 7.3 Future service capability declaration

The service model may include capability declarations for future transport/service types, even if they are not yet implemented in the v1 data plane.

This includes future concepts such as:

- TCP-capable services
- encrypted transport support
- future service transport profiles

Such declarations in v1 are schema-level capability metadata, not proof that the transport behavior is already implemented.

---

## 8. Service metadata

A service may carry metadata used by humans, coordinators, nodes, and policy engines.

## 8.1 Metadata categories

A service may include metadata such as:

- human-readable name
- service type
- labels or tags
- policy labels
- transport capability declarations
- relay eligibility preferences
- local exposure constraints
- notes or descriptions

## 8.2 Human-facing and machine-facing use

The service registry in Transitloom v1 is both:

- human-facing
- machine-facing

So the service model must support both:
- meaningful naming and discovery
- machine-readable policy and association use

---

## 9. Service bindings

## 9.1 Purpose

A service binding maps a logical service to its local runtime endpoint.

## 9.2 Local target

A local target is the actual local endpoint that receives incoming carried traffic for a service.

For raw UDP services, this is typically a local UDP destination such as:

- `127.0.0.1:<port>`
- `[::1]:<port>`
- another explicitly allowed local address/port

## 9.3 Binding fields

A service binding for v1 should be able to express at least:

- service identity
- service type
- local target address/port
- local bind family or preference where useful
- capability metadata
- policy attachment references

---

## 10. Local ingress model

Transitloom distinguishes between the **local target** of a service and **local ingress** endpoints used by local applications to send traffic into the mesh.

## 10.1 Local target

The local target is where Transitloom delivers inbound carried traffic for the local service.

## 10.2 Local ingress

A local ingress is a Transitloom-provided local endpoint used by the application or local system to send traffic toward a remote service over the mesh.

## 10.3 Why the distinction matters

This distinction is critical for v1 use cases such as WireGuard-over-mesh.

Example:
- a local WireGuard instance may listen on one real UDP port
- Transitloom may expose separate local ingress ports for sending to multiple remote WireGuard peers

These are not the same thing and must not be modeled as the same object.

---

## 11. Association model

## 11.1 Association purpose

An association is the logical connectivity object between services.

Transitloom data-plane forwarding is legal only within association context established and authorized by the control plane.

## 11.2 Association scope

An association relates at least:

- one source service
- one destination service

An association may behave bidirectionally in practice, but the model must remain explicit enough to apply policy and bindings correctly.

## 11.3 Association contents

An association should be able to express:

- source service identity
- destination service identity
- permitted path classes
- permitted relay classes
- scheduling policy
- health policy
- local ingress bindings where relevant
- policy references
- metering constraints where relevant

## 11.4 Association and legality

The service model does not itself forward traffic. It defines the identity and policy context that the control plane uses to make forwarding legal and the data plane uses to forward correctly.

---

## 12. Service registry

## 12.1 Registry purpose

The Transitloom service registry is the authoritative catalog of known service objects and their metadata within the coordinator-managed control system.

## 12.2 Registry roles

The service registry serves both:

- human-facing discovery and naming
- machine-facing association and policy usage

## 12.3 Registry contents

The registry should be able to hold at least:

- service identities
- service types
- service metadata
- service capability declarations
- node ownership/scope
- local binding metadata where appropriate
- association-relevant information

## 12.4 Registry and discovery

Presence in the registry does not automatically imply discoverability to all nodes.

Discovery remains policy-filtered.

---

## 13. Discovery semantics

## 13.1 Service discovery is policy-aware

A node may only discover services it is allowed to know about under current policy.

## 13.2 Discovery does not imply use

Discovering a service does not automatically authorize:

- association creation
- relay usage
- direct path use
- private address sharing

These remain controlled separately.

## 13.3 Human-facing discovery

The model should support human-facing listing and naming of services for operational and administrative use.

## 13.4 Machine-facing discovery

The model should also support machine-facing selection and association construction.

---

## 14. Policy attachment points

The service model must support policy at multiple levels.

## 14.1 Minimum policy attachment points

Transitloom v1 should be able to attach policy to:

- nodes
- services
- associations

## 14.2 Examples of service-level policy

Service-level policy may include:

- whether the service is discoverable
- whether private/intranet candidates may be advertised
- whether relay is allowed in principle
- whether future service classes are permitted
- whether the service is available to specific peer groups

## 14.3 Examples of association-level policy

Association-level policy may include:

- which path classes are allowed
- which relay classes are allowed
- which scheduling profile is used
- whether metered paths may participate
- whether intranet candidates may be preferred
- whether the association is currently active or disabled

---

## 15. Capability declarations

## 15.1 Purpose

Capability declarations allow the service model to describe what a service is intended to support, even if not every capability is active in v1 transport.

## 15.2 v1 behavior

Transitloom v1 may allow a service to advertise future capability declarations such as:

- UDP support
- future TCP support
- future encrypted carriage support

This is allowed at the schema/model level, even if the transport is not yet implemented.

## 15.3 Important rule

Capability declaration is not the same thing as active transport availability.

The system must not assume a declared future capability is actually usable in the v1 transport path.

---

## 16. WireGuard-over-mesh service mapping

WireGuard is the flagship v1 use case and should map cleanly onto the generic service model.

## 16.1 WireGuard service representation

A WireGuard service is modeled as:

- a service of UDP-carried type
- with its own service identity
- with a local target bound to the local WireGuard listen port

## 16.2 Multiple WireGuard services

Transitloom v1 supports multiple WireGuard service instances per node.

Each instance must have:

- its own service identity
- its own local target binding
- its own association set

## 16.3 Remote peer carriage

For WireGuard-over-mesh, Transitloom may expose one stable local ingress per remote peer/service association.

This is an application-facing ingress detail, not the same thing as the service’s local target listen port.

---

## 17. Stable local ingress behavior

## 17.1 Requirement

Transitloom v1 should provide deterministic and stable local ingress bindings by default where the use case requires them.

## 17.2 Why this matters

For applications like WireGuard, stable local ingress ports make configuration predictable and persistent across restarts.

## 17.3 Scope

This requirement is especially important for service associations intended to be referenced from static or semi-static application configuration.

---

## 18. Genericity vs flagship UX

## 18.1 Core model

The core service model remains generic.

It should not encode protocol-specific WireGuard semantics as a required foundation.

## 18.2 Public-facing documentation

Transitloom v1 documentation and examples should still explicitly optimize around the flagship use case:

- WireGuard over mesh
- multi-WAN raw UDP aggregation
- stable local peer bindings

This is a UX and adoption stance, not a core model limitation.

---

## 19. Service lifecycle

The exact full lifecycle schema is refined elsewhere, but a service should conceptually move through states such as:

- defined
- registered
- discoverable
- associated
- active
- disabled
- removed

The service model must support the distinction between:
- a service existing
- a service being discoverable
- a service being usable in an active association

---

## 20. Relationship to control plane and data plane

## 20.1 Control plane responsibilities

The control plane is responsible for:

- service registration
- service discovery
- association distribution
- service-related policy distribution
- legality of relay/path usage

## 20.2 Data plane responsibilities

The data plane is responsible for:

- carrying actual service traffic
- using service/association state installed by the control plane
- mapping ingress traffic to the correct association and local target

## 20.3 Service model as contract

The service model is the contract between:
- control-plane identity/policy
- data-plane forwarding behavior
- application-facing local bindings

---

## 21. v1 constraints summary

Transitloom v1 service model includes:

- generic service identities
- multiple services per node
- multiple WireGuard services per node
- generic UDP-focused modeling
- human-facing and machine-facing registry support
- stable local ingress behavior where needed
- policy attachment at node/service/association levels
- future capability declaration support

Transitloom v1 service model does not include:

- deep protocol-specific service semantics
- full L7 service mesh behavior
- assumption that every declared future capability is already transport-usable
- automatic authorization from discovery alone

---

## 22. Future directions

The following are intentionally deferred:

- richer service-type taxonomies
- finalized TCP service behavior
- richer encrypted service profiles
- automatic application config integration helpers
- deeper service lifecycle APIs
- more advanced service health semantics beyond transport-facing needs

---

## 23. Related specifications

This service-model specification depends on and should remain consistent with:

- `spec/v1-architecture.md`
- `spec/v1-control-plane.md`
- `spec/v1-data-plane.md`
- `spec/v1-pki-admission.md`
- `spec/v1-wireguard-over-mesh.md`

---
