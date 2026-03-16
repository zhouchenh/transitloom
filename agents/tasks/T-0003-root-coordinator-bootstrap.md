# agents/tasks/T-0003-root-coordinator-bootstrap.md

## Task ID

T-0003

## Title

Root and coordinator bootstrap scaffolding

## Status

Completed

## Purpose

Implement the first trust/bootstrap scaffolding for the Transitloom root authority and coordinator roles.

This task comes after config loading scaffolding and before full node identity, admission-token, and control-session implementation. Its job is to create the minimal trust bootstrap path and runtime shape needed for the later PKI/admission work.

---

## Why this task matters

Transitloom’s trust model is foundational.

Before node enrollment, admission tokens, or control sessions can become real, the repository needs a clean root/coordinator bootstrap layer that makes these things explicit:

- what a root authority is in code
- what a coordinator is in code
- how a coordinator relates to the root trust anchor
- where root/coordinator trust material lives
- how startup validates that required trust state exists

Without this scaffolding, later PKI and admission work will either:
- be forced into the wrong package boundaries, or
- invent trust behavior ad hoc inside control-plane code

This task prevents that.

---

## Objective

Add the minimum implementation scaffolding required for:

- root authority startup
- coordinator startup with root trust awareness
- coordinator intermediate presence/placeholder handling
- clean separation between:
  - root authority role
  - coordinator role
  - future node-facing trust operations

This task should build the trust/bootstrap shape, not the full issuance system.

---

## Scope

This task includes:

- defining root/coordinator bootstrap-related types and package boundaries
- loading root-authority config and persisted trust material paths
- loading coordinator config and persisted trust material paths
- validating that required trust references exist or are coherently absent when bootstrapping
- defining placeholder startup behavior for:
  - `transitloom-root`
  - `transitloom-coordinator`
- adding initial bootstrap-oriented status/logging

This task may also include:
- a minimal local persistence layout decision for root/coordinator trust material
- stubs or placeholders for future issuance operations
- basic CLI subcommands in `tlctl` only if they help bootstrap shape without overexpanding scope

---

## Non-goals

This task does **not** include:

- full node certificate issuance
- full admission-token issuance
- node enrollment
- node-to-coordinator control sessions
- service registration
- association creation
- data-plane forwarding
- full trust-rotation workflows
- root rollover logic
- full distributed coordinator-security-state replication logic

Do not accidentally widen this task into “implement the entire PKI.”

---

## Design constraints

This task must preserve the following architectural rules:

- root authority is **not** a normal node-facing coordinator target
- root authority is logically separate from coordinator behavior
- coordinator intermediates exist under one root
- routine node issuance should later depend on coordinator intermediates, not root online presence
- trust state must not be silently mixed into generic control-plane runtime state
- config is not distributed truth
- persisted runtime state is distinct from static config

The goal is to build clean scaffolding that later PKI/admission work can attach to.

---

## Expected outputs

This task should produce, at minimum:

1. Clear package-level scaffolding for:
   - root authority logic
   - coordinator trust bootstrap logic
   - trust-material loading/validation
   - future intermediate-related hooks

2. `transitloom-root` startup that can:
   - load config
   - validate required root-related settings
   - report root bootstrap state clearly
   - exit cleanly or report useful errors

3. `transitloom-coordinator` startup that can:
   - load config
   - validate root trust anchor references
   - validate coordinator intermediate-related references or bootstrap expectations
   - report coordinator trust bootstrap state clearly
   - exit cleanly or report useful errors

4. A minimal persistence expectation for root/coordinator trust-related local state

---

## Acceptance criteria

This task is complete when all of the following are true:

1. `transitloom-root` can start with config and clearly report:
   - where root material is expected
   - whether root material exists
   - whether bootstrap state is valid enough for the current placeholder stage

2. `transitloom-coordinator` can start with config and clearly report:
   - which root trust anchor it expects
   - where coordinator intermediate material is expected
   - whether bootstrap state is valid enough for the current placeholder stage

3. Root and coordinator trust/bootstrap code lives in sensible package boundaries and does not collapse:
   - root authority role
   - coordinator role
   - future node identity issuance role

4. The code remains aligned with:
   - `spec/v1-pki-admission.md`
   - `spec/v1-config.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`

5. `go build ./...` succeeds

---

## Files likely touched

Expected primary files:

- `internal/pki/...`
- `internal/identity/...`
- `internal/coordinator/...`
- `internal/config/...`
- `cmd/transitloom-root/main.go`
- `cmd/transitloom-coordinator/main.go`

Possibly:
- `internal/status/...`
- `internal/objectmodel/...`
- `cmd/tlctl/...` if a tiny bootstrap helper is justified

---

## Suggested implementation approach

A good first approach is:

1. define root/bootstrap-related types
2. define coordinator trust-bootstrap-related types
3. define local file/path expectations for root and coordinator trust material
4. load config
5. validate the bootstrap-related trust references
6. print or log clear startup/bootstrap state
7. fail early and clearly on invalid bootstrap conditions

Keep this explicit and boring.

Do **not** try to solve final issuance workflows yet.

---

## Verification

Minimum verification should include:

- `go build ./...`
- startup of `transitloom-root` with:
  - valid bootstrap-style config
  - intentionally invalid config
- startup of `transitloom-coordinator` with:
  - valid bootstrap-style config
  - intentionally invalid config
- confirmation that error paths are useful and specific

If lightweight tests are added, focus on:
- trust/bootstrap config validation
- root/coordinator trust state loading behavior

---

## Risks to watch

### Risk 1: turning bootstrap scaffolding into full PKI implementation
Resist this. This task is about structure and startup path, not final issuance logic.

### Risk 2: collapsing root and coordinator roles
Do not let convenience blur the distinction between root authority and ordinary coordinator behavior.

### Risk 3: hiding trust expectations inside generic startup code
Trust/bootstrap expectations should remain explicit and visible.

### Risk 4: inventing distributed truth too early
This task should stay local/bootstrap-oriented. Do not try to solve full global trust-state propagation here.

---

## Completion notes

Completed on 2026-03-16.

Implemented:

- `internal/pki` trust-bootstrap inspection for root and coordinator roles
- trust-material path resolution relative to `storage.data_dir` for local relative references
- explicit root bootstrap states for:
  - ready trust material
  - initialization required when root material is absent and `trust.generate_key=true`
- explicit coordinator bootstrap states for:
  - ready root-anchor plus intermediate material
  - awaiting intermediate material when the root anchor exists but both intermediate files are absent
- early failure on:
  - missing coordinator root trust anchor
  - partial root trust material
  - partial coordinator intermediate material
- placeholder startup reporting in:
  - `transitloom-root`
  - `transitloom-coordinator`
- table-driven tests for valid and invalid bootstrap-state handling

Verified:

- `go test ./...`
- `go build ./...`
- manual startup of `transitloom-root` with valid and invalid bootstrap-style config
- manual startup of `transitloom-coordinator` with valid and invalid bootstrap-style config

The implementation intentionally stops short of:

- root/intermediate issuance workflows
- admission-token logic
- node enrollment
- control sessions

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0004 — node identity and admission-token scaffolding`

unless implementation reveals a smaller prerequisite task is needed first.

The important outcome is that the repository now has a real and reviewable root/coordinator trust bootstrap shape that later PKI/admission logic can build on safely.

---
