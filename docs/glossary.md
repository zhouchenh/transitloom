# Transitloom Glossary

This glossary defines the main terms used across Transitloom documentation and specifications.

These definitions are intentionally short and practical. For normative behavior, see the files under `spec/`.

## Admission

The current permission for a node to participate in the Transitloom network.

Admission is separate from identity. A node may have a valid identity certificate but still not be admitted.

## Admission token

A short-lived signed authorization proving that a node is currently allowed to participate in the Transitloom network.

A valid certificate identifies a node. A valid admission token proves current participation permission.

## Association

The logical connectivity object between services.

An association defines the context in which Transitloom is allowed to carry traffic between a source service and a destination service.

## Bootstrap coordinator

A coordinator entry configured locally so a node or coordinator can initially contact the Transitloom control network.

Bootstrap information is a connection hint, not trust truth.

## Control plane

The part of Transitloom responsible for identity, admission, service registration, association setup, discovery, policy distribution, and coordination.

It determines what is legal and how the overlay is coordinated.

## Coordinator

An admin-operated Transitloom infrastructure component that handles node-facing control-plane functions such as trust enforcement, admission checks, service registration, association control, discovery assistance, and relay assistance.

## Coordinator catalog entry

A discovery-facing representation of a coordinator, including information such as identity, advertised endpoints, capabilities, and visibility metadata.

## Coordinator intermediate

An intermediate CA issued under the Transitloom root and used by a coordinator to issue node certificates.

## Data plane

The part of Transitloom that carries actual service traffic.

In v1, the primary data-plane focus is raw UDP carriage.

## Direct path

A path between two nodes that does not use an intermediate relay hop.

A direct path may be public or private/intranet.

## Discovery hint

Non-authoritative discovered information learned indirectly, such as from another node.

Hints may be useful, but they do not override trust or policy.

## Health summary

A summarized view of operational quality for a subject such as a path or relay.

A health summary may include information such as latency, loss, jitter, goodput, freshness, and confidence.

## Identity

The stable authenticated identity of a node.

In Transitloom, identity is established using certificates and key material.

## Local ingress

A Transitloom-provided local endpoint used by a local application to send traffic into the mesh toward a remote service.

This is distinct from the local target.

## Local target

The actual local endpoint where Transitloom delivers inbound carried traffic for a service.

For a WireGuard service, this is typically the WireGuard interface’s real local `ListenPort`.

## Node

A Transitloom participant that exposes one or more services and uses the overlay transport.

A node may be an end-user device, server, router, gateway, appliance, or another managed system.

## Node certificate

The certificate-backed identity object for a node.

A node certificate proves who the node is. It does not by itself prove that the node is currently allowed to participate.

## Path candidate

A candidate path that Transitloom may use for control or data transport.

Examples include direct public paths, direct intranet paths, coordinator relay paths, and node relay paths.

## Relay

An intermediate participant that helps carry traffic when direct connectivity is unavailable, undesirable, or policy-constrained.

Transitloom recognizes at least two relay classes:
- coordinator relay
- node relay

## Relay candidate

A relay-capable participant that may be used as part of a path.

A relay candidate is not the same thing as a fully resolved path candidate.

## Root authority

The trust-anchor role for a Transitloom deployment.

The root authority manages the private PKI trust base and coordinator intermediate lifecycle. It is not a normal node-facing coordinator target.

## Scheduler

The Transitloom logic that decides how eligible paths are used for carrying traffic.

In v1, the default scheduler is weighted burst/flowlet-aware, with per-packet striping allowed only when paths are closely matched.

## Service

A logical capability exposed by a node to the mesh.

Examples include a WireGuard UDP listener or another raw UDP endpoint.

## Service binding

The mapping from a logical service to its concrete local runtime endpoint.

A service binding tells Transitloom where inbound carried traffic should be delivered.

## Service instance

The concrete local realization of a service on a node.

In many cases this maps closely to one running local endpoint, but it is still useful to distinguish the logical service from its concrete instance.

## Service registry

The coordinator-managed catalog of known services and related metadata.

It serves both human-facing discovery and machine-facing association and policy behavior.

## WireGuard-over-mesh

The flagship Transitloom v1 use case in which standard WireGuard instances use Transitloom local ingress endpoints as peer endpoints, while Transitloom carries the encrypted WireGuard UDP traffic across the overlay mesh.

## Zero in-band overhead

For Transitloom v1 raw UDP carriage, this means Transitloom does not insert extra payload-visible shim headers into the carried raw UDP data path.

Instead, forwarding relies on state such as service bindings, associations, and path context.
