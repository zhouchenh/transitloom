# agents/REPORTING.md

## Purpose

This file defines how coding agents should report back at the end of a work run.

Its goal is to make each run:

- reviewable
- trustworthy
- easy to continue from
- clear about what is finished vs unfinished
- clear about what was actually verified

A good end-of-run report is part of the work. It is not optional polish.

For Transitloom, this matters because:
- architecture boundaries are important
- coding agents are context-limited
- future agents may need to continue from partial progress
- unverified claims are dangerous
- hidden blockers waste time

---

## Core reporting rule

At the end of a run, the agent must report **what actually happened**, not what it hoped to achieve.

The report must clearly separate:

- completed
- partially completed
- not completed
- verified
- not verified
- blocked

Do not blur these categories.

Do not present partial work as full completion.

Do not imply verification that did not happen.

---

## Commit and push expectation

Before the first stable `v1.0.0` release, a completed and verified task run should normally end with:

- a coherent commit
- a push to `master`

unless:
- the task explicitly says not to do that
- the run is partial or blocked
- the repo would be left in a confusing intermediate state
- a real blocker prevents commit or push

At and after `v1.0.0`, this changes to the branch-based workflow defined in `AGENTS.md`.

## Required end-of-run report structure

Use this structure unless a task explicitly requires a different format.

## 1. Objective

State the task or goal of the run in one short paragraph.

Answer:
- what was the intended objective?
- which task ID or task file was being worked on?

Example:
- `T-0002 — config loading scaffolding`

## 2. Outcome summary

State one of:

- completed
- partially completed
- blocked
- no meaningful change made

This should be explicit.

If partially completed, say so plainly.

## 3. What changed

Describe the meaningful changes made.

Focus on:
- implementation changes
- document/spec changes
- task/workspace changes
- behavior changes

Prefer concise bullets over vague prose.

### Good examples
- added role-specific config structs for root, coordinator, and node
- added config loader and validation entrypoints
- wired `-config` into command startup
- updated `agents/CONTEXT.md` and `agents/TASKS.md`

### Bad examples
- improved things
- fixed stuff
- made progress
- refactored code

---

## 4. Files changed

List the files that were changed in a reviewable way.

Group by purpose when useful, for example:

- command entrypoints
- internal packages
- tests
- specs/docs
- agent workspace files

If many files changed, summarize by area rather than dumping a giant unreadable list.

---

## 5. What was verified

State exactly what was verified.

Examples:
- `go build ./...`
- `go test ./...`
- targeted package tests
- startup of a command with valid config
- startup of a command with intentionally invalid config

Be specific.

If verification was partial, say which parts were verified and which were not.

---

## 6. Tests and benchmarks run

State clearly:

### Tests
- what tests were added
- what tests were run
- whether they passed or failed

### Benchmarks
- what benchmarks were added
- what benchmarks were run
- the measured result, if relevant
- whether the result was actually informative

If no tests or benchmarks were added/run, say that plainly.

Do not fake importance where none exists.

### Good examples
- added table-driven validation tests for root/coordinator/node config
- ran `go test ./...`, all passed
- no benchmarks were added because this task was config scaffolding and no hot path was introduced

---

## 7. What is incomplete

State clearly what remains unfinished.

This section is required whenever the task is not fully complete.

Examples:
- config loads but sample fixtures are not added yet
- command startup exists but status output is still placeholder-level
- validation exists for root/coordinator, node service validation still incomplete

Do not hide unfinished work behind optimistic language.

---

## 8. Blockers, risks, or tensions

State any blockers or important tensions discovered.

A blocker report should include:

- what was verified
- what was ruled out
- the current best explanation
- the exact blocker
- the smallest useful next step

If there is no blocker, say:
- no blocker identified

Risks or tensions may include:
- spec ambiguity
- package-boundary discomfort
- a likely future refactor point
- an implementation shortcut that should not spread

---

## 9. Spec/architecture alignment check

State whether the result still aligns with the relevant specs.

At minimum, report whether the run preserved:

- v1 boundaries
- object-model distinctions
- trust/admission separation
- control/data separation
- generic core model

If a change creates tension with the specs, say so explicitly.

Do not silently drift architecture.

---

## 10. Agents workspace updates

List which `agents/` files were updated.

Examples:
- `agents/CONTEXT.md`
- `agents/TASKS.md`
- `agents/tasks/T-0002-config-loading-scaffolding.md`
- `agents/MEMORY.md`
- `agents/logs/...`

If none were updated, say why.

Remember: for Transitloom, maintaining `agents/` context and memory is part of the work.

---

## 11. Recommended next step

End with the smallest useful next step.

This should be concrete and actionable.

Examples:
- implement coordinator/root bootstrap validation next
- add service declaration validation to node config
- add config fixtures for command startup tests
- begin T-0003 root/coordinator bootstrap scaffolding

Do not end with vague advice like:
- continue working
- do more testing
- refine later

---

## Required honesty rules

### Do not claim success without verification
If you did not run builds/tests/checks, say so.

### Do not blur partial and complete
If only part of the task is done, say “partially completed.”

### Do not hide blockers
If blocked, say what the blocker is.

### Do not report imagined results
Only report what actually happened.

### Do not omit important tension
If the implementation feels misaligned with the specs, record that.

---

## Recommended concise template

Use this template for most runs:

### Objective
...

### Outcome summary
Completed / Partially completed / Blocked / No meaningful change made

### What changed
- ...
- ...

### Files changed
- ...
- ...

### What was verified
- ...
- ...

### Tests and benchmarks
- Tests:
  - ...
- Benchmarks:
  - ...

### What is incomplete
- ...
- ...

### Blockers, risks, or tensions
- ...
- ...

### Spec/architecture alignment check
- ...
- ...

### Agents workspace updates
- ...
- ...

### Recommended next step
- ...

---

## Special rule for blocked runs

If the run is blocked, the report must still be useful.

A blocked run report is acceptable only if it clearly states:

- what the agent tried
- what was learned
- what was ruled out
- why the blocker is real
- what the smallest next useful move is

“Blocked” is acceptable.  
“Blocked without useful information” is not.

---

## Special rule for no-change runs

Sometimes the correct outcome is not changing code.

If that happens, the report should still state:

- what was investigated
- what was verified
- why no change was made
- what the next useful action is

A no-change run can still be valuable if it reduces uncertainty.

---

## Relationship to other files

This file works together with:

- `AGENTS.md`
- `agents/CODING.md`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`
- `agents/tasks/*.md`

Use this file for **how to report**.  
Use `agents/TASKS.md` and task files for **what is being worked on**.  
Use `agents/CONTEXT.md` and `agents/MEMORY.md` for **what should persist**.

---

## One-sentence reporting rule

At the end of a run, report exactly what changed, exactly what was verified, exactly what remains incomplete, and exactly what the next useful step is.

---
