# agents/TASKS.md

## Purpose

This file is the compact task index for Transitloom.

It is intentionally short. Detailed task definitions, progress notes, and acceptance criteria should live under:

- `agents/tasks/`

Use this file to answer, quickly:

- what is active now
- what is next
- what is blocked
- what was recently completed

If a task needs more than a short summary, it belongs in its own task file.

---

## Current phase

**implementation bootstrap**

Transitloom currently has:
- architecture/spec baseline
- docs baseline
- object model
- config model
- implementation plan
- initial Go module and code skeleton
- partially built `agents/` workspace
- role-specific config loading and validation scaffolding
- root/coordinator trust-bootstrap inspection scaffolding
- node identity and admission bootstrap inspection scaffolding

Transitloom does **not** yet have meaningful implementation of:
- node enrollment
- node certificate issuance
- admission-token issuance or refresh
- coordinator-side admission-token validation
- service discovery
- association handling
- raw UDP carriage
- WireGuard-over-mesh runtime behavior

---

## Active task

No implementation task is currently marked active.

The next task to start is `T-0008 — direct raw UDP carriage`.

---

## Recently completed

### T-0002 — config loading scaffolding
**status:** completed  
**task file:** `agents/tasks/T-0002-config-loading-scaffolding.md`

Implemented strict YAML config loading and validation scaffolding for root, coordinator, and node roles, plus `-config` startup wiring and tests.

### T-0003 — root/coordinator bootstrap scaffolding
**status:** completed  
**task file:** `agents/tasks/T-0003-root-coordinator-bootstrap.md`

Implemented explicit root/coordinator trust-bootstrap inspection, trust-material presence checks, role-specific startup reporting, and tests for valid and invalid bootstrap states.

### T-0004 — node identity and admission-token scaffolding
**status:** completed
**task file:** `agents/tasks/T-0004-node-identity-and-admission-token-scaffolding.md`

Implemented explicit node-identity and cached-admission-token bootstrap inspection, distinct persisted-state config sections, `transitloom-node` readiness reporting, and tests for valid and invalid local state combinations.

### T-0005 — minimal node-to-coordinator control session
**status:** completed
**task file:** `agents/tasks/T-0005-minimal-node-to-coordinator-control-session.md`

Implemented a bootstrap-only node-to-coordinator control-session exchange over
the coordinator TCP listener, with explicit readiness snapshots, structured
accept/reject results, clear placeholder reporting, and focused listener/client
tests.

### T-0006 — service registration basics
**status:** completed
**task file:** `agents/tasks/T-0006-service-registration-basics.md`

Implemented bootstrap-only service registration over the existing coordinator
TCP listener, with explicit service declaration mapping, a placeholder
coordinator-side in-memory registry, per-service accept/reject results, clear
separation between service binding/local target and requested local ingress
intent, and focused node/coordinator/service tests.

### T-0007 — association basics
**status:** completed
**task file:** `agents/tasks/T-0007-association-basics.md`

Implemented bootstrap-only association creation over the existing coordinator
TCP listener. Nodes can request associations between registered services;
the coordinator validates service existence, rejects self-associations and
duplicates, stores narrow placeholder association records, and returns
structured per-association results. Association is kept strictly distinct from
service registration, path selection, relay eligibility, and forwarding state.

---

## Queued tasks

The next implementation task should be `T-0008 — direct raw UDP carriage`.

---

## Planned sequence

Unless deliberately changed, the intended order remains:

1. T-0001 — agents workspace baseline
2. T-0002 — config loading scaffolding
3. T-0003 — root/coordinator bootstrap scaffolding
4. T-0004 — node identity and admission-token scaffolding
5. T-0005 — minimal node-to-coordinator control session
6. T-0006 — service registration basics
7. T-0007 — association basics
8. T-0008 — direct raw UDP carriage
9. T-0009 — WireGuard-over-mesh direct-path validation
10. T-0010 — single relay hop
11. T-0011 — scheduler baseline and multi-WAN refinement

---

## Current blockers

No hard technical blocker is currently recorded.

The main risk right now is **architecture drift during early implementation**, not lack of ideas.

---

## Immediate priority rules

Right now, prioritize:

1. keeping specs, docs, and agent context consistent
2. building on the completed config, trust-bootstrap, node bootstrap, bootstrap-only control-session, service-registration, and association scaffolding with direct raw UDP carriage work
3. avoiding premature networking/transport complexity
4. preserving the v1 boundaries already chosen
5. continuing `agents/` workspace maintenance as implementation progresses

---

## Updating rule

Whenever task state changes, update:

- this file
- the relevant file under `agents/tasks/`
- `agents/CONTEXT.md` if the current phase, priorities, or blockers changed

If a future agent would need to know it, write it down.

---
