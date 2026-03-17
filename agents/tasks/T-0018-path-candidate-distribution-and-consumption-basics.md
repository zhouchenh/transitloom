# agents/tasks/T-0018-path-candidate-distribution-and-consumption-basics.md

## Task ID

T-0018

## Title

Path candidate distribution and consumption basics

## Status

Completed

## Purpose

Implement the first explicit path-candidate distribution and consumption flow for Transitloom.

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

Its job is to establish the first coordinator-mediated path-candidate flow so that later runtime path selection can use explicitly distributed candidate data rather than relying only on local/bootstrap assumptions.

This task should create:

- a minimal path-candidate object shape
- a minimal coordinator-side path-candidate generation/distribution flow
- node-side consumption of distributed candidates
- a clear separation between:
  - association
  - external endpoint knowledge
  - path candidate
  - chosen runtime path

This task should **not** become a full routing-policy engine or a broad path-optimization system.

---

## Why this task matters

Transitloom now already has:

- external endpoint modeling
- endpoint freshness/probing ideas
- direct carriage
- single relay hop
- scheduler baseline
- scheduler-to-carrier integration planning

But later runtime behavior should not depend only on local config or ad hoc endpoint guesses.

The system needs a clean, explicit concept of:

- what candidate paths exist for an association
- which of them are direct vs relay-assisted
- what metadata is known about them
- what nodes should consume for later scheduling/runtime use

The goal here is not “perfect path computation forever.”  
The goal is:

**introduce the first honest coordinator-mediated path-candidate layer**

---

## Objective

Add the minimum useful implementation scaffolding for path-candidate distribution and consumption.

The implementation should remain:

- explicit
- association-bound
- reviewable
- narrow in scope
- honest about candidate vs chosen path semantics

This task should not broaden into a full path-policy engine.

---

## Scope

This task includes:

- defining a minimal `PathCandidate` representation suitable for coordinator distribution
- defining direct vs relay-assisted candidate distinctions explicitly
- defining the minimum metadata needed for candidate usefulness
- adding coordinator-side generation/representation of candidate sets for an association
- adding node-side consumption/storage of distributed candidate sets
- preserving the distinction between:
  - endpoint advertisement
  - path candidate
  - chosen path
  - active forwarding state
- adding focused tests for non-trivial candidate modeling/distribution behavior

This task may include:

- focused helpers under `internal/controlplane`
- focused helpers under `internal/coordinator`
- focused helpers under `internal/node`
- focused helpers under `internal/scheduler`
- narrow supporting changes under `internal/transport`
- small reporting/status hooks if useful

---

## Non-goals

This task does **not** include:

- final production path ranking
- scheduler redesign
- broad dynamic routing policy
- arbitrary multi-hop path construction
- broad relay-discovery redesign
- generic NAT traversal completion
- full live path measurement
- traffic engineering optimization
- automatic full convergence logic

Do not accidentally turn this into “implement routing.”

---

## Design constraints

This task must preserve these architectural rules:

- path candidate is **not** the same thing as relay candidate
- path candidate is **not** the same thing as chosen runtime path
- path candidate is **not** the same thing as forwarding state
- candidate distribution remains association-bound
- direct vs relay-assisted candidate distinctions remain explicit
- endpoint advertisement remains a supporting input, not the final runtime decision itself
- v1 hop constraints remain intact

Especially important:

- do **not** collapse external endpoint advertisement into path-candidate truth
- do **not** present candidate presence as proof of verified runtime success
- do **not** introduce arbitrary multi-hop candidate logic
- do **not** blur distributed candidate data with local runtime-applied decision state

---

## Expected outputs

This task should produce, at minimum:

1. A minimal distributed path-candidate model
2. Coordinator-side association-bound candidate generation/representation
3. Node-side candidate consumption/storage
4. Clear direct vs relay-assisted candidate semantics
5. Focused tests for non-trivial candidate behavior
6. A clean foundation for later runtime selection/refinement work

---

## Acceptance criteria

This task is complete when all of the following are true:

1. the coordinator can represent and distribute minimal path-candidate data for an association
2. nodes can consume and store that candidate data
3. direct vs relay-assisted candidates remain explicitly distinct
4. candidate data remains distinct from chosen runtime path and forwarding state
5. the implementation does not claim full routing-policy or final optimization semantics
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

- `internal/controlplane/...`
- `internal/coordinator/...`
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

1. define a narrow `PathCandidate` model
2. define direct and relay-assisted candidate variants or flags explicitly
3. add coordinator-side candidate generation/serialization
4. add node-side candidate consumption/storage
5. keep candidate vs chosen-path boundaries explicit
6. add focused tests
7. update task/context/memory files as needed

Keep the model narrow and explicit.

Do **not** prematurely add:
- broad policy engines
- automatic optimization frameworks
- multi-hop candidate graphs
- hidden ranking heuristics with no observability

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - coordinator candidate generation is correct
  - node candidate consumption is correct
  - direct vs relay-assisted distinctions are preserved
  - candidate presence does not imply chosen/applied runtime behavior
  - state boundaries remain clear

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: collapsing candidate and chosen-path semantics
This is the biggest risk in this task.

Do not let the distributed candidate model quietly become runtime truth.

### Risk 2: endpoint overloading
Do not let advertised endpoint data pretend to be a complete path candidate without explicit modeling.

### Risk 3: routing creep
Do not let candidate distribution become broad routing-policy machinery.

### Risk 4: multi-hop drift
Do not introduce candidate logic that exceeds v1 hop limits.

### Risk 5: weak observability
Do not make candidate state difficult to inspect or reason about later.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower live path-quality measurement task
- or a narrower runtime candidate-refresh/refinement task

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a clean distributed path-candidate layer that future selection and refinement work can build on safely.

---
