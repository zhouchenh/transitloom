# agents/CONTEXT.md

## Purpose

This file captures the **current working context** of the Transitloom repository.

Unlike `agents/IDENTITY.md` and `agents/SOUL.md`, which are meant to stay relatively stable, this file should be updated whenever the project’s active phase, immediate priorities, implementation status, or known blockers change.

This file exists because coding agents are context-limited and should not rely on remembering recent repository state across sessions.

---

## Current project phase

Transitloom is currently in the:

**implementation bootstrap phase**

That means:

- the project has moved beyond broad architecture brainstorming
- the initial v1 spec set has been drafted
- the initial docs set has been drafted
- the initial Go module and repository skeleton have been created
- the `agents/` workspace baseline is being established
- the next step is to begin the first disciplined implementation slice

The project is **not** yet in feature-development mode for advanced networking behavior.

It is currently in the stage where:
- architecture must remain consistent
- object boundaries must remain clean
- implementation sequencing matters a lot
- foundational mistakes are more dangerous than slow progress

---

## Current repository status

At the time this file is written, the repository already contains:

### Top-level documents
- `README.md`
- `LICENSE`
- `AGENTS.md`

### Specs
- `spec/v1-architecture.md`
- `spec/v1-control-plane.md`
- `spec/v1-data-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-pki-admission.md`
- `spec/v1-wireguard-over-mesh.md`
- `spec/v1-object-model.md`
- `spec/v1-config.md`
- `spec/implementation-plan-v1.md`

### Docs
- `docs/vision.md`
- `docs/concepts.md`
- `docs/roadmap.md`
- `docs/glossary.md`

### Agent workspace files drafted so far
- `AGENTS.md`
- `agents/README.md`
- `agents/BOOTSTRAP.md`
- `agents/IDENTITY.md`
- `agents/SOUL.md`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`
- `agents/CODING.md`
- `agents/REPORTING.md`

### Agent task files drafted so far
- `agents/tasks/T-0001-agents-workspace-baseline.md`
- `agents/tasks/T-0002-config-loading-scaffolding.md`
- `agents/tasks/T-0003-root-coordinator-bootstrap.md`

### Agent workspace directories
- `agents/tasks/`
- `agents/context/`
- `agents/memory/`
- `agents/logs/`

### Code skeleton
- `go.mod`
- `cmd/transitloom-root/main.go`
- `cmd/transitloom-coordinator/main.go`
- `cmd/transitloom-node/main.go`
- `cmd/tlctl/main.go`

### Internal package skeleton
- `internal/admission/`
- `internal/config/`
- `internal/controlplane/`
- `internal/coordinator/`
- `internal/dataplane/`
- `internal/identity/`
- `internal/node/`
- `internal/objectmodel/`
- `internal/pki/`
- `internal/scheduler/`
- `internal/service/`
- `internal/status/`
- `internal/transport/`

The code is still placeholder-level. Real implementation has not meaningfully begun yet.

---

## Current implementation state

### What is already done
- project naming is settled: **Transitloom**
- license choice is settled: **GPL-3.0**
- v1 architecture direction is documented
- v1 control-plane direction is documented
- v1 data-plane direction is documented
- service model is documented
- PKI/admission model is documented
- WireGuard-over-mesh model is documented
- object model is documented
- config model is documented
- implementation plan is drafted
- repository and command/package skeleton exist
- agent workspace baseline has been largely drafted
- coding standards and reporting standards have dedicated agent files

### What is not done yet
- no real config loading
- no real object model implementation in Go
- no root/coordinator bootstrap logic
- no PKI issuance logic
- no admission token logic
- no node enrollment flow
- no control-plane transport implementation
- no service registration implementation
- no association implementation
- no raw UDP data path
- no WireGuard-over-mesh working slice
- no relay behavior
- no scheduler implementation

In other words: **the shape exists, but the system does not yet exist.**

---

## Current v1 architectural boundaries

These boundaries are already chosen and should be treated as active constraints.

### Data plane
- raw UDP is the primary v1 data-plane transport
- zero in-band overhead is required for raw UDP
- v1 raw UDP data plane supports:
  - direct public paths
  - direct intranet/private paths
  - single coordinator relay hop
  - single node relay hop
- v1 raw UDP data plane does **not** support arbitrary multi-hop forwarding
- data-plane scheduling is endpoint-owned
- default scheduler is weighted burst/flowlet-aware
- per-packet striping is allowed only when paths are closely matched

### Control plane
- control plane is more flexible than data plane
- QUIC + TLS 1.3 mTLS is primary
- TCP + TLS 1.3 mTLS is fallback
- control semantics should stay logically consistent across both transports

### Trust and admission
- node identity and participation permission are separate
- a valid certificate alone is not enough for normal participation
- normal participation requires:
  - valid node certificate
  - valid admission token
- revoke is hard in operational effect
- root authority is not a normal node-facing coordinator target

### Service model
- core model remains generic
- WireGuard is the flagship v1 use case in docs and examples
- service, service binding, local target, and local ingress are distinct concepts
- multiple services per node are supported
- multiple WireGuard services per node are supported

### Product scope
- Transitloom v1 is not trying to be a full unconstrained service mesh
- Transitloom v1 is trying to make the flagship raw-UDP transport path work well first
- multi-WAN aggregation is still a primary practical target

---

## Current implementation priorities

The current implementation priorities, in order, are:

1. preserve architectural consistency
2. preserve object model boundaries
3. finish the usable `agents/` workspace baseline
4. start implementation in the order defined by `spec/implementation-plan-v1.md`
5. avoid premature feature expansion
6. prove the first real vertical slice as early as possible

The intended implementation order is:

1. config and object-model-aligned scaffolding
2. root/coordinator bootstrap
3. node identity and admission-token flow
4. minimal node-to-coordinator control session
5. service registration
6. association creation/distribution
7. direct raw UDP carriage
8. WireGuard-over-mesh direct path
9. single relay hop
10. scheduler and multi-WAN refinement

---

## Immediate next tasks

The immediate next tasks are:

### Agent workspace completion and normalization
The `agents/` workspace now has a solid baseline, but it should continue to be normalized and kept consistent as work begins.

Near-term agent-workspace work includes:
- keeping `AGENTS.md` and the `agents/` files consistent
- ensuring `agents/TASKS.md` stays a compact index
- using `agents/tasks/*.md` for detailed task tracking
- updating `agents/CONTEXT.md`, `agents/MEMORY.md`, and task files as progress is made

### Implementation bootstrap
The first real implementation work should begin with:

- object-model-aligned Go types where needed
- config loading and validation scaffolding
- root/coordinator/node startup scaffolding
- PKI/admission flow scaffolding after config loading is in place

### Current active implementation-oriented task
The next practical implementation task is:

- `T-0002 — config loading scaffolding`

That task should be treated as the first real code slice unless the task system is deliberately reprioritized.

---

## First target milestone

The first meaningful milestone remains:

**two admitted nodes, one coordinator, one UDP service per node, one legal association, direct raw UDP carriage working**

The first flagship validation milestone after that remains:

**WireGuard-over-mesh over a direct path, using Transitloom local ingress ports**

These milestones should guide what gets built first.

---

## Current risks

The biggest current risks are architectural and sequencing risks, not low-level code bugs.

### Risk 1: architecture drift
Now that code skeleton and agent workspace exist, it is easy for implementation to drift away from the specs if agents start coding from intuition instead of reading.

### Risk 2: collapsing important concepts
The following distinctions are easy to accidentally collapse:
- identity vs admission
- service vs service binding
- local target vs local ingress
- relay candidate vs path candidate
- config vs distributed state

These distinctions must be preserved.

### Risk 3: premature abstraction
It would be easy to build:
- a broad routing framework
- a broad transport abstraction layer
- a broad policy engine
- a broad service-mesh API shape

before the first direct raw UDP vertical slice works.

That would likely slow the project and weaken the architecture.

### Risk 4: implementation in the wrong order
If coding starts with:
- advanced scheduler logic
- advanced discovery/routing
- broad relay behavior
- WireGuard helpers
- speculative encrypted transport

before the trust/control/service/direct-path foundation exists, progress will look larger than it really is.

### Risk 5: poor continuity discipline
Because agents are context-limited, failing to update the `agents/` workspace when meaningful progress or learning occurs is a real project risk, not merely a documentation lapse.

---

## Current practical guidance

At this stage, agents should optimize for:

- simple, clean package boundaries
- object-model fidelity
- correct trust/admission separation
- config clarity
- minimal viable vertical slices
- good status/observability scaffolding
- recording progress in `agents/`
- honest reporting using `agents/REPORTING.md`
- coding discipline using `agents/CODING.md`

Agents should **not** optimize for:
- broad feature counts
- speculative future transport types
- elaborate routing machinery
- local code elegance that breaks the current architecture

---

## Current task-system state

The task system is intended to work like this:

- `agents/TASKS.md` = compact task index
- `agents/tasks/*.md` = detailed task files
- `agents/REPORTING.md` = end-of-run reporting standard
- `agents/CODING.md` = coding standards

This should remain the working model unless deliberately changed.

---

## What should be updated next

This file should be updated when:

- the agent workspace baseline is fully stabilized
- the first implementation package content becomes real
- the project moves from bootstrap into actual config/trust/control implementation
- the active task changes materially
- the first milestone changes
- a serious blocker appears
- the immediate next tasks change

---

## Current summary

Transitloom is currently a **well-specified but not-yet-implemented** project with:

- strong v1 specs
- a clear flagship use case
- a clear implementation order
- a repo/code skeleton
- a mostly established `agents/` workspace
- explicit coding and reporting standards
- no substantive networking or trust code yet

The correct next move is to keep the `agents/` workspace accurate and begin the first real implementation slice in the order already defined, starting with config loading scaffolding.

---
