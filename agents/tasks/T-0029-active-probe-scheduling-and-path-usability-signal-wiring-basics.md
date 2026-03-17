# agents/tasks/T-0029-active-probe-scheduling-and-path-usability-signal-wiring-basics.md

## Task ID

T-0029

## Title

Active probe scheduling and path usability signal wiring basics

## Status

Queued

## Purpose

Implement the first bounded active-probe scheduling loop and path-usability signal wiring for Transitloom.

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
- T-0026 — path change event history and audit basics
- T-0027 — control-plane session resume and state reconciliation basics
- T-0028 — config profile and policy bundling basics

Its job is to make Transitloom’s existing reachability and fallback logic more operationally truthful by driving live probe execution on a bounded schedule and wiring the resulting signals into freshness, quality, and usability state.

This task should create:

- a bounded active probe loop
- explicit scheduling of targeted probe work
- wiring of probe results into endpoint freshness and path-quality state
- more truthful `directUsable` / path usability signals for runtime behavior

This task should **not** become a giant network scanning engine, broad measurement platform, or uncontrolled background controller.

---

## Why this task matters

Transitloom already has or is expected to have:

- external endpoint modeling
- targeted probing primitives
- endpoint freshness tracking
- live path-quality storage
- candidate refresh/revalidation automation
- direct/relay fallback and recovery behavior

But an important gap remains:

- probe primitives exist
- quality stores exist
- fallback policy exists

yet the system still needs a bounded loop that actually **drives** probe execution and updates these layers over time.

Without this task, runtime logic can remain structurally correct but operationally weaker than intended, especially around:

- whether a direct candidate is really still usable
- whether stale/failed endpoint state should change
- whether path-quality signals are based on current probe activity
- whether fallback/recovery decisions reflect current reality

The goal here is not “build a full active measurement framework.”  
The goal is:

**wire the first bounded active probe loop into Transitloom’s existing state model**

---

## Objective

Add the minimum useful active probe scheduling and signal-wiring behavior needed so that Transitloom can periodically and explicitly execute targeted probes, then feed those results into endpoint freshness, path-quality state, and path-usability decisions.

The implementation should remain:

- bounded
- targeted-first
- explicit
- observable
- compatible with the existing staged architecture

This task should not become a broad autonomous probing or network-measurement system.

---

## Scope

This task includes:

- defining a bounded active probe schedule / cadence for targeted probe execution
- selecting probe targets from existing known candidate/endpoint state, such as:
  - configured endpoints
  - router-discovered endpoints
  - previously successful endpoints
  - coordinator-observed + configured-port combinations
  - refresh/revalidation-selected endpoints
- wiring successful/failed probe outcomes into:
  - endpoint freshness / verification state
  - path-quality store updates
  - refined candidate usability inputs
  - runtime direct/relay fallback-relevant signals
- keeping probe scheduling bounded and inspectable
- adding focused tests for non-trivial probe scheduling / signal-wiring behavior

This task may include:

- focused helpers under `internal/transport`
- focused helpers under `internal/node`
- focused helpers under `internal/scheduler`
- focused helpers under `internal/status`
- narrow support under `internal/controlplane` if needed for coordinator-assisted probing
- small `tlctl` or status additions if useful

---

## Non-goals

This task does **not** include:

- blind full-range probing by default
- uncontrolled background scanning
- a giant passive/active measurement framework
- broad scheduler redesign
- broad policy redesign
- arbitrary peer discovery
- a full NAT traversal stack
- a giant control loop platform

Do not accidentally turn this into “build a full network probing subsystem.”

---

## Design constraints

This task must preserve these architectural rules:

- probing remains **targeted-first**
- endpoint freshness remains distinct from measured path quality
- measured path quality remains distinct from chosen runtime path
- probe scheduling remains distinct from scheduler policy
- fallback/recovery remains downstream of better signals, not merged with probe logic
- stale or failed probe results must not silently look healthy
- absent measurement is **not** the same as failed measurement

Especially important:

- do **not** make broad scanning the default fallback
- do **not** let the probe loop grow without clear bounds
- do **not** collapse endpoint verification, path quality, and chosen-path state into one vague “health”
- do **not** make the active loop uninspectable

---

## Expected outputs

This task should produce, at minimum:

1. A bounded active probe scheduling loop
2. Explicit targeted probe target selection
3. Wiring from probe results into endpoint freshness and path-quality state
4. More truthful usability signals for runtime selection/fallback behavior
5. Focused tests for non-trivial probe-loop and signal-wiring behavior
6. A better operational bridge from probing primitives to runtime path behavior

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can execute bounded targeted probes on a schedule or bounded trigger path
2. probe target selection remains targeted-first and bounded
3. probe results update endpoint freshness/verification state explicitly
4. probe results update live path-quality state explicitly
5. runtime path-usability inputs become more truthful than before
6. the implementation does not claim full active measurement or broad scanning semantics
7. the implementation remains aligned with:
   - `spec/v1-data-plane.md`
   - `spec/v1-service-model.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
8. `go build ./...` succeeds
9. tests pass

---

## Files likely touched

Expected primary files:

- `internal/transport/...`
- `internal/node/...`
- `internal/scheduler/...`
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

1. identify the narrowest bounded loop that can drive existing probe primitives
2. define explicit target selection inputs
3. define explicit scheduling / cadence / bounded-trigger behavior
4. wire probe outcomes into endpoint freshness and path-quality updates
5. ensure runtime usability consumers can see the improved signals
6. add focused tests
7. update task/context/memory files as needed

Keep the probe loop narrow and explicit.

Do **not** prematurely add:
- giant schedulers/controllers
- broad internet scanning fallbacks
- excessive probe frequency or fan-out
- hidden background behavior with weak observability

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - targeted probe scheduling selects only intended endpoints
  - probe results update endpoint freshness correctly
  - probe results update path-quality state correctly
  - absent measurement is distinct from failed measurement
  - improved usability signals are visible to downstream runtime logic
  - the implementation remains bounded and explainable

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: uncontrolled probe growth
This is the biggest risk in this task.

Do not let a bounded probe loop become an always-on broad scanning subsystem.

### Risk 2: state collapse
Do not merge endpoint freshness, path quality, and chosen-path state into one vague “health” signal.

### Risk 3: scheduler boundary erosion
Do not let the probe loop become hidden path-policy logic.

### Risk 4: misleading signal semantics
Do not treat missing measurements the same as failed measurements.

### Risk 5: poor observability
Do not make it hard for operators to tell what is being probed, why, and what changed as a result.

---

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0024 — multi-WAN policy and hysteresis basics`
- or a narrower operator/diagnostics refinement that benefits from improved live usability signals
- or a later advanced probing/policy task if clearly justified

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a bounded active probe loop that makes existing freshness, quality, and fallback layers more operationally truthful.

---
