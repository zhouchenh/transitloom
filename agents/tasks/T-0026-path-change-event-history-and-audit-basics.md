# agents/tasks/T-0026-path-change-event-history-and-audit-basics.md

## Task ID

T-0026

## Title

Path change event history and audit basics

## Status

Queued

## Purpose

Implement the first explicit path-change event history and audit baseline for Transitloom.

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
- T-0024 — multi-WAN policy and hysteresis basics
- T-0025 — operator path diagnostics and explainability basics

Its job is to establish the first durable, operator-visible history of important path-related decisions and state changes, without turning Transitloom into a full event-stream platform.

This task should create:

- explicit path-change event records
- bounded recent history for operators and debugging
- enough explanation context to answer “what changed and why”
- a useful foundation for later operational analysis and debugging

This task should **not** become a giant observability backend, analytics platform, or full audit subsystem.

---

## Why this task matters

By this point, Transitloom has or is expected to have:

- distributed path candidates
- endpoint freshness and probing
- live measured path quality
- candidate refinement
- direct/relay fallback behavior
- multi-WAN policy and hysteresis
- operator-facing explainability

That means path behavior is no longer static. Operators will increasingly need to answer questions like:

- when did this association switch from direct to relay
- when did a candidate become stale
- when did revalidation succeed or fail
- when did policy suppress a switch
- what was the previous chosen path

Without a bounded history layer:
- current status may be visible, but recent causal history is not
- debugging still requires stitching together multiple snapshots mentally
- operator confidence remains limited when path behavior changes over time

The goal here is not “build a full audit warehouse.”  
The goal is:

**capture the first bounded, useful history of path changes and reasons**

---

## Objective

Add the minimum useful path-event history needed so that Transitloom can retain and surface recent path-related changes in a structured, operator-friendly way.

The implementation should remain:

- bounded
- read-oriented
- architecture-preserving
- explicit about event meaning
- practical for debugging

This task should not broaden into full long-term persistence or analytics.

---

## Scope

This task includes:

- defining explicit path/event record types for important state changes such as:
  - chosen-path change
  - direct→relay fallback
  - relay→direct recovery
  - candidate excluded / candidate restored
  - endpoint stale / verified / failed
  - revalidation started / succeeded / failed
  - policy/hysteresis hold decision if relevant
- adding a bounded in-memory recent history store or equivalent lightweight mechanism
- surfacing recent history through status and/or `tlctl`
- preserving the distinction between:
  - current status
  - recent events
  - long-term audit persistence
- adding focused tests for non-trivial event/history behavior

This task may include:

- focused helpers under `internal/status`
- focused helpers under `internal/node`
- focused helpers under `internal/scheduler`
- focused helpers under `cmd/tlctl`
- small output-format helpers

---

## Non-goals

This task does **not** include:

- long-term persistent audit storage
- a centralized event bus
- full analytics or reporting pipelines
- a web UI
- distributed event correlation across the whole system
- broad security/compliance audit guarantees

Do not accidentally turn this into “build an event platform.”

---

## Design constraints

This task must preserve these architectural rules:

- event history must not replace current state summaries
- event history must not overclaim causality when only partial reason data exists
- chosen runtime path remains distinct from candidate state and policy state
- bounded history must remain bounded and observable
- event records must preserve important distinctions rather than flattening them

Especially important:

- do **not** merge all path-related changes into one vague event type
- do **not** store huge unbounded history by default
- do **not** imply stronger forensic guarantees than the implementation really provides
- do **not** make event output so verbose that it becomes unusable

---

## Expected outputs

This task should produce, at minimum:

1. Explicit path-change/event record types
2. A bounded recent history mechanism
3. Operator-visible path history output
4. Focused tests for non-trivial event/history behavior
5. A better debugging story for dynamic path behavior

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can record recent path-related events in a structured way
2. recent history is bounded and inspectable
3. important path changes and reasons can be surfaced to operators materially more easily than before
4. current state and history remain distinct
5. the implementation does not claim full long-term audit or analytics semantics
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

- `internal/status/...`
- `internal/node/...`
- `internal/scheduler/...`
- `cmd/tlctl/...`

Possibly:
- `internal/transport/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. identify the most important path changes worth recording
2. define narrow event types and fields
3. add a bounded recent-history store
4. expose recent history through status and/or `tlctl`
5. add focused tests
6. update task/context/memory files as needed

Keep the history layer narrow and explicit.

Do **not** prematurely add:
- persistence backends
- giant event schemas
- broad cross-system correlation
- speculative analytics complexity

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - important path changes generate the expected events
  - bounded history behaves correctly
  - operator-visible history output is useful and not misleading
  - current state remains distinct from history

If tests are added, prefer:
- focused table-driven tests
- small fixture-driven tests for output/history behavior

---

## Risks to watch

### Risk 1: vague event semantics
This is the biggest risk in this task.

Do not let event history become a pile of generic messages with weak meaning.

### Risk 2: unbounded growth
Do not let recent history grow without limits.

### Risk 3: false causality
Do not imply stronger causal certainty than the data really supports.

### Risk 4: duplication
Do not duplicate too much logic that should be shared with existing status/explainability helpers.

### Risk 5: operator overload
Do not drown useful recent history in too much detail by default.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower operator workflow refinement task
- or a later persistence/export task if bounded history proves insufficient
- or a later resilience analysis task informed by real event patterns

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a bounded, operator-usable recent history of path changes and reasons.

---
