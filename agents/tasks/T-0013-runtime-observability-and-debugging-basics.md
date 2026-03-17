# agents/tasks/T-0013-runtime-observability-and-debugging-basics.md

## Task ID

T-0013

## Title

Runtime observability and debugging basics

## Status

Completed

## Purpose

Implement the first explicit runtime observability and debugging baseline for Transitloom.

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

Its job is to make Transitloom substantially easier to operate, inspect, and debug while preserving the current staged v1 architecture.

This task should create:

- clearer runtime status surfaces
- better introspection of important in-memory state
- more useful counters and summaries
- better debugging visibility for control-plane and data-plane behavior
- a practical baseline for future operational hardening

This task should **not** become a full monitoring platform or a broad metrics ecosystem redesign.

---

## Why this task matters

Transitloom will increasingly rely on the interaction of:

- control-plane transport
- trust/bootstrap state
- service registration
- associations
- direct carriage
- relay-assisted carriage
- scheduler behavior

As the system grows, it becomes harder to reason about failures and runtime state unless observability improves alongside features.

Without this task, future work risks becoming slower and more fragile because:
- bugs will take longer to isolate
- state mismatches will be harder to inspect
- control-plane/data-plane behavior will become opaque
- agents and humans will waste time re-deriving what the system is currently doing

The goal here is not “ship full production monitoring.”  
The goal is:

**make Transitloom inspectable enough that future implementation and debugging are safer and faster**

---

## Objective

Add the minimum useful observability and debugging baseline for Transitloom runtime behavior.

The implementation should remain:

- explicit
- reviewable
- low-complexity
- useful to both humans and coding agents
- honest about what is and is not surfaced

This task should improve operational clarity without turning into a giant telemetry project.

---

## Scope

This task includes:

- adding clearer runtime status views or status-reporting helpers
- exposing useful summaries for important runtime objects and counters
- improving visibility into:
  - node bootstrap/readiness state
  - coordinator bootstrap/readiness state
  - registered services
  - associations
  - direct carriage state
  - relay-assisted carriage state, if present
  - scheduler/path-selection state, if present
- improving debugging-oriented error and status reporting
- adding focused tests for non-trivial status/reporting logic

This task may include:

- focused helpers under `internal/status`
- focused helpers under `internal/node`
- focused helpers under `internal/coordinator`
- narrow supporting changes under `internal/controlplane`, `internal/dataplane`, `internal/service`, or `internal/scheduler`
- small CLI/status output improvements in command entrypoints if useful
- small benchmarks only if a repeated reporting helper clearly warrants it

---

## Non-goals

This task does **not** include:

- a full metrics backend
- a full Prometheus/OpenTelemetry integration
- a distributed tracing platform
- a large web UI
- broad admin API redesign
- production-perfect audit logging
- broad control-plane redesign
- broad data-plane redesign
- replacing existing task/context reporting with runtime telemetry

Do not accidentally turn this into “build a monitoring stack.”

---

## Design constraints

This task must preserve these architectural rules:

- observability must reflect actual system boundaries instead of blurring them
- control-plane state and data-plane state remain distinct
- identity and admission remain distinct
- service registration, association state, forwarding state, and scheduler state remain distinct
- reporting must not falsely imply stronger transport/auth guarantees than actually exist
- observability should help explain behavior, not hide it under abstraction

Especially important:

- do not merge multiple concepts into one vague “status”
- do not expose misleading status that implies live authorization where only cached/bootstrap state exists
- do not build logging/status that is so noisy it becomes unusable
- do not lose reviewability for the sake of “more telemetry”

---

## Expected outputs

This task should produce, at minimum:

1. A clearer runtime observability/status baseline
2. Useful summaries for major runtime object categories
3. Better debugging-oriented visibility for control-plane and data-plane behavior
4. Clearer counters and/or summaries for key activity
5. Focused tests for non-trivial status/reporting logic
6. A better base for future debugging and operational hardening

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom exposes significantly clearer runtime status/debugging information than before
2. major runtime object/state categories can be inspected or summarized without collapsing their meanings
3. reporting remains aligned with real system semantics
4. debugging a control-plane or data-plane issue is materially easier than before
5. the implementation does not falsely claim a fully mature monitoring system
6. the implementation remains aligned with:
   - `spec/v1-architecture.md`
   - `spec/v1-control-plane.md`
   - `spec/v1-data-plane.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
7. `go build ./...` succeeds
8. tests pass

---

## Files likely touched

Expected primary files:

- `internal/status/...`
- `internal/node/...`
- `internal/coordinator/...`

Possibly:
- `internal/controlplane/...`
- `internal/dataplane/...`
- `internal/service/...`
- `internal/scheduler/...`
- `cmd/transitloom-node/main.go`
- `cmd/transitloom-coordinator/main.go`
- `cmd/tlctl/main.go`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. identify the most important runtime questions that are currently hard to answer
2. define narrow, explicit summaries for those state categories
3. add clear status/debug output helpers
4. improve counters/reporting where the signal is useful
5. add focused tests
6. add small targeted benchmarks only if a repeated status helper is clearly hot
7. update task/context/memory files as needed

Keep the observability model narrow and explicit.

Do **not** prematurely add:
- a giant generic observability framework
- hidden status aggregation that erases boundaries
- extremely verbose logging by default
- speculative telemetry integrations with no immediate value

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - key runtime states can be surfaced clearly
  - summaries preserve important architectural distinctions
  - debugging output is useful and not misleading
  - counters/status change appropriately where relevant
  - new reporting does not silently change core behavior

If benchmarks are added, they should be run and reported.

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: blurred status semantics
This is the biggest risk in this task.

Do not let reporting merge distinct concepts into ambiguous “state.”

### Risk 2: noisy observability
Too much output can become almost as bad as too little.

### Risk 3: misleading confidence
Do not present bootstrap/cached/placeholder state as stronger truth than it really is.

### Risk 4: framework creep
Do not turn a practical observability baseline into a broad telemetry platform.

### Risk 5: hidden behavior changes
Status/reporting code must not quietly alter runtime semantics.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower runtime-hardening or operator-tooling task
- or another explicitly revealed integration/measurement task

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom becomes materially easier to inspect and debug without violating the staged v1 architecture.

---
