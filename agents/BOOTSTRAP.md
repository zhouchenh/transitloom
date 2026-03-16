# agents/BOOTSTRAP.md

## Purpose

This file is the fast-start entrypoint for coding agents working in the Transitloom repository.

Read this file first after `AGENTS.md`.

Its job is to help an agent get oriented quickly, avoid wasting context window on the wrong things, and start work in the right order.

This file is intentionally practical and current-state oriented.

---

## What Transitloom is

Transitloom is a coordinator-managed overlay mesh transport platform.

Its v1 focus is:

- high-performance raw UDP service carriage
- practical multi-WAN aggregation
- WireGuard-over-mesh as the flagship documented use case

Transitloom is **not** being built as a WireGuard-only product, but WireGuard is the first major proof point.

Transitloom is also **not** trying to become a full unconstrained service mesh in v1.

---

## What matters most right now

The current project priority is:

**turn the architecture and spec set into a disciplined first implementation path without violating v1 boundaries**

That means the main goal is not “write lots of code.”  
The main goal is:

- preserve architecture consistency
- implement the first useful vertical slice
- keep the generic core intact
- make the flagship use case actually achievable

---

## Required reading order

Read these files in this order before doing substantial work:

1. `AGENTS.md`
2. `agents/IDENTITY.md`
3. `agents/SOUL.md`
4. `agents/CONTEXT.md`
5. `agents/MEMORY.md`
6. `agents/TASKS.md`
7. `agents/CODING.md`
8. `agents/REPORTING.md`

Then read any task files referenced from:

- `agents/tasks/`

Then read the relevant specs under:

- `spec/`

Do not skip directly into code unless the task is trivial and clearly isolated.

`agents/README.md` is useful for orientation, especially for humans, but the list above is the operational minimum.

---

## Current repo state

At the time this file is being written, the repository already contains:

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

### Code skeleton
- `go.mod`
- `cmd/transitloom-root`
- `cmd/transitloom-coordinator`
- `cmd/transitloom-node`
- `cmd/tlctl`
- initial `internal/` package skeleton

### Agent workspace baseline
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
- initial task files under `agents/tasks/`

That means the project is **past brainstorming** and is now in the **implementation bootstrap phase**.

---

## Current phase

Transitloom is currently in this phase:

**spec-complete enough to begin careful implementation bootstrap**

That means:

- the architecture is defined enough to start coding
- the first implementation slices should now begin
- but architecture drift is still a major risk
- code should stay tightly aligned to specs

---

## v1 invariants you must keep in mind

These are the most important non-negotiable v1 boundaries.

### Data plane
- raw UDP is the primary v1 data-plane transport
- raw UDP v1 requires zero in-band overhead
- data-plane forwarding is direct or single relay hop only
- no arbitrary multi-hop raw UDP data forwarding in v1
- data-plane scheduling is endpoint-owned
- default scheduler is weighted burst/flowlet-aware
- per-packet striping is allowed only when paths are closely matched

### Control plane
- control plane is more flexible than data plane
- QUIC + TLS 1.3 mTLS is primary
- TCP + TLS 1.3 mTLS is fallback
- control semantics must stay logically consistent across transports

### Trust/admission
- node identity and current participation permission are separate
- valid cert alone is not enough for normal participation
- normal participation requires valid certificate + valid admission token
- revoke is hard in operational effect
- root authority is not a normal node-facing coordinator

### Service model
- generic core
- WireGuard is flagship in docs/examples, not privileged in the core model
- service, service binding, local target, and local ingress must not be collapsed into one concept

---

## What to implement first

Do **not** jump directly into advanced transport logic.

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

This is deliberate. It matches the implementation plan and protects the architecture.

---

## First meaningful implementation milestone

The first meaningful end-to-end milestone is:

**two admitted nodes, one coordinator, one UDP service per node, one legal association, direct raw UDP carriage working**

The first flagship validation milestone after that is:

**WireGuard-over-mesh over a direct path, using Transitloom local ingress ports**

Do not optimize for broader feature coverage before these work.

---

## What not to do early

Avoid these mistakes:

- do not introduce arbitrary multi-hop raw UDP forwarding
- do not invent a generic encrypted data plane as if it already exists
- do not introduce generic TCP carriage as if it already exists
- do not let relays make independent end-to-end scheduling decisions
- do not collapse config, distributed state, and runtime state into one thing
- do not make WireGuard special in the core model
- do not implement elaborate routing machinery before the first direct-path vertical slice works

---

## Safe first commands

Before making changes, it is generally safe to run:

```bash
go build ./...
git status
find . -maxdepth 3 | sort
```

If tests exist later, it should also be safe to run:

```bash
go test ./...
```

Do not assume tests exist yet. Verify first.

---

## Safe first engineering tasks

These are good early tasks for agents:

- define basic object-model-aligned Go types
- implement config loading and validation scaffolding
- implement root/coordinator startup scaffolding
- implement node startup scaffolding
- implement identity/admission persistence boundaries
- add minimal structured logging/status support
- implement bootstrap command flows in `tlctl`

These are **not** good early tasks:

- advanced scheduler optimization
- broad discovery/routing sophistication
- arbitrary relay logic
- helper automation for WireGuard configs
- speculative transport plugins

---

## How to use the spec set

The specs are the architectural truth.

Use them like this:

### For overall boundaries
- `spec/v1-architecture.md`

### For control/session behavior
- `spec/v1-control-plane.md`

### For raw UDP and scheduling behavior
- `spec/v1-data-plane.md`

### For service objects and bindings
- `spec/v1-service-model.md`

### For trust/admission/revoke behavior
- `spec/v1-pki-admission.md`

### For WireGuard flagship mapping
- `spec/v1-wireguard-over-mesh.md`

### For concrete object vocabulary
- `spec/v1-object-model.md`

### For config shape
- `spec/v1-config.md`

### For implementation sequence
- `spec/implementation-plan-v1.md`

If code or a task seems to conflict with specs, do not guess. Stop and resolve the contradiction deliberately.

---

## How to use the agent standards files

Use these files actively while working:

### `agents/CODING.md`
Use this for:
- coding style expectations
- testing rules
- benchmark rules
- commenting standards
- verification requirements
- package-boundary discipline

### `agents/REPORTING.md`
Use this at the end of the run for:
- outcome reporting
- explicit complete/partial/blocked status
- verification reporting
- incompleteness reporting
- next-step reporting

Do not treat either file as optional.

---

## What to record while working

As work progresses, update the `agents/` workspace so future agents do not lose context.

Especially update:

- `agents/CONTEXT.md`
- `agents/context/*.md`
- `agents/MEMORY.md`
- `agents/memory/*.md`
- `agents/TASKS.md`
- `agents/tasks/*.md`
- `agents/logs/`

At a minimum, record:

- meaningful progress
- important decisions
- blockers
- verified assumptions
- disproven assumptions
- implementation constraints discovered during coding
- any change in current priorities

If a future agent would benefit from knowing it, write it down.

---

## How to decide whether to proceed or stop

Proceed when:

- the task is clearly inside the v1 boundary
- the specs already support the change
- the package boundaries remain clean
- the change improves the current implementation slice

Stop and clarify or record a blocker when:

- the task appears to widen v1 scope
- the specs conflict with each other
- the task would collapse important object boundaries
- the task requires introducing behavior explicitly deferred in v1
- the implementation path looks elegant but misaligned with the flagship use case

---

## Current expected next implementation direction

Unless a task explicitly says otherwise, the expected next engineering direction is:

1. object-model-aligned config loading
2. root/coordinator bootstrap path
3. node identity material and admission-token handling
4. minimal node-to-coordinator control session
5. service registration
6. association legality
7. direct raw UDP data path
8. WireGuard-over-mesh direct-path validation

That is the current default direction of work.

---

## Success mindset

A good Transitloom agent session should leave the repo in a state that is:

- more correct
- more consistent
- more understandable
- more useful for the next implementation step
- better documented in `agents/`

Do not optimize for volume of change.

Optimize for **making the next correct step easier and safer**.

---
