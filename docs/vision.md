# Transitloom Vision

Transitloom exists to make overlay transport practical on real networks.

Modern connectivity is messy. Nodes sit behind NAT or CGNAT. Public addresses change. Some links are fast, some metered, some unstable, and some only work part of the time. Direct connectivity may exist, or it may not. Intranet paths may be available but underused. Relay paths may be necessary. Multiple WAN links may exist on one side, both sides, or neither in a predictable way.

Transitloom is being built for this reality.

## The problem Transitloom addresses

Many existing tools assume a cleaner network than people actually have. They often expect:

- one stable endpoint
- one preferred path
- one simple trust model
- one obvious underlay route
- one application-specific integration story

That is often not enough.

Transitloom is intended for environments where:

- nodes may have multiple uplinks
- direct paths and relayed paths may coexist
- private and public reachability may both matter
- traffic may need to move across changing path conditions
- operators need strong centralized control over trust and participation
- applications should not have to understand all the transport complexity underneath

## The core idea

Transitloom is a coordinator-managed overlay mesh transport platform.

It separates:

- **identity and authorization**
- **control and coordination**
- **service modeling**
- **data transport**

The project is designed so that applications can use stable local service bindings while Transitloom handles the real network complexity underneath.

That means applications should be able to benefit from:

- direct paths
- intranet paths
- relay-assisted paths
- multi-WAN aggregation
- policy-controlled overlay connectivity

without being rewritten to understand the mesh itself.

## The first flagship use case

Transitloom’s first flagship use case is **WireGuard over mesh**.

That use case matters because it combines many of the real-world constraints Transitloom is meant to address:

- UDP transport
- NAT and CGNAT complications
- multi-WAN opportunity
- sensitivity to path quality
- a need for operational simplicity at the application edge

Transitloom is not intended to replace WireGuard. Instead, it aims to provide a transport substrate under it.

WireGuard can remain standard. Transitloom can provide:

- stable local endpoints
- direct and relay-assisted connectivity
- overlay path selection
- liveness handling
- multi-WAN aggregation behavior

while keeping the core model generic enough to support other services later.

## Why generic service carriage matters

Even though WireGuard is the flagship starting point, Transitloom is not meant to become a one-protocol product.

The long-term direction is broader:

- represent services generically
- represent relationships between services as associations
- enforce trust and policy centrally
- carry service traffic over the best legal overlay paths available

This allows the platform to stay useful beyond a single protocol, while still focusing v1 on a concrete and demanding real-world workload.

## What Transitloom values

Transitloom is being designed around a few strong priorities.

### Real-world usefulness over idealized elegance

Transitloom is for the networks people actually have, not only the ones they wish they had.

### Performance where it matters

The project places special weight on high-performance raw UDP carriage and practical multi-WAN aggregation.

### Strong operational control

Transitloom assumes that authorized admins should be able to control admission, revocation, trust, and coordinator behavior across the network.

### Clear separation of concerns

Identity is not the same thing as current permission to participate.  
Control traffic is not the same thing as service traffic.  
Service modeling is not the same thing as packet forwarding.

### Generic foundation, concrete first use case

Transitloom should remain architecturally generic, while still being unapologetically optimized in documentation and examples for the first flagship deployment model.

## What Transitloom is not trying to be on day one

Transitloom is not trying to solve every overlay or service-mesh problem in v1.

It is not trying to become:

- an everything-platform for every protocol immediately
- a full unconstrained routed mesh from day one
- a marketing-first abstraction with weak operational behavior underneath
- a product that sacrifices predictable performance just to maximize topology flexibility

The first priority is narrower and more practical:

**make multi-WAN-capable raw UDP overlay transport work well in real deployments**

That focus is deliberate.

## Long-term direction

If Transitloom succeeds at its first job, it creates a strong foundation for future growth.

That future may include:

- richer service types
- encrypted generic data carriage
- broader relay and route models
- deeper operational tooling
- more advanced policy and discovery behavior

But those only matter if the core foundation is sound.

The project’s long-term value depends on building the transport, trust, and service model correctly first.

## The vision in one sentence

Transitloom aims to become a practical transport fabric for overlay connectivity on real networks, starting with high-performance WireGuard-over-mesh and multi-WAN raw UDP carriage, and growing from that solid base into a broader coordinator-managed service transport platform.
