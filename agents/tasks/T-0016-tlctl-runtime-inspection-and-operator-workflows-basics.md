# agents/tasks/T-0016-tlctl-runtime-inspection-and-operator-workflows-basics.md

## Task ID

T-0016

## Title

tlctl runtime inspection and operator workflows basics

## Status

Queued

## Purpose

Implement the first operator-facing runtime inspection and workflow baseline for Transitloom through `tlctl`.

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

Its job is to expose the project’s existing runtime state in a practical operator-facing way, without turning `tlctl` into a broad management platform.

This task should create:

- basic `tlctl` inspection commands
- clear summaries of important runtime state
- operator-friendly workflows for checking whether the system is healthy and doing what it claims
- a narrow but useful read-oriented CLI surface

This task should **not** become a full administrative control plane, a large UX redesign, or a broad API redesign.

---

## Why this task matters

By this stage, Transitloom already has or is expected to have:

- bootstrap and trust state
- service registration
- associations
- direct and relay-assisted carriage
- scheduler decisions
- transport hardening
- observability/status internals

That is enough system complexity that humans and future agents need practical read-oriented tooling.

Without this task:
- runtime state may technically exist but remain awkward to inspect
- debugging may still require reading logs or code rather than using a coherent tool
- operational validation of later tasks becomes slower and more error-prone
- the observability work from T-0013 may remain harder to use in practice

The goal here is not “build an admin console.”  
The goal is:

**make Transitloom inspectable through a practical, low-friction operator CLI**

---

## Objective

Add the minimum useful `tlctl` runtime inspection and operator workflow support so that an operator can query important Transitloom state without reading internal code or scraping ad hoc logs.

The implementation should remain:

- read-oriented
- explicit
- narrow
- useful for humans and coding agents
- aligned with existing state boundaries

This task should not broaden into broad write/mutate/admin flows.

---

## Scope

This task includes:

- adding basic `tlctl` subcommands or equivalent inspection flows
- surfacing useful runtime summaries for major state categories
- exposing clearly structured inspection output for at least some of:
  - node bootstrap/readiness state
  - coordinator bootstrap/readiness state
  - registered services
  - associations
  - direct carriage state
  - relay-assisted carriage state
  - scheduler status/counters
  - transport status/errors
  - externally advertised endpoint state and freshness, if available by then
- making the output useful for operator workflows such as:
  - “what is currently registered?”
  - “what associations exist?”
  - “what carriage paths exist?”
  - “what path/scheduler mode is being used?”
  - “what is unhealthy or missing?”
- adding focused tests for non-trivial CLI/status formatting or query behavior

This task may include:

- focused helpers under `cmd/tlctl`
- focused helpers under `internal/status`
- narrow read-only helpers under `internal/node`, `internal/coordinator`, `internal/service`, `internal/dataplane`, `internal/scheduler`, or `internal/controlplane`
- small output-format helpers
- small fixture-driven tests

---

## Non-goals

This task does **not** include:

- broad mutating admin commands
- service creation/deletion through `tlctl`
- association creation/deletion through `tlctl`
- a full REST/gRPC admin API redesign
- a web UI
- a broad configuration management interface
- replacing runtime observability internals with CLI-only logic
- building a giant kubectl-like platform

Do not accidentally turn this into “build the full operator plane.”

---

## Design constraints

This task must preserve these architectural rules:

- CLI output must reflect real system boundaries, not blur them
- control-plane, data-plane, bootstrap, scheduler, service, and association state remain distinct concepts
- bootstrap/cached/placeholder state must not be mislabeled as stronger truth than it really is
- `tlctl` should consume and present existing runtime state, not redefine it
- operator workflows should be helpful without overstating maturity or completeness

Especially important:

- do **not** merge multiple concepts into one vague “status”
- do **not** present “registered” as “authorized for all use”
- do **not** present “configured” as “verified/reachable”
- do **not** make output so verbose that it becomes unusable by default

---

## Expected outputs

This task should produce, at minimum:

1. A practical read-oriented `tlctl` inspection surface
2. Useful summaries for major runtime state categories
3. Operator-friendly output for common validation/debugging workflows
4. Focused tests for non-trivial CLI/status behavior
5. A base for future operator tooling without over-expanding scope

---

## Acceptance criteria

This task is complete when all of the following are true:

1. `tlctl` can inspect at least several major runtime state categories in a practical way
2. output remains aligned with actual Transitloom architectural boundaries
3. operator workflows such as checking services, associations, carriage state, and scheduler status are materially easier than before
4. the implementation remains read-oriented and does not quietly become a broad admin surface
5. the implementation does not falsely imply more operational maturity than actually exists
6. the implementation remains aligned with:
   - `spec/v1-architecture.md`
   - `spec/v1-control-plane.md`
   - `spec/v1-data-plane.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
7. `go build ./...` succeeds
8. tests pass

---

## Files likely touched

Expected primary files:

- `cmd/tlctl/...`
- `internal/status/...`

Possibly:
- `internal/node/...`
- `internal/coordinator/...`
- `internal/service/...`
- `internal/dataplane/...`
- `internal/scheduler/...`
- `internal/controlplane/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. identify the most important runtime questions an operator needs answered
2. map those to existing runtime state or narrow new status helpers
3. add a small set of read-oriented `tlctl` commands
4. make output concise by default and explicit about state meaning
5. add focused tests
6. update task/context/memory files as needed

Keep the operator surface narrow and explicit.

Do **not** prematurely add:
- broad admin mutation flows
- giant output modes by default
- hidden state aggregation that erases boundaries
- an API redesign just to support the CLI

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - `tlctl` can query important runtime states successfully
  - output preserves key distinctions between state categories
  - common operator inspection flows are materially easier than before
  - CLI/status output is useful and not misleading

If tests are added, prefer:
- focused table-driven tests where appropriate
- small fixture-based tests for formatting and query behavior

---

## Risks to watch

### Risk 1: blurred status semantics
This is the biggest risk in this task.

Do not let CLI output merge distinct concepts into one vague status view.

### Risk 2: operator overconfidence
Do not let the tool make bootstrap/cached/placeholder state look stronger than it is.

### Risk 3: too much default output
Do not make the default UX too noisy.

### Risk 4: admin-surface creep
Do not let a read-oriented tooling task become a broad write/mutate control interface.

### Risk 5: duplicated logic
Do not duplicate major chunks of runtime interpretation logic in `tlctl` if shared status helpers should exist instead.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower operator-action or validation harness task
- or a later operational hardening task informed by actual `tlctl` usage

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a practical operator-facing inspection workflow through `tlctl` without violating the staged v1 architecture.

---
