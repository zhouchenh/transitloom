# agents/tasks/T-0028-config-profile-and-policy-bundling-basics.md

## Task ID

T-0028

## Title

Config profile and policy bundling basics

## Status

Queued

## Purpose

Implement the first bounded config-profile and policy-bundling baseline for Transitloom.

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

Its job is to reduce operator configuration friction by introducing the first explicit, bounded way to bundle related path/policy behavior into reusable config profiles.

This task should create:

- reusable config profiles or policy bundles
- a cleaner way to apply consistent path/fallback/hysteresis behavior
- reduced config duplication for common deployment patterns
- a bounded operator-facing config convenience layer

This task should **not** become a giant inheritance framework or a full policy language.

---

## Why this task matters

By this point, Transitloom is expected to expose multiple related configuration areas such as:

- external endpoint advertisement behavior
- probing/revalidation behavior
- candidate refinement behavior
- direct/relay fallback behavior
- multi-WAN hysteresis behavior
- explainability/history controls
- control-plane transport security modes

As the system grows, operators will need a cleaner way to express common patterns such as:

- conservative branch office
- aggressive direct-preference lab
- relay-preferred recovery profile
- operator-heavy diagnostics profile

Without this task:
- config duplication will grow
- consistent policy application becomes harder
- operational drift becomes more likely across nodes/sites

The goal here is not “invent a policy language.”  
The goal is:

**introduce the first bounded and explicit config-profile mechanism for common Transitloom behaviors**

---

## Objective

Add the minimum useful config-profile and policy-bundling behavior needed so that operators can apply reusable, explicit bundles of related Transitloom behavior without copy-pasting the same detailed settings everywhere.

The implementation should remain:

- explicit
- bounded
- transparent
- debuggable
- compatible with the existing config model

This task should not broaden into general-purpose inheritance or scripting.

---

## Scope

This task includes:

- defining a bounded profile/bundle concept for related config behavior
- supporting reuse of grouped settings for areas such as:
  - probing / revalidation
  - fallback / recovery
  - multi-WAN hysteresis
  - observability / explainability
  - control-plane transport mode preferences where appropriate
- preserving the distinction between:
  - raw config values
  - selected profile
  - effective resolved configuration
- exposing enough status/reporting that operators can tell which profile is in effect
- adding focused tests for non-trivial profile resolution behavior

This task may include:

- focused helpers under `internal/config`
- focused helpers under `internal/status`
- small `tlctl` inspection/reporting additions if useful
- narrow supporting changes under runtime packages only where needed for resolved settings consumption

---

## Non-goals

This task does **not** include:

- a giant config inheritance tree
- arbitrary config templating language
- scriptable policy logic
- hidden implicit overrides that are hard to debug
- broad redesign of the existing config model
- central distributed config management

Do not accidentally turn this into “build a configuration language.”

---

## Design constraints

This task must preserve these architectural rules:

- effective configuration must remain inspectable
- profiles must not hide important resolved behavior
- raw config remains distinct from resolved/effective config
- profile selection must not silently erase operator intent
- profile convenience must not come at the cost of debuggability
- bounded explicit profiles are preferred over open-ended inheritance

Especially important:

- do **not** create deep inheritance chains
- do **not** make override order ambiguous
- do **not** make it hard to tell why a resolved setting has a specific value
- do **not** let profile convenience become opaque policy magic

---

## Expected outputs

This task should produce, at minimum:

1. A bounded config-profile / policy-bundle mechanism
2. Explicit resolution of effective settings
3. Clear status/reporting of selected profile and effective behavior
4. Focused tests for non-trivial resolution behavior
5. Reduced duplication for common deployment patterns

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom supports a bounded reusable profile/bundle concept for related behavior
2. effective resolved configuration is explicit and inspectable
3. raw config, selected profile, and effective config remain distinct
4. the implementation does not become a broad inheritance framework or policy language
5. the implementation remains aligned with:
   - `spec/v1-config.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
6. `go build ./...` succeeds
7. tests pass

---

## Files likely touched

Expected primary files:

- `internal/config/...`
- `internal/status/...`

Possibly:
- `cmd/tlctl/...`
- narrow consumers in runtime packages
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. identify the most repeated configuration clusters
2. define a bounded profile/bundle model
3. implement explicit resolution to effective config
4. expose resolved config through status/inspection
5. add focused tests
6. update task/context/memory files as needed

Keep the profile system narrow and explicit.

Do **not** prematurely add:
- arbitrary nesting/inheritance
- scripting
- ambiguous override precedence
- giant config redesign

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - profile selection resolves correctly
  - effective config is inspectable
  - overrides are explicit and predictable
  - profile use reduces duplication without hiding behavior

If tests are added, prefer:
- focused table-driven tests
- small fixture-based resolution tests

---

## Risks to watch

### Risk 1: opaque config resolution
This is the biggest risk in this task.

Do not let profile convenience hide the actual effective configuration.

### Risk 2: inheritance creep
Do not let a bounded profile system become a deep inheritance model.

### Risk 3: ambiguous precedence
Do not make override order hard to reason about.

### Risk 4: boundary erosion
Do not let config convenience blur architecture boundaries or state meaning.

### Risk 5: poor operator visibility
Do not make it hard for operators to inspect which profile is active and why.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower operator workflow refinement task
- or a later deployment-pattern hardening task
- or a later config distribution/management task if truly needed

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a bounded, explicit, and inspectable way to reuse common behavior bundles without a giant configuration framework.

---
