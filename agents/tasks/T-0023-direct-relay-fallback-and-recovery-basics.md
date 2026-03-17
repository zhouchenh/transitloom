# agents/tasks/T-0023-direct-relay-fallback-and-recovery-basics.md

## Task ID

T-0023

## Title

Direct-relay fallback and recovery basics

## Status

Queued

## Purpose

Implement the first explicit fallback and recovery behavior between direct and single-relay-hop carriage for Transitloom.

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
- T-0020 — quality-aware path selection refinement
- T-0021 — control-plane transport security maturation
- T-0022 — candidate refresh and revalidation automation basics

Its job is to establish the first explicit behavior for:

- preferring direct paths when usable
- falling back to single-relay-hop paths when direct usability degrades
- recovering back toward direct when direct reachability becomes usable again

This task should create Transitloom’s first honest direct-vs-relay recovery loop without turning it into a full high-complexity routing system.

---

## Why this task matters

Transitloom already has, or is expected to have:

- direct carriage
- single relay hop
- scheduler logic
- distributed candidate data
- endpoint freshness and probing
- live path-quality data
- candidate refresh/revalidation behavior

But the system still needs an explicit policy for what happens when:

- direct reachability becomes stale or unhealthy
- relay remains usable
- direct later becomes usable again
- both paths exist but their current trustworthiness differs

Without this task, the runtime may have candidate data and scheduling inputs but still lack a clean operational fallback story.

The goal here is not “solve full failover forever.”  
The goal is:

**implement the first bounded direct-to-relay fallback and recovery behavior**

---

## Objective

Add the minimum useful behavior needed so that Transitloom can move between direct and relay-assisted paths in a clear, explicit, bounded way as reachability and freshness conditions change.

The implementation should remain:

- endpoint-owned for final selection
- explicit
- observable
- association-bound
- conservative about churn

This task should not become a giant failover/orchestration framework.

---

## Scope

This task includes:

- defining bounded fallback/recovery conditions between:
  - direct path candidates
  - single-relay-hop candidates
- defining explicit preference behavior such as:
  - direct preferred when fresh and usable
  - relay fallback when direct becomes stale/failed/unusable
  - controlled recovery toward direct when revalidated
- integrating this behavior with existing candidate freshness/quality layers
- preserving the distinction between:
  - candidate presence
  - candidate usability
  - chosen runtime path
  - recovery policy
- adding focused tests for non-trivial fallback/recovery behavior

This task may include:

- focused helpers under `internal/scheduler`
- focused helpers under `internal/node`
- focused helpers under `internal/transport`
- focused helpers under `internal/status`
- narrow controlplane additions if needed for explicit candidate state updates
- small observability/reporting additions if useful

---

## Non-goals

This task does **not** include:

- arbitrary multi-hop recovery logic
- broad routing failover engines
- per-packet route flapping policies
- giant multi-WAN policy systems
- full traffic-engineering optimization
- opaque hysteresis frameworks with no observability
- uncontrolled automatic path thrashing

Do not accidentally turn this into “build a full failover subsystem.”

---

## Design constraints

This task must preserve these architectural rules:

- final path selection remains endpoint-owned
- direct and relay-assisted paths remain distinct concepts
- fallback policy remains distinct from candidate generation
- fallback policy remains distinct from measurement itself
- chosen runtime path remains distinct from candidate presence
- relay fallback does not imply arbitrary routing freedom
- churn avoidance and observability both matter

Especially important:

- do **not** let the system flap rapidly between direct and relay with weak evidence
- do **not** treat stale direct state as healthy
- do **not** make relay fallback invisible to operators
- do **not** erase the distinction between “direct exists” and “direct is currently usable”

---

## Expected outputs

This task should produce, at minimum:

1. Explicit direct-vs-relay fallback conditions
2. Explicit recovery-back-to-direct conditions
3. Clear state/reporting around why a path was chosen or recovered
4. Focused tests for non-trivial fallback/recovery behavior
5. A more operationally realistic direct/relay runtime story

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can explicitly prefer direct when it is fresh/usable
2. Transitloom can explicitly fall back to relay when direct becomes unusable
3. Transitloom can explicitly recover toward direct when direct becomes valid again
4. fallback/recovery behavior remains distinct from candidate generation and raw scheduling inputs
5. the implementation does not claim full dynamic routing or full traffic-engineering semantics
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

1. define narrow direct-vs-relay fallback and recovery rules
2. consume existing candidate freshness and quality signals explicitly
3. ensure chosen runtime path changes are observable and explainable
4. add focused tests
5. update task/context/memory files as needed

Keep the fallback behavior narrow and explicit.

Do **not** prematurely add:
- giant hysteresis engines
- broad failover orchestration systems
- hidden routing complexity
- speculative multi-path failover policies

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - direct is preferred when actually usable
  - relay is chosen when direct is not usable
  - recovery to direct happens only under explicit conditions
  - fallback/recovery behavior is observable and understandable
  - no uncontrolled flapping or hidden path choice logic is introduced

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: path flapping
This is the biggest risk in this task.

Do not let the system oscillate rapidly between direct and relay with weak evidence.

### Risk 2: boundary collapse
Do not merge candidate presence, candidate usability, and chosen runtime path into one vague state.

### Risk 3: hidden policy
Do not make fallback/recovery rules opaque.

### Risk 4: over-ambition
Do not turn a bounded fallback task into a giant failover framework.

### Risk 5: poor operator visibility
Do not make it hard to explain why the system chose direct, fell back to relay, or recovered to direct.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower multi-WAN policy refinement task
- or a narrower operator workflow refinement for fallback/recovery inspection
- or a later broader resilience task if justified by implementation results

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has an explicit, bounded direct/relay fallback and recovery behavior that fits the staged v1 architecture.

---
