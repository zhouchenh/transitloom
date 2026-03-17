# agents/tasks/T-0021-control-plane-transport-security-maturation.md

## Task ID

T-0021

## Title

Control-plane transport security maturation

## Status

Completed

## Purpose

Implement the next maturation step for Transitloom control-plane transport security, moving beyond bootstrap-only HTTP scaffolding toward the intended authenticated transport direction.

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

Its job is to advance Transitloom’s control-plane transport closer to the intended secure model, while preserving staged delivery and avoiding overclaiming maturity.

This task should create:

- a more explicitly authenticated control-plane transport foundation
- clearer security boundaries for control transport
- a narrower step toward QUIC + TLS 1.3 mTLS primary and TCP + TLS 1.3 fallback
- better separation between bootstrap-only transport and matured authenticated transport

This task should **not** become a total rewrite of the control plane or a giant protocol redesign.

---

## Why this task matters

Transitloom already has:

- bootstrap control sessions
- control-plane hardening
- richer runtime state
- more operational dependency on control-plane correctness

But the control plane still needs to mature beyond bootstrap-only semantics if the system is to become more trustworthy and operationally realistic.

Without this task, the project risks:
- keeping bootstrap-only transport in place too long
- accumulating more control-plane semantics on top of insufficient security guarantees
- making later security migration harder

The goal here is not “finish transport security forever.”  
The goal is:

**take the next honest, bounded step toward the intended authenticated control-plane transport**

---

## Objective

Add the minimum useful transport-security maturation needed so that Transitloom’s control-plane transport is meaningfully closer to the intended secure architecture.

The implementation should remain:

- staged
- explicit
- reviewable
- honest about what is and is not yet final
- compatible with the intended direction of:
  - QUIC + TLS 1.3 mTLS as primary
  - TCP + TLS 1.3 fallback

This task should not pretend to fully finish the control-plane transport architecture unless it truly does so.

---

## Scope

This task includes:

- replacing or narrowing bootstrap-only transport behavior where appropriate
- introducing a more explicit authenticated transport layer or boundary
- tightening trust/identity use in control-plane transport setup
- improving secure transport lifecycle/reporting/verification behavior
- adding focused tests for non-trivial transport-security behavior

This task may include:

- focused helpers under `internal/controlplane`
- focused helpers under `internal/node`
- focused helpers under `internal/coordinator`
- focused helpers under `internal/pki`
- narrow config clarifications if strictly necessary
- small observability/status additions if useful
- small benchmarks only if a clearly repeated transport helper warrants them

---

## Non-goals

This task does **not** include:

- a broad control-plane redesign
- unrelated data-plane redesign
- generic message-bus architecture
- speculative protocol framework expansion
- broad service/association feature redesign
- production-perfect security claims beyond what is actually implemented
- rewriting everything at once because bootstrap HTTP exists today

Do not accidentally turn this into “rebuild the whole control plane.”

---

## Design constraints

This task must preserve these architectural rules:

- control plane remains distinct from data plane
- identity and admission remain distinct
- bootstrap semantics remain distinct from final authenticated semantics
- transport security boundaries must be explicit
- role boundaries must remain preserved
- the system should move toward, not away from, the intended v1 control-plane transport direction

Especially important:

- do **not** overclaim security maturity
- do **not** quietly collapse bootstrap and final transport semantics
- do **not** hide trust or auth assumptions in implicit behavior
- do **not** widen the protocol surface unnecessarily
- do **not** abandon observability while increasing security complexity

---

## Expected outputs

This task should produce, at minimum:

1. A more explicitly authenticated control-plane transport foundation
2. Clearer trust/identity handling in transport setup
3. Improved secure transport reporting and failure visibility
4. Focused tests for non-trivial transport-security behavior
5. A better base for later control-plane maturation

---

## Acceptance criteria

This task is complete when all of the following are true:

1. control-plane transport is meaningfully closer to the intended secure architecture than the bootstrap-only baseline
2. trust/identity/security boundaries are more explicit than before
3. the implementation does not falsely claim fully finished production transport security unless it truly provides it
4. the implementation remains aligned with:
   - `spec/v1-control-plane.md`
   - `spec/v1-pki-admission.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
5. `go build ./...` succeeds
6. tests pass

---

## Files likely touched

Expected primary files:

- `internal/controlplane/...`
- `internal/node/...`
- `internal/coordinator/...`
- `internal/pki/...`

Possibly:
- `internal/status/...`
- `internal/config/...`
- `cmd/transitloom-node/main.go`
- `cmd/transitloom-coordinator/main.go`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. identify the smallest meaningful security maturation step beyond bootstrap-only transport
2. make trust/auth boundaries explicit in transport setup
3. preserve observability of transport failures and state
4. add focused tests
5. update task/context/memory files as needed

Keep the maturation step narrow and explicit.

Do **not** prematurely add:
- giant protocol rewrites
- speculative abstraction layers
- hidden security assumptions
- broad unrelated feature work

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - secure transport setup behaves as intended
  - trust/identity assumptions are explicit and testable
  - transport failures remain visible enough to debug
  - bootstrap-only semantics are not mislabeled as final if they still exist

If benchmarks are added, they should be run and reported.

If tests are added, prefer:
- focused table-driven tests
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: security overclaim
This is the biggest risk in this task.

Do not say the control plane is fully secure if the implementation is only partially matured.

### Risk 2: hidden trust assumptions
Do not bury important identity or trust decisions in implicit behavior.

### Risk 3: protocol creep
Do not let a bounded maturation task become a broad redesign.

### Risk 4: observability regression
Do not make the transport more secure but much harder to debug.

### Risk 5: staged-model breakage
Do not break the staged v1 architecture by skipping too many intermediate steps at once.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower control-plane feature maturation task
- or a narrower reachability/selection refresh task
- or a later runtime hardening task revealed by the secure transport step

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a more credible and explicit control-plane transport security foundation without abandoning staged delivery.

---
