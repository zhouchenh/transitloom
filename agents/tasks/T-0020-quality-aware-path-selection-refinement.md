# agents/tasks/T-0020-quality-aware-path-selection-refinement.md

## Task ID

T-0020

## Title

Quality-aware path selection refinement

## Status

Completed

## Purpose

Implement the first refinement layer that connects distributed path candidates and live path-quality measurements into more realistic runtime path selection.

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
- T-0019 — live path quality measurement basics

Its job is to make path selection more operationally meaningful by combining:

- coordinator-distributed path candidates
- endpoint freshness / reachability state
- live measured path quality
- existing scheduler semantics

This task should create the first explicit bridge between candidate distribution and quality-aware scheduler inputs, without turning Transitloom into a full traffic-engineering system.

---

## Why this task matters

Transitloom now has, or is expected to have:

- direct and relay-assisted carriage
- scheduler baseline logic
- scheduler-to-carrier integration
- distributed path candidates
- external endpoint reachability/freshness state
- live path-quality measurement

But those layers are still only partially connected.

Without this task, the system risks having:
- distributed candidate data that is not strongly informed by reachability/quality freshness
- live measurement data that is not fully incorporated into candidate consumption/refinement
- scheduler behavior that remains correct but less practical than it could be

The goal here is not “perfect path optimization forever.”  
The goal is:

**make path selection more quality-aware and freshness-aware using the layers that now exist**

---

## Objective

Add the minimum useful refinement needed so that Transitloom can consume distributed path candidates together with endpoint freshness and live path-quality information in a coherent way before scheduling/runtime application.

The implementation should remain:

- endpoint-owned for final selection
- explicit
- bounded
- observable
- honest about stale or weak data

This task should not become a broad optimization or policy engine.

---

## Scope

This task includes:

- defining how distributed path candidates are refined or filtered using:
  - external endpoint freshness/reachability state
  - live measured path quality
- defining or implementing a narrow “candidate enrichment/refinement” layer before scheduling
- making stale or failed endpoint state affect candidate usability explicitly
- making measured quality freshness affect candidate quality explicitly
- ensuring scheduler inputs reflect this refined candidate view
- adding focused tests for non-trivial refinement behavior

This task may include:

- focused helpers under `internal/scheduler`
- focused helpers under `internal/node`
- focused helpers under `internal/transport`
- focused helpers under `internal/controlplane`
- small reporting/status additions if useful
- small benchmarks if a clearly repeated refinement helper warrants them

---

## Non-goals

This task does **not** include:

- broad scheduler redesign
- full path-policy engine behavior
- machine-learned path ranking
- arbitrary multi-hop path refinement
- broad transport redesign
- full convergence logic
- production-complete traffic engineering
- generic NAT traversal redesign

Do not accidentally turn this into “build a path optimization framework.”

---

## Design constraints

This task must preserve these architectural rules:

- final runtime scheduling remains endpoint-owned
- candidate existence is **not** the same as candidate usability
- candidate usability is **not** the same as chosen runtime path
- endpoint freshness and path quality remain distinct inputs
- stale reachability data must not silently look healthy
- stale path quality data must not silently look current
- direct and relay-assisted candidates remain distinct

Especially important:

- do **not** collapse endpoint freshness and measured path quality into one opaque score too early
- do **not** hide why a candidate was downgraded, excluded, or preferred
- do **not** let refinement blur the line between distributed candidate state and local runtime application
- do **not** make relays independent schedulers

---

## Expected outputs

This task should produce, at minimum:

1. A narrow candidate refinement/enrichment layer
2. Explicit use of endpoint freshness/reachability state during candidate consumption
3. Explicit use of measured path quality freshness during candidate enrichment
4. Clear scheduler input behavior based on refined candidates
5. Focused tests for non-trivial refinement behavior
6. A cleaner bridge from distributed candidates to runtime path choice

---

## Acceptance criteria

This task is complete when all of the following are true:

1. distributed path candidates can be refined/enriched using endpoint freshness and measured quality
2. stale/failed endpoint state affects candidate usability explicitly
3. stale/weak measurement state affects candidate quality explicitly
4. refined candidate state remains distinct from final chosen runtime path
5. the implementation does not claim full traffic-engineering or broad optimization semantics
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
- `internal/node/...`
- `internal/transport/...`

Possibly:
- `internal/controlplane/...`
- `internal/status/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define a narrow refinement/enrichment function or layer
2. consume endpoint freshness/reachability state explicitly
3. consume measured path quality/freshness explicitly
4. produce refined scheduler inputs with visible reasons/flags
5. add focused tests
6. add small benchmarks only if clearly useful
7. update task/context/memory files as needed

Keep the refinement layer narrow and explicit.

Do **not** prematurely add:
- broad policy engines
- hidden ranking heuristics
- giant score-composition frameworks
- speculative optimization complexity

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - stale endpoint state degrades or excludes candidates correctly
  - stale measurement state is handled explicitly
  - refined candidates remain distinct from chosen runtime paths
  - scheduler inputs become more realistic and observable

If benchmarks are added, they should be run and reported.

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: opaque refinement logic
This is the biggest risk in this task.

Do not let candidate refinement become a hidden scoring black box.

### Risk 2: state collapse
Do not merge endpoint freshness, measurement freshness, and runtime application state into one vague result.

### Risk 3: scheduler boundary erosion
Do not let refinement logic become a second scheduler.

### Risk 4: over-optimization
Do not turn a refinement task into a broad traffic-engineering system.

### Risk 5: misleading observability
Do not make it hard to explain why a candidate was usable, degraded, or excluded.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower control-plane transport-security maturation task
- or a narrower path-refresh / candidate-refresh automation task

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a coherent, freshness-aware, quality-aware bridge from distributed candidate data to runtime scheduling inputs.

---
