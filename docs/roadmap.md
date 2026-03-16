# Transitloom Roadmap

This roadmap describes the intended direction of Transitloom at a high level.

It is not a promise of delivery dates. Priorities may change as the architecture is refined, implementation progresses, and real-world testing reveals what matters most.

## Overall strategy

Transitloom is being developed in stages.

The strategy is:

1. define a strong architectural foundation
2. implement the minimum useful coordinator-managed overlay transport
3. prove the primary real-world use case
4. expand carefully without breaking the core model

The first priority is not to build every possible mesh feature. The first priority is to make the foundation solid enough that the flagship use case works well.

## Near-term priorities

The current near-term focus is on architecture and specification work.

This includes:

- freezing the v1 architecture baseline
- defining the control-plane model
- defining the raw UDP data-plane model
- defining the service and association model
- defining the PKI and admission model
- defining the WireGuard-over-mesh model
- clarifying the v1 scope boundary

The goal of this phase is to reduce architecture drift before implementation grows.

## Phase 1: foundation and architecture

The first major phase of Transitloom is design-first.

### Focus areas

- coordinator, root authority, and node roles
- trust model
- admission and revocation
- coordinator-managed service registry
- association model
- direct and relay path model
- multi-WAN aggregation-oriented scheduling
- WireGuard-over-mesh workflow
- policy boundaries
- observability expectations

### Expected outputs

- architecture specifications
- concept and vision documents
- repository structure
- initial project conventions
- implementation plan for the first working milestone

This phase is considered successful when Transitloom has a coherent, reviewable v1 design baseline.

## Phase 2: minimal working control plane

The next phase is a minimal but real control plane.

### Focus areas

- QUIC + mTLS primary control transport
- TCP + mTLS fallback
- node identity and certificate handling
- short-lived admission tokens
- coordinator authentication and trust enforcement
- service registration basics
- association distribution basics
- initial coordinator discovery
- basic administrative control path

### Expected outputs

- working coordinator binary
- working node binary with control-plane connectivity
- minimal root-authority function
- basic admin tooling
- initial end-to-end admission flow

This phase is considered successful when a node can be admitted, authenticated, registered, and authorized through the Transitloom control system.

## Phase 3: minimal working raw UDP carriage

Once the control plane exists, the next major milestone is a real raw UDP data plane.

### Focus areas

- zero in-band overhead raw UDP forwarding
- direct path carriage
- single relay hop support
- association-bound forwarding legality
- deterministic local ingress bindings
- generic UDP service carriage
- initial path health signals
- endpoint-owned scheduling baseline

### Expected outputs

- direct raw UDP service carriage
- coordinator relay or node relay support in minimal form
- basic path selection behavior
- initial observability for service and association traffic

This phase is considered successful when Transitloom can legally and predictably carry raw UDP traffic over the overlay under coordinator control.

## Phase 4: WireGuard-over-mesh flagship milestone

This is the first major user-facing proof point.

### Focus areas

- stable WireGuard local ingress model
- mapping WireGuard peers to Transitloom associations
- keeping WireGuard standard and unchanged
- Transitloom-owned liveness behavior
- direct and relay-assisted WireGuard carriage
- practical examples and documentation
- real deployment testing

### Expected outputs

- working WireGuard-over-mesh deployment pattern
- example configs
- operational guidance
- early performance validation

This phase is considered successful when WireGuard-over-mesh works well enough to be the first serious public demonstration of Transitloom.

## Phase 5: multi-WAN aggregation refinement

Once basic WireGuard-over-mesh works, the next priority is improving the core reason Transitloom exists.

### Focus areas

- weighted burst/flowlet-aware scheduler implementation
- conditional per-packet striping for closely matched paths
- path scoring refinement
- metered vs unmetered behavior
- direct vs relay tradeoffs
- relay fallback hysteresis
- throughput and reordering analysis
- multi-WAN real-world tuning

### Expected outputs

- stronger aggregation performance
- better scheduling stability
- clearer observability for path decisions
- more predictable behavior under mixed path conditions

This phase is considered successful when Transitloom shows convincing practical value for multi-WAN raw UDP aggregation.

## After v1 foundation

Once the core v1 foundation is proven, Transitloom may expand in several directions.

These are important possibilities, but they are intentionally secondary to getting the core transport right first.

### Possible next directions

- richer relay and route selection
- better operational tooling
- more detailed metrics and status surfaces
- helper tooling for application integration
- future encrypted UDP carriage
- future generic TCP carriage
- richer service types
- broader policy and discovery features

These should be evaluated only after the primary v1 transport and trust model are proven.

## Deferred items

The following ideas are intentionally not immediate priorities unless the architecture clearly supports them:

- arbitrary multi-hop raw UDP data forwarding
- broad service-mesh ambitions before UDP aggregation is solid
- deep application-specific helpers in the core model
- highly polished management UX before the core system behavior is correct
- advanced routing flexibility that harms predictability for the flagship use case

## Success criteria for the early project

Transitloom should be considered on the right track if it can demonstrate:

- a clear trust and admission model
- strong coordinator-managed control behavior
- legal and observable service/association behavior
- zero-overhead raw UDP carriage
- practical single-relay-hop support
- working WireGuard-over-mesh examples
- meaningful multi-WAN aggregation behavior
- stable enough scheduling under real path diversity

## Long-term direction

Transitloom is intended to grow from a strong transport foundation into a broader coordinator-managed service transport platform.

That growth should be earned through real working behavior, not assumed from the start.

The long-term vision remains broad, but the near-term roadmap stays disciplined:

**make the core transport, trust, and service model work well first**
