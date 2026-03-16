# agents/IDENTITY.md

## Purpose

This file defines the project identity of Transitloom.

Its job is to answer:

- what Transitloom is
- what Transitloom is not
- who it is for
- what problem it is trying to solve first
- what kind of product it is becoming

This file is intended to keep coding agents aligned on product identity so they do not accidentally implement a different project than the one this repository is meant to build.

---

## Project identity

Transitloom is a **coordinator-managed overlay mesh transport platform**.

Its v1 identity is centered on:

- high-performance raw UDP service carriage
- practical multi-WAN aggregation
- policy-controlled direct and relay-assisted connectivity
- WireGuard-over-mesh as the flagship use case

Transitloom is intended to be a **transport substrate**, not just a tunnel wrapper, and not just a single-purpose WireGuard helper.

---

## What Transitloom is

Transitloom is:

- an overlay transport system
- coordinator-managed
- service-oriented
- performance-first for the v1 data plane
- built for real-world mixed network conditions
- designed to separate identity, authorization, control, service modeling, and traffic carriage

Transitloom is meant for networks where some or all of the following may be true:

- multiple WAN links exist
- direct and relay-assisted paths may both be useful
- public addresses may be dynamic
- nodes may sit behind NAT or CGNAT
- intranet/private reachability may exist and matter
- strong centralized admission and revocation control is required
- applications should not need to understand the underlying network complexity

---

## What Transitloom is not

Transitloom is **not**:

- a WireGuard protocol fork
- a WireGuard replacement
- a one-protocol product
- a general-purpose service mesh for every protocol in v1
- an unconstrained arbitrary-hop routed overlay in v1 data plane
- a public-Internet-PKI-dependent system
- a “smart relay” project with weak trust boundaries
- a project whose first success criterion is management UX polish instead of transport correctness

Transitloom should not drift into “whatever overlay feature seems cool” without regard to the flagship use case and architecture boundaries.

---

## Primary problem Transitloom solves

The first real problem Transitloom is trying to solve is:

**make multi-WAN-capable raw UDP overlay transport work well in real deployments**

That includes environments with:

- mixed path quality
- dynamic addressing
- NAT or CGNAT constraints
- direct and relay path coexistence
- the need for stable application-facing local endpoints
- the need for coordinator-managed trust and admission

This problem is concrete, difficult, and valuable enough to justify the project by itself.

---

## Flagship use case

The flagship v1 use case is:

**WireGuard over mesh**

This matters because WireGuard-over-mesh exercises many of the exact conditions Transitloom is meant to handle:

- UDP carriage
- endpoint indirection
- sensitivity to path quality
- need for stable local bindings
- potential benefit from multi-WAN aggregation
- direct vs relay tradeoffs
- real-world operator need

Transitloom does not require WireGuard changes.  
Instead, Transitloom provides the transport behavior underneath.

WireGuard remains standard.  
Transitloom handles the mesh.

---

## Product stance

Transitloom’s v1 product stance is:

- generic core model
- WireGuard-first documentation and validation path
- strong trust and admission control
- constrained, performance-focused data-plane scope
- architecture-first implementation discipline

The core should stay generic.  
The docs and examples should still clearly optimize for WireGuard-over-mesh adoption.

That combination is intentional.

---

## Core value proposition

Transitloom’s core value proposition is:

- allow applications to use stable local service bindings
- let Transitloom handle the messy network underneath
- combine direct paths, intranet paths, and relays where appropriate
- enable practical multi-WAN raw UDP transport
- preserve centralized control over trust, participation, and policy

The application should not need to understand:
- public IP churn
- relay choice
- path switching
- coordinator topology
- multi-WAN scheduling

Transitloom should absorb that complexity.

---

## Who Transitloom is for

Transitloom is primarily for:

- operators of real networks with mixed path conditions
- engineers deploying overlay transport across imperfect networks
- people who need strong coordinator-managed trust and admission
- users who want WireGuard to benefit from overlay intelligence without modifying WireGuard itself

Likely deployment environments include:

- servers
- routers/gateways
- appliances
- managed endpoints
- labs
- private infrastructure

Transitloom is not primarily optimized for casual consumer VPN UX in v1.

---

## Why the generic core matters

Transitloom is not meant to become trapped inside one application’s semantics.

The generic core matters because the same system concepts should be reusable across more than one UDP-carried service:

- services
- service bindings
- local ingress
- associations
- path candidates
- relay candidates
- policy-controlled legality
- endpoint-owned scheduling

WireGuard is the first flagship use case, not the final shape of the core model.

Agents must not “simplify” the system by making WireGuard-specific assumptions foundational if those assumptions damage the generic model.

---

## Why the constrained v1 matters

Transitloom v1 is intentionally constrained.

This is not a weakness. It is part of the identity of the project.

The project is not trying to prove every future idea at once.  
It is trying to prove the right first thing:

- correct trust and admission behavior
- useful control-plane coordination
- high-performance raw UDP carriage
- single-relay-hop data behavior
- practical multi-WAN aggregation
- a real WireGuard-over-mesh workflow

Agents should respect that discipline.

---

## Identity-level non-negotiables

At the project identity level, these are the most important non-negotiables:

- Transitloom is coordinator-managed
- Transitloom v1 is raw-UDP-first
- Transitloom v1 data plane is constrained for performance and predictability
- Transitloom separates identity from current participation permission
- Transitloom keeps a generic service model
- Transitloom uses WireGuard-over-mesh as the flagship proof point
- Transitloom should feel like a transport fabric, not just a relay utility

If a design or implementation path weakens these points, it is likely drifting away from the intended project identity.

---

## One-sentence identity

Transitloom is a coordinator-managed overlay mesh transport platform focused first on high-performance raw UDP carriage, practical multi-WAN aggregation, and WireGuard-over-mesh over real-world networks.

---

## Short identity reminder for agents

If you need to compress the project identity into one working sentence while coding, use this:

**Transitloom is building a generic but performance-focused overlay transport core, with WireGuard-over-mesh as the flagship v1 proof point and multi-WAN UDP aggregation as the main practical target.**

---
