# agents/tasks/T-0022-candidate-refresh-and-revalidation-automation-basics.md

## Task ID

T-0022

## Title

Candidate refresh and revalidation automation basics

## Status

Completed

## Purpose

Implement the first bounded automation layer for refreshing distributed path candidates and revalidating their supporting endpoint/reachability assumptions.

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

Its job is to establish the first explicit refresh loop so that candidate state, endpoint freshness, and reachability verification do not remain static longer than they should.

This task should create:

- bounded refresh triggers
- explicit candidate invalidation / revalidation behavior
- narrow automation around stale reachability inputs
- a clearer path from modeled candidate state to maintained candidate state

This task should **not** become a broad convergence engine or a giant controller loop framework.

---

## Why this task matters

Transitloom already has, or is expected to have:

- explicit external endpoint modeling
- targeted endpoint probing
- distributed path candidates
- live path quality inputs
- scheduler/runtime consumption of candidate state

But those layers still need bounded maintenance behavior.

Without this task, the system risks:
- keeping stale path candidates around too long
- relying on endpoint knowledge that is no longer valid
- making scheduler/runtime decisions from candidate sets that lag reality
- requiring too much manual/operator intervention after unhealthy/down events

The goal here is not “continuous perfect convergence forever.”  
The goal is:

**add the first honest automation for refreshing and revalidating candidate state**

---

## Objective

Add the minimum useful automation needed so that Transitloom can refresh distributed path candidates and revalidate their underlying endpoint assumptions in response to explicit freshness and health signals.

The implementation should remain:

- bounded
- explicit
- observable
- conservative
- compatible with the current staged architecture

This task should not become a distributed control loop platform.

---

## Scope

This task includes:

- defining explicit triggers for candidate refresh / revalidation, such as:
  - endpoint staleness
  - unhealthy/down path signals
  - expired or weak measurement freshness
  - failed targeted reachability checks
- defining the minimum automation behavior for:
  - marking candidates stale
  - selecting candidates/endpoints for revalidation
  - requesting refreshed candidate state from the coordinator
  - clearing or degrading unusable candidate inputs
- preserving clear distinction between:
  - endpoint freshness
  - candidate freshness
  - measured quality freshness
  - chosen/applied runtime path
- adding focused tests for non-trivial refresh/revalidation behavior

This task may include:

- focused helpers under `internal/node`
- focused helpers under `internal/controlplane`
- focused helpers under `internal/transport`
- focused helpers under `internal/scheduler`
- small status/observability additions if useful
- small CLI inspection hooks if useful

---

## Non-goals

This task does **not** include:

- broad autonomous routing convergence
- a giant distributed reconciliation framework
- arbitrary full-network rescan behavior
- full NAT traversal redesign
- scheduler redesign
- broad policy engine behavior
- hidden always-on control loops with unclear boundaries

Do not accidentally turn this into “implement dynamic routing convergence.”

---

## Design constraints

This task must preserve these architectural rules:

- endpoint freshness remains distinct from candidate freshness
- candidate freshness remains distinct from measured quality freshness
- candidate refresh does **not** itself equal chosen runtime path change
- final path selection remains endpoint-owned
- refresh/revalidation behavior must remain explicit and observable
- stale state must not silently look current
- broad blind scanning must not become the default remediation path

Especially important:

- do **not** make refresh automation opaque
- do **not** silently delete candidate information without clear reason/state
- do **not** let revalidation loops become uncontrolled
- do **not** blur refresh policy with scheduler policy

---

## Expected outputs

This task should produce, at minimum:

1. Explicit candidate refresh / revalidation triggers
2. A narrow automation path for refreshing stale candidate inputs
3. Clear state transitions for stale, degraded, and refreshed candidate data
4. Focused tests for non-trivial automation behavior
5. A better operational bridge from static candidate distribution to maintained candidate quality

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can react explicitly to stale or unhealthy candidate-supporting inputs
2. candidate refresh/revalidation behavior is bounded and observable
3. candidate freshness remains distinct from endpoint freshness and measured quality freshness
4. refreshed state remains distinct from chosen/applied runtime path
5. the implementation does not claim full convergence or dynamic routing semantics
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

- `internal/node/...`
- `internal/controlplane/...`
- `internal/transport/...`
- `internal/scheduler/...`

Possibly:
- `internal/status/...`
- `cmd/tlctl/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define narrow stale/refresh triggers
2. define clear candidate refresh state transitions
3. wire a small refresh/revalidation path using existing endpoint/probe/candidate layers
4. keep scheduler/runtime consumption separate from refresh logic
5. add focused tests
6. update task/context/memory files as needed

Keep the automation bounded and explicit.

Do **not** prematurely add:
- giant controller loops
- hidden convergence engines
- broad network scanning fallbacks
- speculative distributed reconciliation complexity

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - stale candidate-supporting inputs trigger the intended refresh path
  - refresh state transitions are explicit and correct
  - revalidation is bounded and observable
  - refreshed candidate state remains distinct from runtime-applied path state

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: hidden control-loop complexity
This is the biggest risk in this task.

Do not let bounded refresh automation become an opaque convergence system.

### Risk 2: state collapse
Do not merge endpoint freshness, candidate freshness, and measured quality freshness into one vague “stale” concept.

### Risk 3: scheduler boundary erosion
Do not let refresh logic silently act as path selection logic.

### Risk 4: excessive revalidation
Do not trigger too much probing or candidate refresh without clear limits.

### Risk 5: weak observability
Do not make it hard to understand why a candidate was refreshed, degraded, or invalidated.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower multi-WAN policy refinement task
- or a narrower relay/direct fallback refinement task
- or a later operator workflow refinement based on refresh behavior

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has the first bounded automation loop for keeping candidate-supporting state reasonably fresh and usable.

---
