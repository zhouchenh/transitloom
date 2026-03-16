# agents/tasks/T-0001-agents-workspace-baseline.md

## Task ID

T-0001

## Title

Agents workspace baseline

## Status

Active

## Purpose

Build the minimum useful `agents/` workspace so coding agents can work in this repository with persistent context, stable terminology, and clear continuity despite context window limits.

This task is foundational. It reduces the risk of repeated confusion, architecture drift, and wasted rediscovery.

---

## Why this task matters

Transitloom already has:
- substantial architecture and spec work
- multiple role concepts
- a staged implementation plan
- nontrivial design boundaries

That means coding agents now need a repo-local continuity layer.

Without a proper `agents/` workspace:
- important decisions will be forgotten
- future agents will have to reload too much context
- terminology will drift
- task sequencing will become unclear
- implementation may diverge from the intended v1 shape

This task exists to prevent that.

---

## Objective

Establish the core `agents/` workspace files and structure needed for agents to operate effectively.

---

## Scope

This task includes:

- drafting `AGENTS.md`
- drafting the core `agents/` files:
  - `agents/BOOTSTRAP.md`
  - `agents/IDENTITY.md`
  - `agents/SOUL.md`
  - `agents/CONTEXT.md`
  - `agents/MEMORY.md`
  - `agents/TASKS.md`
- establishing the intended structure for:
  - `agents/tasks/`
  - `agents/context/`
  - `agents/memory/`
  - `agents/logs/`
- making the task system use `agents/TASKS.md` as an index and `agents/tasks/*.md` as detailed task records

---

## Non-goals

This task does **not** include:

- implementing networking features
- implementing PKI logic
- implementing control-plane transport
- implementing data-plane transport
- building a large amount of placeholder/noise content in `agents/context/`, `agents/memory/`, or `agents/logs/`
- creating many empty task files without real use

---

## Expected outputs

At minimum, this task should leave the repo with:

- a usable `AGENTS.md`
- a usable core `agents/` workspace baseline
- clear reading order for coding agents
- durable project identity and philosophy captured
- current project state captured
- durable project memory captured
- a short task index in `agents/TASKS.md`
- this detailed task file present under `agents/tasks/`

---

## Acceptance criteria

This task is complete when all of the following are true:

1. `AGENTS.md` clearly explains:
   - required reading order
   - v1 invariants
   - workflow expectations
   - the critical requirement to maintain `agents/` context/memory

2. The following files exist and are coherent:
   - `agents/BOOTSTRAP.md`
   - `agents/IDENTITY.md`
   - `agents/SOUL.md`
   - `agents/CONTEXT.md`
   - `agents/MEMORY.md`
   - `agents/TASKS.md`

3. `agents/TASKS.md` is a concise task index, not a giant ledger

4. Detailed tasks live under `agents/tasks/*.md`

5. The content across the core `agents/` files is consistent with:
   - `spec/`
   - `docs/`
   - the current repo state

6. The workspace helps a future coding agent answer:
   - what Transitloom is
   - what matters most
   - what is already decided
   - what is happening now
   - what should happen next

---

## Files likely touched

- `AGENTS.md`
- `agents/BOOTSTRAP.md`
- `agents/IDENTITY.md`
- `agents/SOUL.md`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`
- `agents/tasks/T-0001-agents-workspace-baseline.md`

Potentially:
- `agents/README.md`
- `agents/tasks/T-0002-config-loading-scaffolding.md`

---

## Verification

Verify this task by checking:

- the `agents/` workspace files exist
- the reading order is clear and consistent
- task indexing is short and points to detailed task files
- the core agent files do not contradict the specs
- the workspace gives enough context to start the next implementation task without reloading the whole repo manually

This is primarily a content/consistency verification task rather than a build/test task.

---

## Current progress notes

Known progress so far:
- `AGENTS.md` drafted
- core `agents/` files are being drafted
- task indexing model has been refined so that detailed tasks live in `agents/tasks/*.md`

Remaining likely work:
- finalize `agents/TASKS.md`
- finalize this task file
- draft the next task file(s)
- optionally add `agents/README.md` if it still feels useful after the core files settle

---

## Handoff notes

When this task is complete, the next task to activate should be:

- `T-0002 — config loading scaffolding`

That is the first implementation-oriented task that should follow the agent workspace baseline.

---
