# agents/tasks/T-0014-scheduler-to-carrier-integration.md

## Task ID

T-0014

## Title

Scheduler-to-carrier integration

## Status

Queued

## Purpose

Implement the first runtime integration between Transitloom’s scheduler decisions and its actual carriage paths.

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

Its job is to make the scheduler operationally real by wiring scheduler decisions into actual direct and/or relay-assisted egress behavior.

This task should create:

- runtime use of `Scheduler.Decide()`
- explicit carrier/path selection based on scheduler output
- clear handling of single-path vs weighted burst/flowlet vs eligible striping decisions
- observable evidence that scheduler decisions affect real carriage behavior

This task should **not** become a broad rewrite of the scheduler, carriers, or transport architecture.

---

## Why this task matters

Transitloom now has a scheduler baseline from T-0011, but it still exists primarily as a decision layer.

Without this task, the project risks having:

- a correct scheduler model
- good tests
- good observability

but **no real runtime effect on carriage behavior**

That would leave a gap between architecture and operation.

The goal here is not “finish all multi-path behavior forever.”  
The goal is:

**make scheduler decisions actually govern runtime egress behavior in a narrow, explicit, reviewable way**

---

## Objective

Add the minimum useful runtime integration needed so that direct and/or single-relay-hop egress paths can be selected according to scheduler decisions.

The implementation should remain:

- endpoint-owned
- association-bound
- explicit
- observable
- conservative about striping
- honest about what is and is not yet implemented

This task should not broaden into new scheduling research, path discovery redesign, or broad transport refactoring.

---

## Scope

This task includes:

- integrating `internal/scheduler` decisions into real carrier/runtime selection
- making direct and/or relay-assisted egress path usage reflect scheduler output
- wiring scheduler decisions into the relevant egress send path(s)
- preserving the distinction between:
  - scheduler decision
  - forwarding context
  - path candidate
  - relay candidate
  - direct carriage
  - relay-assisted carriage
- making runtime behavior observable enough to confirm that scheduler decisions are actually being applied
- adding focused tests for non-trivial integration behavior

This task may include:

- focused helpers under `internal/dataplane`
- focused helpers under `internal/node`
- focused helpers under `internal/scheduler`
- narrow supporting changes under `internal/transport`
- small observability/reporting changes if useful
- small integration-style tests if they remain simple and reviewable

---

## Non-goals

This task does **not** include:

- redesigning the scheduler model from T-0011
- live path measurement infrastructure
- broad coordinator-distributed path-candidate redesign
- relay ranking redesign
- multi-WAN policy redesign
- arbitrary routing policy
- generic encrypted carriage
- generic TCP carriage
- full production traffic-engineering behavior

Do not accidentally turn this into “rewrite scheduling and transport together.”

---

## Design constraints

This task must preserve these architectural rules:

- scheduling remains **endpoint-owned**
- scheduler decisions remain **association-bound**
- relays do not become independent end-to-end schedulers
- direct and relay-assisted carriage remain distinct concepts
- weighted burst/flowlet-aware remains the default baseline
- per-packet striping remains allowed only when paths are sufficiently closely matched
- zero in-band overhead remains preserved for raw UDP carriage
- integration must not blur scheduler choice into unrestricted forwarding freedom

Especially important:

- do **not** let the relay choose its own unrelated end-to-end schedule
- do **not** activate striping when scheduler gates say not to
- do **not** make runtime behavior contradict the reported scheduler decision
- do **not** hide scheduling effects so deeply that they become unreviewable

---

## Expected outputs

This task should produce, at minimum:

1. Real runtime use of scheduler decisions for egress behavior
2. Correct handling of at least:
   - single-path decision
   - weighted burst/flowlet-aware default decision
   - conditional striping decision, if already practical to apply safely
3. Clear observability showing that scheduler output affects runtime behavior
4. Focused tests for non-trivial scheduler-to-carrier integration
5. A cleaner operational bridge between T-0011 and later multi-WAN refinement work

---

## Acceptance criteria

This task is complete when all of the following are true:

1. runtime egress behavior reflects scheduler decisions rather than ignoring them
2. integration remains endpoint-owned and association-bound
3. direct and relay-assisted path usage remain semantically distinct
4. scheduler output and runtime behavior are observable enough to verify alignment
5. per-packet striping is not applied unless the scheduler explicitly allows it
6. the implementation does not claim fully mature production traffic engineering
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

- `internal/dataplane/...`
- `internal/node/...`
- `internal/scheduler/...`

Possibly:
- `internal/transport/...`
- `internal/status/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. identify the actual egress decision points in direct and/or relay-assisted runtime paths
2. thread scheduler decision inputs into those points explicitly
3. apply single-path and weighted burst/flowlet-aware selection first
4. apply striping only if already practical and safe within current runtime architecture
5. make the applied decision visible in reporting/counters
6. add focused tests
7. update task/context/memory files as needed

Keep the integration narrow and explicit.

Do **not** prematurely add:
- broad scheduler redesign
- hidden adaptive behavior
- transport-wide abstraction rewrites
- speculative optimization machinery

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - scheduler output changes actual egress behavior
  - direct vs relay-assisted selection remains correct
  - striping does not activate outside allowed conditions
  - runtime reporting matches actual applied decision behavior
  - no new hidden forwarding freedom was introduced

If benchmarks are added, they should be run and reported.

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: integration drift from scheduler semantics
This is the biggest risk in this task.

Do not let runtime behavior quietly diverge from the explicit scheduler model.

### Risk 2: hidden carrier-side scheduling
Do not let carriers become their own policy engines.

### Risk 3: harmful striping activation
Do not turn on runtime striping where the gating model does not support it.

### Risk 4: over-integration
Do not rewrite large parts of dataplane/runtime just to “make it elegant.”

### Risk 5: observability mismatch
Do not report one scheduler decision while runtime behavior effectively does another.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower path-quality measurement task
- or a narrower multi-WAN refinement/hardening task
- or a later observability/transport-integration follow-up if implementation reveals a gap

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a scheduler whose decisions actually influence runtime carriage behavior, without violating the v1 architecture.

---
