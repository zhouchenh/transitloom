# agents/tasks/T-0024-multi-wan-policy-and-hysteresis-basics.md

## Task ID

T-0024

## Title

Multi-WAN policy and hysteresis basics

## Status

Completed

## Purpose

Implement the first explicit multi-WAN policy and hysteresis baseline for Transitloom.

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
- T-0023 — direct-relay fallback and recovery basics

Its job is to establish the first explicit policy layer for how Transitloom should behave when multiple eligible uplinks or path groups exist, while also preventing unnecessary path flapping.

This task should create:

- explicit multi-WAN policy inputs
- bounded hysteresis / stability rules
- a clear difference between “best path right now” and “worth switching now”
- operator-visible reasons for path-hold vs path-switch behavior

This task should **not** become a full traffic-engineering policy language or a giant routing-policy framework.

---

## Why this task matters

By this point, Transitloom already has or is expected to have:

- distributed path candidates
- endpoint freshness and revalidation
- live path-quality measurements
- scheduler baseline logic
- direct/relay fallback behavior

That means the next practical problem is no longer just “which path scores better.”

It is also:

- when should the system **stay** on the current path
- when should it **switch**
- how should multiple uplinks or path families be grouped and preferred
- how should churn be limited without hiding policy

Without this task, the system risks:
- frequent switching on weak evidence
- hard-to-explain path changes
- too much sensitivity to short-lived measurement swings
- inadequate operator control over multi-WAN behavior

The goal here is not “perfect WAN optimization forever.”  
The goal is:

**introduce the first bounded, explainable multi-WAN policy and hysteresis layer**

---

## Objective

Add the minimum useful multi-WAN policy and hysteresis behavior needed so that Transitloom can make more stable path-switch decisions when multiple eligible paths exist.

The implementation should remain:

- endpoint-owned
- explicit
- conservative
- observable
- compatible with the existing scheduler/refinement layers

This task should not become a broad policy engine.

---

## Scope

This task includes:

- defining explicit policy inputs for multi-WAN path behavior, such as:
  - path-group or uplink-group identity
  - direct vs relay preference weighting
  - minimum improvement threshold before switching
  - hold-down / cool-down periods
  - preferred-path stickiness rules
- defining bounded hysteresis behavior for:
  - not switching on weak improvement
  - not recovering too aggressively after short improvement spikes
  - handling degraded vs failed paths differently
- integrating hysteresis behavior with existing:
  - candidate refinement
  - quality inputs
  - direct/relay fallback logic
- adding focused tests for non-trivial policy/hysteresis behavior

This task may include:

- focused helpers under `internal/scheduler`
- focused helpers under `internal/node`
- focused helpers under `internal/status`
- narrow config additions under `internal/config`
- small `tlctl` or status-reporting additions if useful

---

## Non-goals

This task does **not** include:

- a broad DSL for traffic engineering
- arbitrary policy scripting
- fully dynamic autonomous WAN optimization
- machine-learned path control
- per-packet policy routing across the whole system
- broad relay-policy redesign
- uncontrolled path exploration

Do not accidentally turn this into “build a WAN policy engine.”

---

## Design constraints

This task must preserve these architectural rules:

- final path choice remains endpoint-owned
- hysteresis policy remains distinct from raw quality measurement
- hysteresis policy remains distinct from candidate generation
- candidate presence is **not** the same as switch-worthiness
- direct vs relay distinction remains explicit
- operator visibility into switch/no-switch reasons must remain possible

Especially important:

- do **not** make switching behavior opaque
- do **not** silently hide current-path stickiness
- do **not** let hysteresis become an uninspectable black box
- do **not** confuse “slightly better” with “worth switching now”

---

## Expected outputs

This task should produce, at minimum:

1. A bounded multi-WAN policy input model
2. Explicit hysteresis / stickiness behavior
3. Clear rules for when switching is suppressed vs allowed
4. Focused tests for non-trivial hysteresis behavior
5. Observable reasons for path hold vs switch decisions

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can apply bounded hysteresis/stickiness behavior when multiple eligible paths exist
2. path switching requires more than trivial short-lived improvement
3. switching/no-switch reasons are explicit enough to inspect
4. policy/hysteresis remains distinct from raw quality measurement and candidate generation
5. the implementation does not claim full traffic-engineering or broad routing-policy semantics
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
- `internal/status/...`

Possibly:
- `internal/config/...`
- `cmd/tlctl/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define narrow multi-WAN policy inputs and hysteresis state
2. define switch/no-switch thresholds explicitly
3. consume existing refined candidate inputs and measured quality
4. make hold/switch reasons observable
5. add focused tests
6. update task/context/memory files as needed

Keep the policy layer narrow and explicit.

Do **not** prematurely add:
- giant policy DSLs
- hidden switching heuristics
- broad automatic optimization frameworks
- uncontrolled oscillation-management complexity

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - trivial improvements do not force switches
  - clear improvements can trigger switches
  - degraded vs failed path behavior is distinguishable
  - hysteresis/stickiness reasons are inspectable
  - current-path hold behavior is explainable

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: opaque hysteresis
This is the biggest risk in this task.

Do not let switch suppression become a hidden policy black box.

### Risk 2: path flapping
Do not allow small measurement noise to cause frequent switching.

### Risk 3: over-sticky behavior
Do not let hysteresis prevent justified recovery or failover.

### Risk 4: boundary collapse
Do not merge policy, measurement, and runtime application into one vague decision layer.

### Risk 5: operator confusion
Do not make it impossible to explain why the system stayed or switched.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower operator workflow refinement task for path policy visibility
- or a narrower recovery/failback hardening task
- or a later advanced multi-WAN refinement task

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a bounded, explainable, and stable multi-WAN switching policy baseline.

---
