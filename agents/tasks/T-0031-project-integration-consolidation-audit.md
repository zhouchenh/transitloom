# agents/tasks/T-0031-project-integration-consolidation-audit.md

## Task ID

T-0031

## Title

Project integration consolidation audit

## Status

Completed

## Purpose

Perform a focused post-integration audit of the Transitloom codebase so the project has an accurate, up-to-date view of what is:

- fully implemented and live
- implemented but not yet operationally wired
- inspectable but not yet consumed by runtime logic
- cleanly separated by architecture boundaries
- drifting, duplicated, weakly integrated, or at risk of semantic confusion

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
- T-0029 — active probe scheduling and path usability signal wiring basics
- T-0030 — live node probe-loop lifecycle integration basics

Its job is **not** to add a large new feature.

Its job is to tell the truth about the current integrated system and produce a reliable roadmap from the actual codebase state.

---

## Why this task matters

Transitloom has recently accumulated a large number of interdependent changes across:

- node runtime behavior
- candidate refresh and revalidation
- direct/relay fallback and recovery
- multi-WAN stickiness/hysteresis
- active probe scheduling and probe-loop lifecycle
- path explainability and recent event history
- control-plane reconnect/reconciliation
- policy profiles and effective config resolution
- secure control transport maturation

This is now beyond the point where task-by-task completion reports alone are sufficient to understand the real project state.

Without a focused consolidation audit, the project risks:

- mistaking “implemented” for “live and operational”
- leaving inspectable config/profile fields only partially consumed
- allowing runtime/status/reporting surfaces to drift semantically
- carrying duplicate or near-duplicate concepts across packages
- making roadmap decisions from incomplete integration understanding

The goal here is not “read everything and say vague things.”  
The goal is:

**produce a high-confidence integration report from the actual codebase state**

---

## Objective

Perform a focused integration audit that answers, from the current repo state:

1. what is fully implemented **and live**
2. what is implemented but only **scaffolded / helper-only / partially wired**
3. what config/profile fields are **resolved but not yet consumed**
4. which runtime/status/reporting boundaries are clean
5. which areas show semantic drift, duplication, weak integration, or unclear ownership
6. what the **top next tasks** should be, based on actual code, not just prior plans

This task should produce a highly actionable report, not just general observations.

---

## Scope

This task includes:

- reading the current integrated code and workspace docs
- verifying key runtime/control/config/status surfaces against each other
- identifying fully live vs partially wired areas
- identifying resolved-but-not-consumed config/policy fields
- identifying boundary confusion or duplicate concept risks
- identifying likely cleanup or follow-up tasks
- updating `agents/TASKS.md`, `agents/CONTEXT.md`, and optionally `agents/MEMORY.md` if the audit changes the project’s known state materially

This task may include:

- small corrections to `agents/CONTEXT.md`
- small corrections to `agents/TASKS.md`
- small corrections to `agents/MEMORY.md`
- **very small, obviously safe** clarifying edits if a report would otherwise be misleading

This task should **not** become a broad implementation task.

---

## Non-goals

This task does **not** include:

- building a large new feature
- broad refactoring
- broad cleanup just because something could be cleaner
- speculative redesign
- rewriting large parts of the spec set
- rewriting working code unless a small correction is necessary to produce a truthful report

Do not accidentally turn this into “fix everything found.”

---

## Required reading / inspection surfaces

At minimum, inspect these areas carefully:

### Commands
- `cmd/transitloom-node/main.go`
- `cmd/transitloom-coordinator/main.go`
- `cmd/tlctl/main.go`

### Node/runtime
- `internal/node/...`

### Scheduler / runtime path selection
- `internal/scheduler/...`

### Status / explainability / history
- `internal/status/...`

### Control plane / coordinator
- `internal/controlplane/...`
- `internal/coordinator/...`

### Transport / probing / endpoint reachability
- `internal/transport/...`

### Config / effective policy
- `internal/config/...`

### Specs and workspace memory
- `agents/TASKS.md`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `spec/v1-architecture.md`
- `spec/v1-data-plane.md`
- `spec/v1-control-plane.md`
- `spec/v1-config.md`
- `spec/implementation-plan-v1.md`

---

## Audit questions that must be answered

The audit must explicitly answer at least these questions.

### A. Live vs scaffolded
- Which features are fully live in runtime?
- Which features exist only as helper code or test-only scaffolding?
- Which features are implemented but not yet started in live node/coordinator lifecycle?

### B. Effective policy consumption
- Which `EffectivePolicy` fields are already consumed by runtime?
- Which are inspectable/configurable but not yet actually used?
- Which runtime components still use defaults or constants instead of resolved policy?

### C. Runtime signal layering
- Are endpoint freshness, candidate freshness, path quality, fallback state, stickiness state, and chosen runtime path still cleanly separated?
- Are there places where those concepts are drifting together or being double-interpreted?

### D. Status/reporting coherence
- Do `internal/status`, `tlctl`, and runtime summaries reflect current runtime truth accurately?
- Are there duplicated or semantically inconsistent status surfaces?
- Are there places where operator-visible output overstates certainty or completeness?

### E. Control-plane realism
- What parts of control-plane transport/security/reconciliation are truly live?
- What parts remain scaffolded or incomplete?
- Is the current secure control path clearly distinguished from bootstrap-only behavior?

### F. Technical debt / next tasks
- What are the top 5 highest-value next tasks **from the actual current state**?
- Which previously planned tasks should be reordered, split, or dropped?

---

## Expected output

This task should produce a report that is structured around the actual current system, such as:

1. **Implemented and live**
2. **Implemented but not fully wired**
3. **Configured/inspectable but not runtime-consumed**
4. **Boundary checks: clean / risky / drifting**
5. **Operator-visible truthfulness assessment**
6. **Top technical debt / follow-up opportunities**
7. **Recommended next 5 tasks in priority order**

The report should be concrete, evidence-based, and specific.

---

## Acceptance criteria

This task is complete when all of the following are true:

1. it produces a trustworthy integrated view of current Transitloom state
2. it distinguishes live vs scaffolded vs inspectable-but-not-consumed behavior
3. it identifies important integration gaps and semantic drift risks
4. it proposes next tasks based on actual codebase state
5. it does not drift into broad implementation work
6. `go build ./...` succeeds
7. `go test ./...` succeeds
8. relevant `agents/` files are updated if the audit changes project understanding materially

---

## Files likely touched

Expected primary files:

- `agents/TASKS.md`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`

Possibly:
- very small clarifying edits in nearby docs/status code if absolutely necessary for truthfulness

This task should minimize code churn.

---

## Suggested implementation approach

A good approach is:

1. inspect the required runtime/control/config/status surfaces
2. verify current repo state with build/tests
3. classify features into live / scaffolded / inspectable-but-not-consumed
4. identify the top integration mismatches or drift points
5. update workspace context/memory to reflect actual project state
6. produce the final audit report
7. make only minimal necessary changes, if any

Keep the task audit-focused and truth-oriented.

---

## Verification

Minimum verification should include:

- `git status`
- `git log --oneline -20`
- `go build ./...`
- `go test ./...`

And targeted inspection of the required files listed above.

---

## Risks to watch

### Risk 1: vague audit
This is the biggest risk in this task.

Do not produce a generic “things look good” report.

### Risk 2: turning audit into implementation
Do not let this become a broad cleanup/refactor task.

### Risk 3: shallow reading
Do not rely only on agent task files; inspect actual code and runtime surfaces.

### Risk 4: overclaiming certainty
Do not claim a subsystem is fully live if it is only partially wired.

### Risk 5: roadmap inertia
Do not simply repeat the existing roadmap if the code suggests a better next order.

---

## Completion handoff

When this task is complete, it should leave the project with:

- a more accurate understanding of what Transitloom currently is
- a corrected sense of which gaps remain
- a stronger basis for the next wave of tasks

The next tasks after this audit should be chosen from the actual findings, not assumed in advance.

---
