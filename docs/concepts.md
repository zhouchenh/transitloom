# Transitloom Concepts

This document introduces the main concepts used throughout Transitloom.

It is a conceptual guide, not a protocol specification. For normative details, see the files under `spec/`.

## Overlay mesh

Transitloom is an overlay mesh transport platform.

That means Transitloom creates a managed transport layer above the real network underneath. The real network may include:

- public Internet paths
- private intranet paths
- NAT or CGNAT environments
- multiple WAN uplinks
- relays
- changing endpoint addresses

Transitloom uses control, policy, and path selection to carry traffic across that underlying network.

## Root authority

The root authority is the trust anchor for a Transitloom deployment.

Its main role is to anchor the Transitloom private PKI and support coordinator intermediate lifecycle operations. It is not meant to be a normal coordinator target for end-user nodes.

In practice, the root authority exists to support trust, not to serve as an ordinary participant in day-to-day mesh traffic.

## Coordinator

A coordinator is an admin-operated infrastructure component in the Transitloom network.

Coordinators are responsible for things such as:

- trust and admission enforcement
- service registration and discovery
- policy distribution
- coordinator-assisted route and relay setup
- control-plane participation
- certificate issuance through coordinator intermediates
- admission-token validation
- relay assistance where allowed

A deployment may have multiple coordinators for HA, reachability, or operational reasons.

## Node

A node is a Transitloom participant that exposes one or more services to the mesh and uses the overlay transport.

A node may be:

- an end-user system
- a server
- a router
- a gateway
- an appliance
- another managed host

Nodes use the control plane to authenticate, learn policy, register services, and establish legal associations. They use the data plane to carry actual service traffic.

## Service

A service is the logical capability a node exposes to the mesh.

Examples include:

- a WireGuard UDP listener
- a generic raw UDP endpoint
- future transport-capable services declared in the registry

A service is a logical object, not just a port number. It has identity, type, metadata, and policy context.

## Service instance

A service instance is the concrete local realization of a service on a node.

For example, a WireGuard interface listening on a specific UDP port is a concrete local service instance.

This distinction matters because the logical service model is broader than one local process or one port binding.

## Service registry

The service registry is the catalog of known services and related metadata in the coordinator-managed system.

It serves both:

- human-facing discovery and naming
- machine-facing association and policy behavior

Being present in the service registry does not automatically mean every node can discover or use that service. Discovery and usage are still policy-controlled.

## Local target

The local target is the actual local endpoint where Transitloom delivers inbound carried traffic for a service.

For UDP services, that is usually a local UDP destination such as:

- `127.0.0.1:<port>`
- `[::1]:<port>`
- another explicitly allowed local address and port

For a WireGuard service, the local target is the WireGuard interface’s real UDP `ListenPort`.

## Local ingress

A local ingress is a Transitloom-provided local endpoint used to send traffic into the mesh toward a remote service.

This is especially important for WireGuard-over-mesh.

For example:

- a local WireGuard instance listens on its real local port
- Transitloom exposes separate local ingress ports for remote peers
- WireGuard sends peer traffic to those Transitloom local ingress ports
- Transitloom carries the packets over the mesh to the correct remote service

The local ingress is not the same thing as the local target.

## Association

An association is the logical connectivity object between services.

Associations define the context in which Transitloom is allowed to carry traffic between a source service and a destination service.

An association may define things such as:

- which services are connected
- whether direct paths are allowed
- whether relay is allowed
- which path classes are eligible
- scheduling policy
- health policy
- metering constraints

Transitloom forwarding is legal only inside valid association context.

## Identity

Identity answers the question:

**Who is this node?**

In Transitloom, node identity is established using node certificates and corresponding key material.

Identity is meant to be more stable than short-term network conditions or current participation permission.

## Admission

Admission answers the question:

**Is this node currently allowed to participate?**

Transitloom intentionally separates identity from admission.

A node may have a valid identity certificate but still not be admitted to participate. Admission is governed by current control state and short-lived admission tokens.

## Admission token

An admission token is a short-lived signed proof that a node is currently allowed to participate in the Transitloom network.

This is important because it allows Transitloom to support hard revocation without requiring extremely short-lived node identity certificates.

A valid certificate identifies a node.  
A valid admission token proves current permission to participate.

Transitloom normal node participation requires both.

## Revocation

Revocation removes current participation permission.

In Transitloom v1, revoke is meant to be hard in operational effect:

- revoked nodes should not be able to establish normal control sessions
- revoked nodes should not be able to reconnect normally
- still-valid identity certificates do not override revoke state

This is enforced through current admission checks and admission-token requirements.

## Control plane

The control plane is responsible for coordination and authorization.

It handles things such as:

- identity and admission checks
- service registration
- association setup
- policy distribution
- discovery
- relay assistance
- coordinator interactions
- route/path assistance where needed

The control plane tells the system what is legal and how to coordinate. It is not the raw service traffic path itself.

## Data plane

The data plane carries the actual service traffic.

In Transitloom v1, the primary data-plane focus is raw UDP.

The data plane is responsible for things such as:

- carrying packets for legal associations
- path usage
- relay usage within v1 limits
- scheduling/splitting traffic across eligible paths
- local delivery to the correct service target

The data plane is intentionally separate from the trust/admission logic of the control plane.

## Zero in-band overhead

For Transitloom v1 raw UDP carriage, zero in-band overhead means Transitloom does not add extra payload-visible shim headers in the carried raw UDP path.

Instead, forwarding is based on installed state such as:

- service bindings
- association context
- path/relay state
- local ingress identity

This is important for the primary v1 goal of practical high-performance raw UDP carriage.

## Path

A path is a candidate way to carry traffic between two association endpoints.

Transitloom may consider different classes of paths, such as:

- direct public path
- direct intranet/private path
- coordinator relay path
- node relay path

Not every path is always legal or useful. Path use depends on policy, health, and association context.

## Direct path

A direct path is a path that does not require an intermediate relay hop.

It may be:

- a public direct path
- a private or intranet direct path

Direct paths are generally preferred when they are healthy and policy allows them.

## Relay

A relay is an intermediate participant that helps carry traffic when direct connectivity is unavailable, undesirable, or policy-constrained.

Transitloom recognizes at least two relay classes:

- coordinator relay
- node relay

Control relay and data relay are not identical ideas. Control relay can be broader and more flexible. Data relay is more constrained in v1 because performance matters.

## Single relay hop

Transitloom v1 limits raw UDP data-plane forwarding to:

- direct paths
- or one relay hop only

This is a deliberate constraint. It helps preserve better aggregation behavior, simpler observability, and more predictable performance.

## Scheduling

Scheduling is how Transitloom decides how to distribute traffic across eligible paths.

In Transitloom v1, data-plane scheduling is endpoint-owned. That means the association endpoints decide how traffic is split, rather than arbitrary relays independently reshaping traffic.

The v1 default scheduler is:

- weighted burst/flowlet-aware scheduling

Per-packet striping is allowed only when paths are closely matched.

## Path health

Path health is the system’s view of whether a path is suitable for carrying traffic.

It may consider inputs such as:

- latency
- jitter
- loss
- effective goodput
- metering
- relay penalty
- administrative policy

Transitloom uses these signals to decide which paths should be active, degraded, standby, or probe-only.

## Metered path

A metered path is a path where traffic volume matters economically or operationally.

Transitloom treats metering as a per-path property, not only as a node property.

This affects probing, active capacity tests, and path scoring.

## Discovery

Discovery is how nodes learn about coordinators, nodes, and services.

Transitloom supports discovery, but discovery does not automatically imply permission to use what was discovered.

Discovery remains policy-controlled.

Coordinator discovery learned from nodes is treated as a hint, not as trust truth.

## Private address sharing

Private address sharing refers to advertising intranet or private network candidates that may be useful for direct paths.

These can be very valuable for performance, but they also expose topology information. Transitloom therefore keeps private-address sharing policy-aware.

## WireGuard-over-mesh

WireGuard-over-mesh is Transitloom’s flagship v1 use case.

In this model:

- WireGuard remains standard
- WireGuard listens on its own normal local UDP `ListenPort`
- Transitloom provides stable local loopback ingress endpoints per remote peer association
- WireGuard sends to those local Transitloom endpoints
- Transitloom carries the encrypted WireGuard UDP traffic across the mesh
- inbound carried packets are delivered to the local WireGuard service

This lets WireGuard benefit from direct paths, relay paths, and multi-WAN behavior without requiring WireGuard protocol changes.

## Generic core, flagship use case

Transitloom keeps its core architecture generic.

That means the platform is not built as a WireGuard-only special case. Services, associations, control, and data transport are modeled generically.

At the same time, Transitloom v1 documentation and examples are intentionally optimized around WireGuard-over-mesh because it is the first flagship use case and a demanding real-world workload.

## Summary

The key concepts to keep in mind are:

- Transitloom is a coordinator-managed overlay mesh transport platform
- nodes expose services to the mesh
- services connect through associations
- control plane manages identity, admission, policy, and coordination
- data plane carries the actual traffic
- identity and current participation permission are separate
- raw UDP v1 aims for zero in-band overhead
- WireGuard-over-mesh is the flagship use case, but the core model remains generic
