# Transitloom v1 Implementation Plan

## Status

Draft

This document defines the implementation plan for Transitloom v1. It translates the architecture and object model into an execution sequence, identifies the first meaningful milestones, and constrains early implementation so that development stays aligned with the flagship use case:

- **WireGuard over mesh**
- **high-performance raw UDP carriage**
- **multi-WAN aggregation**
- **coordinator-managed trust and admission**

This document is not a product roadmap. It is an engineering sequencing document.

---

## 1. Purpose

Transitloom already has a broad architectural direction. The implementation plan exists to answer the practical questions that architecture alone does not answer:

- What should be implemented first?
- What should be deferred?
- What is the first end-to-end success criterion?
- What should be real, and what can be stubbed initially?
- In what order should binaries, packages, and protocols appear?

The goal is to avoid building the wrong layer first.

---

## 2. Implementation principles

Transitloom v1 implementation should follow these principles:

- build **end-to-end vertical slices**, not isolated subsystems forever
- prefer **one real working path** over many incomplete abstractions
- keep the **core model generic**, while optimizing examples around WireGuard
- enforce **trust/admission correctness early**
- keep **data-plane scope constrained** to the v1 boundary
- measure success by **working overlay transport**, not by protocol surface area
- optimize for the flagship use case before broadening the feature set

---

## 3. v1 success definition

Transitloom v1 is considered meaningfully implemented when all of the following are true:

1. A node can obtain identity and admission through the Transitloom control system
2. A node can register a UDP service
3. Two nodes can form a legal association for that service
4. Transitloom can carry raw UDP traffic between them over a direct path
5. Transitloom can carry raw UDP traffic through a single relay hop when direct is unavailable or less suitable
6. WireGuard can use Transitloom local ingress endpoints without WireGuard protocol changes
7. The data plane uses endpoint-owned scheduling and supports the v1 default scheduler
8. Revoke prevents normal participation even when a still-valid node certificate exists

The first major public demonstration should be a working WireGuard-over-mesh example.

---

## 4. Explicit v1 boundaries

The implementation must respect the following scope boundaries.

### Included in v1

- root authority role
- coordinator role
- node role
- private PKI under Transitloom control
- per-coordinator intermediates
- short-lived admission tokens
- coordinator-managed service registration
- service associations
- direct raw UDP carriage
- single relay hop for raw UDP data
- stable local ingress bindings
- WireGuard-over-mesh flagship support
- weighted burst/flowlet-aware scheduling
- conditional per-packet striping only for closely matched paths

### Deferred beyond the first working v1

- arbitrary multi-hop raw UDP data forwarding
- generic encrypted data plane
- generic TCP data plane
- deep helper automation for WireGuard configs
- broad application service-mesh features
- highly polished admin UX before correctness
- advanced routing sophistication beyond v1 needs

---

## 5. Initial deliverables

The implementation should produce these binaries or equivalent roles:

- `transitloom-root`
- `transitloom-coordinator`
- `transitloom-node`
- `tlctl`

These names may evolve, but the role separation should remain.

### 5.1 `transitloom-root`

Purpose:
- trust anchor operations
- coordinator intermediate issuance/lifecycle support
- not a normal node-facing coordinator

### 5.2 `transitloom-coordinator`

Purpose:
- node-facing control-plane endpoint
- mTLS/auth/admission enforcement
- service registry and association control
- optional relay behavior
- certificate issuance via coordinator intermediate

### 5.3 `transitloom-node`

Purpose:
- establish control sessions
- host local services
- expose local ingress bindings
- carry service traffic over the mesh
- perform path measurement and endpoint-owned scheduling

### 5.4 `tlctl`

Purpose:
- admin/operator CLI
- bootstrap operations
- admission/revoke operations
- visibility and status
- future config and management tooling

---

## 6. Recommended repository and package baseline

Before substantial implementation, establish a stable code layout.

Recommended initial structure:

```text
cmd/
  transitloom-root/
  transitloom-coordinator/
  transitloom-node/
  tlctl/

internal/
  admission/
  config/
  controlplane/
  coordinator/
  dataplane/
  identity/
  node/
  objectmodel/
  pki/
  scheduler/
  service/
  status/
  transport/

pkg/
```

### Package guidance

- keep **spec concepts visible in package boundaries**
- do not collapse identity, admission, service, and transport too early
- use `internal/objectmodel` or equivalent for central shared logical types
- keep transport concerns separated from service and policy concerns

---

## 7. Recommended implementation order

The implementation should proceed in stages.

---

## 8. Stage 0: repository and development baseline

### Objectives

- initialize Go module
- create binary entrypoints
- create package skeleton
- establish configuration loading pattern
- establish logging/status conventions
- establish object identifiers and shared basic types

### Deliverables

- `go.mod`
- empty or minimal command entrypoints
- config scaffolding
- logging scaffolding
- shared object IDs and basic model package

### What can be stubbed

- transport details
- cert issuance internals
- relay logic
- scheduler logic

### Exit criteria

- project builds
- commands run with basic scaffolding
- config can be loaded and validated in a minimal way

---

## 9. Stage 1: trust foundation and root/coordinator bootstrap

### Objectives

- implement root authority basics
- implement coordinator intermediate issuance
- define trust bootstrap flow
- establish mTLS trust chain basics
- ensure no external OpenSSL dependency is required

### Deliverables

- root authority key/cert handling
- coordinator intermediate issuance flow
- coordinator trust initialization
- minimal persistence for authority state
- basic CLI commands for bootstrap

### What can be stubbed

- node certificate issuance beyond minimal happy path
- admission tokens
- service registry
- data plane

### Exit criteria

- a root authority can initialize trust
- a coordinator can obtain or load an intermediate
- coordinator trust state can be verified locally

---

## 10. Stage 2: node identity and admission foundation

### Objectives

- implement node identity bootstrap
- implement node certificate issuance
- implement admission state model
- implement short-lived admission token issuance and validation
- enforce hard revoke behavior at normal session establishment

### Deliverables

- node enrollment flow
- node certificate issuance
- admission state storage and evaluation
- admission token issuance and refresh
- revoke logic
- coordinator-side rejection of invalid/revoked nodes

### What can be stubbed

- full service model
- relay usage
- advanced discovery
- data plane

### Exit criteria

- a node can enroll and become admitted
- a node can establish a normal control session only with valid cert + valid admission token
- revoke stops normal participation even if the node certificate is still valid

---

## 11. Stage 3: minimal control-plane session and service registration

### Objectives

- implement real node-to-coordinator control sessions
- implement coordinator-side service registration
- implement coordinator-side service lookup/discovery basics
- implement basic association creation/distribution

### Deliverables

- QUIC + TLS 1.3 mTLS control transport
- TCP + TLS 1.3 mTLS fallback transport
- control-session establishment
- service registration messages/handlers
- association definition and distribution
- minimal status reporting

### What can be stubbed

- complex discovery catalogs
- relay-assisted control sessions
- advanced coordinator federation behaviors
- data plane

### Exit criteria

- an admitted node can establish a control session
- the node can register a UDP service
- another admitted node can obtain enough coordinator-provided context to form a valid association

---

## 12. Stage 4: direct raw UDP carriage (first end-to-end data plane)

### Objectives

- implement local service binding
- implement local ingress binding
- implement direct raw UDP forwarding for a legal association
- preserve zero in-band overhead
- keep forwarding stateful and association-bound

### Deliverables

- UDP service binding logic
- local ingress listener logic
- association-bound forwarding context
- direct path send/receive path
- generic status counters for service/association/path visibility

### What can be stubbed

- relay data paths
- sophisticated scheduler
- advanced path measurement
- intranet candidate optimization

### Exit criteria

- two admitted nodes can register services
- an association can be created
- raw UDP traffic can be carried directly between them
- local ingress and local target roles are clearly separated

---

## 13. Stage 5: WireGuard-over-mesh first working slice

### Objectives

- prove the flagship use case as early as possible after direct UDP carriage exists
- map one WireGuard service per node into the generic service model
- expose stable local ingress ports for remote WireGuard peers
- deliver carried packets to the local WireGuard listen port

### Deliverables

- deterministic local ingress assignment for WireGuard-over-mesh use
- documented example configuration for 2-node deployment
- successful end-to-end WireGuard connectivity over Transitloom direct path
- initial operational visibility for that deployment

### What can be stubbed

- relay for WireGuard traffic
- multi-path aggregation
- richer discovery

### Exit criteria

- standard WireGuard can operate over Transitloom without protocol changes
- WireGuard peer `Endpoint` can point to Transitloom local loopback ingress
- packets reach the remote WireGuard service correctly

---

## 14. Stage 6: relay-assisted data path (single relay hop only)

### Objectives

- implement single coordinator relay hop
- implement single node relay hop
- preserve zero-overhead raw UDP behavior
- keep relay use legal only under control-plane-installed context

### Deliverables

- relay candidate modeling in code
- relay path selection basics
- relay forwarding state installation
- coordinator relay support
- node relay support
- relay-aware status and observability

### What can be stubbed

- advanced relay scoring
- aggressive relay optimization
- complex control relay topologies

### Exit criteria

- raw UDP traffic can be carried through one relay hop
- direct and relayed paths remain clearly distinguished
- relay is association-bound and policy-controlled
- no arbitrary multi-hop data forwarding is introduced

---

## 15. Stage 7: path health and scheduler baseline

### Objectives

- implement path measurement inputs
- implement v1 scheduler baseline
- implement direct vs relay preference logic
- support multi-path use for the flagship use case

### Deliverables

- health summary inputs
- path state transitions
- weighted burst/flowlet-aware scheduler
- conditional per-packet striping only for closely matched paths
- direct vs relay scoring and hysteresis
- metered/unmetered-aware probing rules

### What can be stubbed

- very advanced scheduler families
- sophisticated route diversity algorithms
- future transport types

### Exit criteria

- endpoints own the split decisions
- path selection uses combined quality scoring
- per-packet striping is limited to closely matched paths
- multi-WAN aggregation shows meaningful benefit in controlled tests

---

## 16. Stage 8: coordinator-assisted discovery and operational hardening

### Objectives

- improve coordinator discovery
- improve service/association visibility
- improve route/relay assistance
- harden admin operations and observability

### Deliverables

- coordinator catalog handling
- node-provided discovery hints as non-authoritative hints
- better `tlctl` status and admin commands
- health summary reporting
- operator-facing visibility for services, associations, paths, and relays

### Exit criteria

- the system is operable without intimate internal knowledge
- discovery and control visibility are sufficient for real testing
- revoke/admission/policy changes are observable and understandable

---

## 17. First end-to-end milestone

The first meaningful end-to-end milestone should be intentionally narrow.

### Milestone definition

**Two admitted nodes, one coordinator, one UDP service per node, direct path, WireGuard-over-mesh working through Transitloom local ingress ports.**

This milestone is more valuable than building several partial subsystems in parallel.

### Why this is the right first milestone

It validates:

- trust chain
- admission token enforcement
- control session establishment
- service registration
- association legality
- direct raw UDP forwarding
- local ingress vs local target model
- flagship WireGuard use case

---

## 18. Second milestone

After the first end-to-end direct milestone works:

### Milestone definition

**Two admitted nodes, one coordinator, one relay-assisted path, WireGuard-over-mesh working through a single relay hop.**

This validates the first constrained relay behavior without introducing arbitrary routing complexity.

---

## 19. Third milestone

After relay-assisted carriage works:

### Milestone definition

**One node with multiple WAN-capable candidate paths, one remote peer, endpoint-owned scheduler active, measurable aggregation benefit under acceptable reordering.**

This validates the core reason Transitloom exists.

---

## 20. Recommended implementation priority by subsystem

### Highest priority first

1. PKI bootstrap and admission token enforcement
2. node-to-coordinator control sessions
3. service registration and association legality
4. direct raw UDP carriage
5. WireGuard-over-mesh direct path
6. single relay hop
7. path health + scheduler
8. coordinator-assisted discovery hardening

### Lower priority later

- broad admin UX
- generalized routing features
- future transport types
- extensive helper automation
- broad service-type ecosystem

---

## 21. Suggested development strategy

### 21.1 Build vertical slices

Prefer building “one real thing that works” over abstracting everything before proving the flagship flow.

### 21.2 Keep protocol surfaces narrow at first

The initial control-plane message surface should be only large enough to support:

- identity/admission
- service registration
- association creation
- minimal path eligibility
- status

### 21.3 Avoid early abstraction explosion

Do not build large generic frameworks for:
- arbitrary routing
- arbitrary transport plugins
- advanced policy engines
- deep config templating

until the first end-to-end path is proven.

### 21.4 Keep observability from the beginning

Even early milestones should expose enough status to debug:

- who is admitted
- which service exists
- which association exists
- which local ingress maps to which service
- which path is active
- whether direct or relay is in use

---

## 22. Testing strategy

### 22.1 Stage-aligned testing

Testing should align to the staged milestones:

- trust bootstrap tests
- admission/revoke tests
- service registration tests
- direct UDP carriage tests
- WireGuard-over-mesh tests
- single-relay-hop tests
- multi-WAN scheduler tests

### 22.2 Real-network focus

Because Transitloom is intended for messy networks, testing should include conditions such as:

- dynamic IP changes
- direct vs relay fallback
- mixed path quality
- metered/unmetered behavior
- partial reachability
- NAT-sensitive behavior where possible

### 22.3 Deterministic local tests first

Start with deterministic local or lab tests before adding messy real-world network conditions.

---

## 23. Documentation deliverables that should track implementation

As implementation begins, documentation should grow in this order:

1. architecture/spec consistency updates
2. object model updates
3. config schema docs
4. first direct-path WireGuard example
5. relay example
6. multi-WAN tuning guidance
7. operator/admin usage docs

Do not write large polished setup guides before the first real end-to-end slice works.

---

## 24. Immediate next implementation artifacts

The next useful artifacts after this implementation plan are:

- `spec/v1-config.md`
- `docs/glossary.md`
- code skeleton under `cmd/` and `internal/`
- object-model-aligned basic type definitions
- initial control-plane transport scaffolding
- trust bootstrap commands in `tlctl`

---

## 25. v1 implementation checklist summary

### Foundation
- [ ] Go module initialized
- [ ] command entrypoints created
- [ ] config skeleton created
- [ ] logging/status baseline created

### Trust and admission
- [ ] root authority implemented
- [ ] coordinator intermediate issuance implemented
- [ ] node certificate issuance implemented
- [ ] admission token issuance/validation implemented
- [ ] revoke enforced

### Control plane
- [ ] QUIC+mTLS control transport implemented
- [ ] TCP+mTLS fallback implemented
- [ ] service registration implemented
- [ ] association distribution implemented

### Data plane
- [ ] direct raw UDP carriage implemented
- [ ] local ingress vs local target model implemented
- [ ] single relay hop implemented
- [ ] endpoint-owned scheduling implemented
- [ ] default scheduler implemented

### Flagship use case
- [ ] WireGuard-over-mesh direct path works
- [ ] WireGuard-over-mesh single relay hop works
- [ ] multi-WAN aggregation baseline demonstrated

---

## 26. Relationship to other specifications

This implementation plan depends on and should remain consistent with:

- `spec/v1-architecture.md`
- `spec/v1-control-plane.md`
- `spec/v1-data-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-pki-admission.md`
- `spec/v1-wireguard-over-mesh.md`
- `spec/v1-object-model.md`

---
