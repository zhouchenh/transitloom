# agents/tasks/T-0025-operator-path-diagnostics-and-explainability-basics.md

## Task ID

T-0025

## Title

Operator path diagnostics and explainability basics

## Status

Queued

## Purpose

Implement the first explicit operator-facing path diagnostics and explainability baseline for Transitloom.

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

Its job is to make Transitloom’s path behavior explainable to operators without requiring them to infer decisions from raw logs or internal code.

This task should create:

- operator-facing explanations of candidate, quality, freshness, policy, and applied-path state
- path diagnostics that answer “why this path?” and “why not that path?”
- read-oriented CLI/status surfaces for path reasoning
- clearer debug workflows around direct vs relay, freshness, quality, fallback, and hysteresis

This task should **not** become a giant troubleshooting platform or a broad admin/control surface.

---

## Why this task matters

By this stage, Transitloom is expected to have:

- distributed path candidates
- endpoint freshness and probing
- live measured path quality
- refined scheduler inputs
- direct/relay fallback behavior
- multi-WAN policy and hysteresis

That means operators will increasingly need to answer questions like:

- why is this association using direct instead of relay
- why did a direct candidate get excluded
- why is a path considered stale
- why did the system stay on the old path
- why did it switch now
- what inputs were missing or weak

Without this task:
- behavior may be correct but still hard to trust
- debugging may require reading code or piecing together multiple reports manually
- operator confidence in path behavior will lag behind implementation maturity

The goal here is not “build a full troubleshooting suite.”  
The goal is:

**make path decisions and non-decisions meaningfully explainable**

---

## Objective

Add the minimum useful operator-facing diagnostics and explainability support so that Transitloom can surface path-decision reasoning in a practical, read-oriented way.

The implementation should remain:

- read-oriented
- explicit
- low-friction
- boundary-preserving
- honest about uncertainty, staleness, and missing data

This task should not broaden into a giant UX or control-plane redesign.

---

## Scope

This task includes:

- defining or surfacing operator-facing explanations for:
  - candidate availability
  - candidate exclusion/degradation
  - endpoint freshness / staleness
  - measured path quality / measurement freshness
  - fallback/failback state
  - hysteresis / stickiness decisions
  - applied runtime path state
- adding read-oriented CLI/status/reporting support for “why” questions, such as:
  - why this path is currently chosen
  - why another path was excluded
  - why the system did not switch
  - why the system switched
- ensuring path explanations preserve distinctions among:
  - candidate state
  - refined candidate usability
  - measured quality
  - policy/hysteresis state
  - applied runtime path
- adding focused tests for non-trivial explainability/diagnostics behavior

This task may include:

- focused helpers under `internal/status`
- focused helpers under `internal/scheduler`
- focused helpers under `internal/node`
- focused helpers under `cmd/tlctl`
- narrow supporting additions under `internal/transport`
- small output-format helpers

---

## Non-goals

This task does **not** include:

- a giant observability platform
- a broad mutation/control interface
- broad log-ingestion or tracing infrastructure
- a web UI
- opaque auto-diagnosis systems
- replacing existing runtime/status layers with new duplicated ones

Do not accidentally turn this into “build a network troubleshooting suite.”

---

## Design constraints

This task must preserve these architectural rules:

- explanations must reflect actual system boundaries
- candidate existence is **not** the same as candidate usability
- candidate usability is **not** the same as chosen runtime path
- measured quality is **not** the same as freshness or policy state
- path explanations must not overclaim confidence
- bootstrap/cached/placeholder state must not be mislabeled as stronger truth than it is

Especially important:

- do **not** hide uncertainty
- do **not** flatten multiple reasons into one vague status
- do **not** present stale or weak data as authoritative
- do **not** make explanations so verbose that they become unusable by default

---

## Expected outputs

This task should produce, at minimum:

1. Operator-facing path explainability surfaces
2. Clear “why chosen / why not chosen” reporting for important path states
3. Read-oriented CLI/status support for path diagnostics
4. Focused tests for non-trivial explanation behavior
5. A more trustworthy operational story for Transitloom path behavior

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can explain important path decisions and non-decisions in a practical operator-facing way
2. path explanations preserve the distinctions among candidate, quality, freshness, policy, and applied state
3. operators can answer basic “why this path / why not that path” questions materially more easily than before
4. the implementation remains read-oriented and does not turn into a broad control surface
5. the implementation does not falsely imply certainty where data is stale, weak, or incomplete
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
- `internal/scheduler/...`
- `internal/node/...`
- `cmd/tlctl/...`

Possibly:
- `internal/transport/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. identify the most important operator “why” questions
2. map those to existing candidate/quality/freshness/policy/applied-path state
3. add narrow explanation helpers
4. expose them through `tlctl` and/or status reporting
5. add focused tests
6. update task/context/memory files as needed

Keep the explainability layer narrow and explicit.

Do **not** prematurely add:
- giant troubleshooting frameworks
- hidden reasoning summaries that erase real state distinctions
- extremely verbose default output
- speculative UX complexity

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - chosen-path explanations are clear
  - excluded/degraded candidate explanations are clear
  - stale/weak data is labeled honestly
  - operators can answer common path-behavior questions materially more easily than before

If tests are added, prefer:
- focused table-driven tests
- small fixture-based tests for explanation output

---

## Risks to watch

### Risk 1: false clarity
This is the biggest risk in this task.

Do not let simplified explanations misrepresent the actual decision state.

### Risk 2: explanation overload
Do not make output so verbose that it becomes unusable.

### Risk 3: state collapse
Do not merge candidate, quality, freshness, and applied-path state into one vague summary.

### Risk 4: confidence overstatement
Do not imply stronger certainty than the data supports.

### Risk 5: duplicated logic
Do not duplicate large parts of runtime decision logic just for explanation output if shared helpers should be used.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower operator workflow refinement task
- or a later advanced policy/optimization refinement task
- or a resilience/debugging hardening task informed by actual operator use

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a practical operator-facing explanation layer for path behavior, not just raw state.

---
