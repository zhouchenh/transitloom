# agents/README.md

## Purpose

This directory exists to help coding agents and humans work on Transitloom with continuity, clarity, and architectural discipline.

Transitloom is already large enough in scope that agents should not rely on short-term conversational memory alone. The `agents/` workspace is the repository-local continuity layer for:

- project identity
- design philosophy
- current implementation context
- durable project memory
- active task tracking
- coding standards
- reporting standards
- deeper task/context/memory files
- handoff and investigation notes

Think of this directory as the **persistent working memory and operating layer** for the project.

---

## How to use this directory

If you are a coding agent, start with:

1. `../AGENTS.md`
2. `BOOTSTRAP.md`
3. `IDENTITY.md`
4. `SOUL.md`
5. `CONTEXT.md`
6. `MEMORY.md`
7. `TASKS.md`
8. `CODING.md`
9. `REPORTING.md`

Then read any task files under:

- `tasks/`

And, when relevant, the authoritative specs under:

- `../spec/`

`../AGENTS.md` is the top-level operating contract.  
This directory is the repo-local context, memory, task, coding, and reporting workspace.

---

## File roles

### `BOOTSTRAP.md`
Fast-start guidance for agents.

Use this to understand:
- what Transitloom is
- what matters right now
- what should be implemented first
- what to avoid doing too early

### `IDENTITY.md`
Project identity.

Use this to understand:
- what Transitloom is
- what Transitloom is not
- who it is for
- what the flagship v1 use case is

### `SOUL.md`
Design philosophy and decision compass.

Use this when deciding:
- how to trade off performance vs flexibility
- how to preserve the generic core
- how to avoid “good ideas” that are wrong for v1

### `CONTEXT.md`
Current working state.

Use this to understand:
- current repo status
- current project phase
- what has already been done
- what the immediate next priorities are

Update this file when meaningful progress or current-state changes happen.

### `MEMORY.md`
Durable project memory.

Use this to preserve:
- settled design decisions
- invariants
- important naming choices
- rejected approaches
- lessons that should not be rediscovered repeatedly

Update this file when a decision becomes durable enough to matter across tasks.

### `TASKS.md`
Compact task index.

Use this to see:
- what is active
- what is queued
- what is blocked
- what was recently completed

Detailed task definitions should live under `tasks/`, not grow unbounded in `TASKS.md`.

### `CODING.md`
Coding standards and implementation discipline.

Use this to understand:
- coding expectations
- testing expectations
- benchmark expectations
- commenting standards
- verification rules
- package-boundary discipline

This file helps prevent task prompts from having to restate the same coding rules repeatedly.

### `REPORTING.md`
End-of-run reporting standard.

Use this to understand:
- how to report what changed
- how to distinguish complete vs partial vs blocked work
- how to describe verification honestly
- what must be recorded at the end of a run
- how to make handoff and continuation easier for future agents

This file makes reporting reviewable and trustworthy.

---

## Subdirectories

### `tasks/`
Detailed task files.

Use one file per meaningful task when a short line in `TASKS.md` is not enough.

A task file should usually include:
- objective
- why it matters
- scope
- non-goals
- acceptance criteria
- verification steps
- status
- handoff notes

### `context/`
Deeper supporting context documents.

Use this when `CONTEXT.md` would become too crowded, or when a particular implementation area needs a focused context file.

Examples:
- scheduler rationale
- relay model notes
- service model implementation notes
- config design notes

### `memory/`
Durable memory shards.

Use this for long-lived knowledge that is important enough to preserve, but too specific or detailed to keep piling into `MEMORY.md`.

Examples:
- naming conventions
- architectural invariants
- persistent “do not do this” lessons
- implementation sequencing rules

### `logs/`
Work logs, handoff notes, and investigation records.

Use this for:
- session summaries
- blocked-path investigations
- failed-attempt notes
- debugging writeups
- progress checkpoints that should not vanish

---

## Important working rule

The `agents/` directory is **not optional documentation**.

For context-limited coding agents, this directory is part of the working system. If knowledge is not written down here when it matters, it may be forgotten, rediscovered poorly, or violated later.

When meaningful progress is made, ask:

1. What changed?
2. What was learned?
3. What should persist?
4. Which file under `agents/` should be updated now?

Then update it.

---

## Relationship to other parts of the repo

### `../spec/`
Authoritative engineering specifications.

If implementation decisions or architecture boundaries matter, the specs are the main source of truth.

### `../docs/`
Human-facing documentation.

Useful for understanding the project and explaining it, but not the primary place to derive implementation truth.

### `../AGENTS.md`
Repo-wide agent operating rules.

This file defines:
- mandatory reading order
- v1 invariants
- workflow expectations
- memory/update requirements

Read it first.

---

## Current intention

At this stage of the project, the main role of the `agents/` workspace is to help guide Transitloom from:

- well-specified architecture

to:

- disciplined first implementation

without letting architecture drift, task continuity break, coding standards become inconsistent, or important decisions disappear between sessions.

---

## One-sentence summary

The `agents/` directory is the persistent operational memory, task-navigation layer, coding standard layer, and reporting layer for coding agents working on Transitloom.

---
