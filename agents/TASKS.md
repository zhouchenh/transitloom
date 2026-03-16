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

Transitloom does **not** yet have meaningful implementation of:
- trust bootstrap
- admission-token flow
- control sessions
- service registration
- association handling
- raw UDP carriage
- WireGuard-over-mesh runtime behavior

---

## Active task

### T-0001 — agents workspace baseline
**status:** active  
**task file:** `agents/tasks/T-0001-agents-workspace-baseline.md`

Finish the minimum `agents/` workspace baseline so coding agents can operate with stable context, persistent memory, and clear task continuity.

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

---

## Queued tasks

The next implementation task should be `T-0004 — node identity and admission-token scaffolding` once its task file is drafted.

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

1. finishing the `agents/` workspace baseline
2. keeping specs, docs, and agent context consistent
3. building on the completed config and trust-bootstrap scaffolding with node identity/admission work
4. avoiding premature networking/transport complexity
5. preserving the v1 boundaries already chosen

---

## Updating rule

Whenever task state changes, update:

- this file
- the relevant file under `agents/tasks/`
- `agents/CONTEXT.md` if the current phase, priorities, or blockers changed

If a future agent would need to know it, write it down.

---
