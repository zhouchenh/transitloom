# agents/tasks/T-0011-scheduler-baseline-and-multi-wan-refinement.md

## Task ID

T-0011

## Title

Scheduler baseline and multi-WAN refinement

## Status

Queued

## Purpose

Implement the first scheduler baseline and multi-WAN refinement layer for Transitloom.

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

Its job is to introduce the first real path-selection and traffic-splitting behavior for Transitloom while staying inside the v1 architecture:

- endpoint-owned scheduling
- weighted burst/flowlet-aware baseline behavior
- conditional per-packet striping only when paths are closely matched
- multi-WAN-aware path use
- no collapse into arbitrary routing policy
- no false claim of “fully optimized” transport behavior

This task should create the first practical scheduler that makes Transitloom’s multi-WAN value real without over-expanding scope.

---

## Why this task matters

Transitloom’s v1 value is not just:

- control-plane coordination
- service registration
- association modeling
- direct or relay-assisted raw UDP carriage

It is also:

**practical use of multiple available paths in a way that improves transport value without destroying correctness**

The project has already established:

- direct carriage
- relay-assisted carriage
- WireGuard-over-mesh direct-path validation

The next important capability is:

**an honest first scheduler that decides how to use eligible paths**

This matters because Transitloom is explicitly intended to provide practical multi-WAN aggregation value, not just the existence of multiple paths.

The goal here is not “perfect scheduling forever.”  
The goal is:

**implement the first correct, measurable, reviewable scheduler baseline**

---

## Objective

Add the minimum useful scheduler and path-refinement behavior needed so that Transitloom can choose among eligible paths and, where appropriate, distribute traffic across them.

The implementation should remain:

- endpoint-owned
- explicit
- measurable
- conservative where paths do not match well
- honest about what is and is not implemented

This task should not become a broad routing engine or a free-form optimization framework.

---

## Scope

This task includes:

- defining the minimum scheduler model for eligible paths
- implementing endpoint-owned path selection
- implementing weighted burst/flowlet-aware baseline behavior
- implementing conditional per-packet striping only when paths are sufficiently close in quality
- using available path-quality inputs in a narrow, explicit way
- adding reporting or counters that make scheduling behavior observable
- adding focused tests for non-trivial scheduler behavior

This task may include:

- focused helpers under `internal/scheduler`
- focused helpers under `internal/dataplane`
- focused helpers under `internal/node`
- narrow supporting changes under `internal/controlplane` if additional path metadata delivery is strictly needed
- small integration-style tests if they remain simple and reviewable
- small benchmark coverage if a clearly hot decision path is introduced

---

## Non-goals

This task does **not** include:

- arbitrary routing policy
- uncontrolled per-hop path decisions
- broad dynamic policy engine behavior
- full-blown congestion-control research
- generic machine-learned scheduling
- encrypted generic data-plane support
- generic TCP data-plane support
- broad relay-discovery expansion
- “perfect” network measurement
- production-complete traffic engineering

Do not accidentally turn this task into “implement a networking research platform.”

---

## Design constraints

This task must preserve these architectural rules:

- scheduling is **endpoint-owned**
- relays do not become unconstrained end-to-end traffic schedulers
- direct and relay-assisted paths remain distinct concepts
- weighted burst/flowlet-aware behavior is the v1 default baseline
- per-packet striping is allowed only when paths are **closely matched**
- path use must remain association-bound
- scheduler behavior must not imply arbitrary forwarding freedom
- zero in-band overhead must remain preserved for raw UDP carriage

Especially important:

- do **not** allow every hop to make unrelated traffic-splitting decisions
- do **not** force per-packet striping where path mismatch would likely make it harmful
- do **not** confuse “multiple paths exist” with “all should always be used equally”
- do **not** lose observability of why a path was or was not selected

---

## Expected outputs

This task should produce, at minimum:

1. A minimal scheduler model for eligible paths
2. Endpoint-owned path selection behavior
3. Weighted burst/flowlet-aware baseline scheduling
4. Conditional per-packet striping only for sufficiently matched paths
5. Observable counters/status/reporting for scheduling decisions
6. Focused tests for non-trivial scheduler behavior
7. A measurable base for later refinement

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can choose among eligible direct and/or single-relay-hop paths using endpoint-owned logic
2. the default scheduling behavior is weighted burst/flowlet-aware
3. per-packet striping is only used when paths are sufficiently close in quality
4. scheduling decisions remain association-bound
5. the implementation keeps distinct:
   - path candidate
   - relay candidate
   - direct carriage
   - relay-assisted carriage
   - scheduler decision
6. the implementation does not claim arbitrary routing-policy or fully optimized traffic-engineering support
7. the implementation remains aligned with:
   - `spec/v1-data-plane.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
8. `go build ./...` succeeds
9. tests pass

---

## Files likely touched

Expected primary files:

- `internal/scheduler/...`
- `internal/dataplane/...`
- `internal/node/...`

Possibly:
- `internal/controlplane/...` if a narrow path-metadata input extension is required
- `internal/coordinator/...` if a narrow reporting hook is needed
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define a very small path candidate model for scheduling inputs
2. define scheduler inputs explicitly
3. implement the baseline weighted burst/flowlet-aware decision logic
4. add a conservative rule for when per-packet striping is allowed
5. make scheduling behavior observable
6. add focused tests
7. add small targeted benchmarks only if a real hot-path decision function exists
8. update task/context/memory files as needed

Keep the scheduler narrow and explicit.

Do **not** prematurely add:
- broad policy plug-in systems
- dynamic optimization frameworks
- hidden heuristics with no observability
- per-hop independent schedulers
- speculative complexity with no measurement

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - eligible path selection behaves as expected
  - weighted burst/flowlet-aware decisions are stable and reviewable
  - per-packet striping does not activate for clearly mismatched paths
  - scheduling output is association-bound
  - decision/reporting state is visible enough to debug

If benchmarks are added, they should be run and reported.

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they stay simple and reviewable

---

## Risks to watch

### Risk 1: violating endpoint-owned scheduling
This is the biggest risk in this task.

Do not let relays or intermediate components become uncontrolled end-to-end schedulers.

### Risk 2: harmful striping
Do not enable per-packet striping on badly mismatched paths.

### Risk 3: hidden heuristics
Do not add scheduler behavior that cannot be explained or observed.

### Risk 4: premature optimization complexity
Do not build a large optimization framework before the first baseline scheduler is real and measurable.

### Risk 5: collapsing path concepts
Do not blur:
- path candidate
- relay candidate
- chosen path
- forwarding context

---

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0012 — control-plane transport hardening`  
  or another narrow hardening/measurement task revealed by implementation

unless implementation reveals that a smaller observability or measurement prerequisite should be split out first.

The important outcome is that Transitloom now has a real, explicit, measurable scheduler baseline that makes multi-WAN transport value practical without violating the v1 architecture.

---
