# agents/tasks/T-0019-live-path-quality-measurement-basics.md

## Task ID

T-0019

## Title

Live path quality measurement basics

## Status

Completed

## Purpose

Implement the first live path-quality measurement baseline for Transitloom.

This task comes after:

- T-0002 — config loading scaffolding
- T-0003 — root/coordinator bootstrap scaffolding
- T-0004 — node identity and admission-token scaffolding
- T-0005 — minimal node-to-coordinator control session
- T-0006 — service registration basics
- T-0007 — association basics
- T-0008 — direct raw UDP carriage basics
- T-0009 — WireGuard-over-mesh direct-path validation
- T-0010 — single relay hop basics
- T-0011 — scheduler baseline and multi-WAN refinement
- T-0012 — control-plane transport hardening
- T-0013 — runtime observability and debugging basics
- T-0014 — scheduler-to-carrier integration
- T-0015 — external endpoint advertisement and DNAT-aware reachability basics
- T-0016 — tlctl runtime inspection and operator workflows basics
- T-0017 — targeted external endpoint probing and freshness revalidation basics
- T-0018 — path candidate distribution and consumption basics

Its job is to make Transitloom’s path selection inputs more real by adding the first live measurement baseline for path quality.

This task should create:

- explicit live path-quality inputs
- narrow measurement for key metrics such as RTT, loss, and confidence
- runtime updates to candidate/path-quality state
- a usable foundation for later scheduler refinement

This task should **not** become a full traffic-engineering platform or a large measurement subsystem.

---

## Why this task matters

Transitloom already has or is expected to have:

- scheduler baseline logic
- direct and relay-assisted paths
- endpoint advertisement/reachability modeling
- path candidates
- scheduler-to-carrier integration

But the scheduler and path selection become much more meaningful when they can consume real path quality signals instead of only static or manually populated values.

Without this task:
- scheduling remains more theoretical than practical
- path preference may be too static
- observability and operator trust in scheduler behavior will be weaker

The goal here is not “perfect measurement forever.”  
The goal is:

**introduce the first bounded, explicit, usable live path-quality measurement layer**

---

## Objective

Add the minimum useful live path-quality measurement scaffolding needed so that Transitloom can update path-quality state with real observed signals.

The implementation should remain:

- explicit
- bounded
- measurable
- reviewable
- honest about confidence and limits

This task should not broaden into an uncontrolled measurement framework.

---

## Scope

This task includes:

- defining the minimum live path-quality measurement model
- measuring and updating at least a narrow subset of path-quality inputs such as:
  - RTT
  - packet loss indication
  - confidence / measurement freshness
- associating measured quality with appropriate path/candidate/runtime state
- making measured quality available for later scheduler use or observability
- preserving the distinction between:
  - candidate existence
  - measured quality
  - chosen runtime behavior
- adding focused tests for non-trivial measurement/state-update behavior
- adding small useful benchmarks if a repeated measurement or aggregation helper clearly warrants it

This task may include:

- focused helpers under `internal/scheduler`
- focused helpers under `internal/dataplane`
- focused helpers under `internal/node`
- focused helpers under `internal/status`
- narrow supporting changes under `internal/controlplane` if needed for measurement signal transport
- small reporting/CLI hooks if useful

---

## Non-goals

This task does **not** include:

- perfect network measurement
- broad congestion-control research
- machine-learned scheduling
- large multi-dimensional policy engines
- arbitrary passive/active measurement of everything
- encrypted carriage redesign
- broad transport redesign
- production-complete traffic engineering

Do not accidentally turn this into “build a network measurement lab.”

---

## Design constraints

This task must preserve these architectural rules:

- measurement is an input to scheduling, not the same thing as scheduling itself
- measured quality is **not** the same thing as candidate existence
- measured quality is **not** the same thing as authorization or association truth
- confidence/freshness must remain explicit
- live measurement must not quietly override architecture boundaries
- relays do not become independent global schedulers

Especially important:

- do **not** overclaim confidence when measurement is sparse or stale
- do **not** hide freshness/aging behavior
- do **not** make measurement logic so broad that it becomes hard to reason about
- do **not** collapse path-quality state into one vague score with no inspectable meaning

---

## Expected outputs

This task should produce, at minimum:

1. A minimal live path-quality measurement model
2. Runtime updates for at least a bounded set of useful path-quality signals
3. Explicit confidence/freshness handling for measured state
4. Focused tests for non-trivial measurement behavior
5. Optional targeted benchmarks where measurement helpers are clearly repeated/hot
6. A better foundation for later scheduler refinement

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can update at least a bounded set of path-quality inputs using real observed signals
2. measured quality remains distinct from candidate existence and runtime-applied path choice
3. confidence/freshness semantics are explicit and usable
4. measured quality can be surfaced for later scheduler use and/or observability
5. the implementation does not falsely claim perfect measurement or fully optimized traffic engineering
6. the implementation remains aligned with:
   - `spec/v1-data-plane.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
7. `go build ./...` succeeds
8. tests pass

---

## Files likely touched

Expected primary files:

- `internal/scheduler/...`
- `internal/dataplane/...`
- `internal/node/...`
- `internal/status/...`

Possibly:
- `internal/controlplane/...`
- `cmd/tlctl/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define a narrow measurement state model
2. define freshness/confidence handling explicitly
3. add bounded live measurement for key signals
4. wire measured updates into path-quality state
5. expose measured state for later scheduler/reporting use
6. add focused tests
7. add small benchmarks only where clearly useful
8. update task/context/memory files as needed

Keep the measurement layer narrow and explicit.

Do **not** prematurely add:
- giant scoring engines
- opaque heuristics
- broad passive/active probing platforms
- excessive complexity beyond current runtime needs

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - measured RTT/loss/confidence state updates correctly
  - stale/aged quality data is represented clearly
  - measurement state remains distinct from candidate existence
  - scheduler/reporting consumers can inspect the state cleanly

If benchmarks are added, they should be run and reported.

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: false confidence
This is the biggest risk in this task.

Do not let sparse or stale measurements look authoritative.

### Risk 2: score collapse
Do not reduce all measurement meaning into one opaque score too early.

### Risk 3: measurement sprawl
Do not let a bounded measurement task become a giant instrumentation subsystem.

### Risk 4: consumer confusion
Do not let later scheduler or reporting code confuse measured quality with actual chosen runtime behavior.

### Risk 5: hidden freshness semantics
Do not hide aging or confidence decay behavior.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower scheduler refinement task
- or a narrower multi-WAN policy refinement task
- or a later measurement/observability integration task

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a real, bounded, freshness-aware live path-quality measurement baseline that later scheduler refinement can build on safely.

---
