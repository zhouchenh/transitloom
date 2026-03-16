# agents/tasks/T-0004-node-identity-and-admission-token-scaffolding.md

## Task ID

T-0004

## Title

Node identity and admission-token scaffolding

## Status

Completed

## Purpose

Implement the first node-side identity and admission scaffolding for Transitloom.

This task comes after:

- T-0002 — config loading scaffolding
- T-0003 — root/coordinator bootstrap scaffolding

Its job is to establish the minimum code structure and persisted-state boundaries needed so future tasks can build:

- node enrollment
- node certificate handling
- admission-token issuance and refresh
- node participation validation
- later control-session authentication

without collapsing identity and authorization into one concept.

---

## Why this task matters

Transitloom’s architecture depends on a strict separation between:

- **who a node is**
- **whether that node is currently allowed to participate**

That separation is one of the most important durable design decisions in the project.

The repository now has:

- config loading scaffolding
- root/coordinator trust-bootstrap scaffolding
- a PKI/admission specification
- an object model
- a staged implementation plan

The next correct step is to build the node-side identity/admission scaffolding in a way that preserves those boundaries before any real node-to-coordinator control session exists.

Without this task, later implementation work would be at risk of:
- blurring identity and admission
- hiding persisted state boundaries
- creating ad hoc auth/admission behavior inside control-session code
- making later revoke behavior harder to implement correctly

---

## Objective

Add the minimum useful implementation scaffolding for:

- node identity material handling
- node identity state presence/validation
- admission-token data structures and local persisted-state handling
- coordinator-side scaffolding for future admission-token validation
- clear startup/bootstrap reporting for node identity/admission readiness

This task should create the structure and boundaries for later identity/admission behavior.

It should **not** implement the full enrollment or authentication workflow yet.

---

## Scope

This task includes:

- defining node-identity-related types and package boundaries
- defining admission-token-related types and package boundaries
- defining local persisted-state expectations for node identity material
- defining local persisted-state expectations for cached/current admission-token material
- loading/inspecting configured or persisted node identity state
- loading/inspecting configured or persisted admission-token state
- clear placeholder reporting from `transitloom-node` about identity/admission bootstrap readiness
- coordinator-side scaffolding that represents future admission-token validation inputs or expectations, where useful and minimal

This task may include:
- focused helpers under `internal/identity`
- focused helpers under `internal/admission`
- small supporting helpers under `internal/node` or `internal/pki` if clearly justified
- tests for node identity/admission state loading and validation behavior
- small local fixture files where useful for tests

---

## Non-goals

This task does **not** include:

- node certificate issuance
- full enrollment workflow
- real coordinator-issued admission-token issuance
- real admission-token refresh logic
- node-to-coordinator control sessions
- service registration
- association creation
- data-plane forwarding
- revocation enforcement through live control sessions
- distributed global admission-state replication logic

Do not accidentally turn this task into “implement full node auth.”

---

## Design constraints

This task must preserve these architectural rules:

- node identity and current participation permission are separate
- valid identity material must not imply current participation permission
- admission-token state must remain conceptually distinct from node identity state
- persisted runtime state is distinct from static config
- this task should prepare future control/auth work, not replace it
- root/coordinator trust bootstrap from T-0003 remains the foundation, but this task must not widen into full issuance or enrollment flows
- object-model distinctions must remain explicit

Especially do **not** collapse:

- `NodeCertificate` semantics into admission semantics
- identity presence checks into admission success
- local persisted token presence into real current authorization truth

This task is scaffolding, not final authorization.

---

## Expected outputs

This task should produce, at minimum:

1. Clear package-level scaffolding for:
   - node identity state
   - admission-token state
   - node-side local persisted-state inspection/validation
   - future coordinator-side token-validation attachment points

2. `transitloom-node` startup that can:
   - load config
   - inspect node identity-related local state expectations
   - inspect admission-token-related local state expectations
   - report clear placeholder readiness state
   - fail early on invalid local state/config combinations where appropriate

3. Tests for non-trivial node identity/admission scaffolding behavior

4. Clear separation in code between:
   - identity state
   - admission-token state
   - later control-session behavior

---

## Acceptance criteria

This task is complete when all of the following are true:

1. `transitloom-node` can clearly report node identity bootstrap/readiness state
2. `transitloom-node` can clearly report admission-token bootstrap/readiness state
3. identity-related local persisted-state handling is distinct from admission-token-related handling
4. invalid identity/admission local state combinations fail early and clearly where appropriate
5. code structure does not blur:
   - identity
   - admission
   - future control-session validation
6. the implementation stays aligned with:
   - `spec/v1-pki-admission.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
7. `go build ./...` succeeds
8. tests pass

---

## Files likely touched

Expected primary files:

- `internal/identity/...`
- `internal/admission/...`
- `internal/node/...`
- `cmd/transitloom-node/main.go`

Possibly:
- `internal/pki/...` if a small shared trust/identity helper is necessary
- `internal/config/...` if node config needs a minimal clarifying adjustment for identity/admission persisted-state references
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

---

## Suggested implementation approach

A good first approach is:

1. define node identity state types
2. define admission-token state types
3. define local file/path expectations for node identity material and cached/current admission-token material
4. add explicit state inspection/validation helpers
5. wire `transitloom-node` startup to:
   - load config
   - inspect local identity/admission state
   - report readiness clearly
   - fail clearly on invalid combinations
6. add focused tests

Keep this explicit and conservative.

Do **not** implement live auth flow yet.

---

## Completion notes

This task was completed with the following implementation shape:

- explicit `node_identity` and `admission` config sections pointing at persisted local state under `storage.data_dir`
- `internal/identity` bootstrap inspection that distinguishes:
  - bootstrap required
  - awaiting certificate issuance
  - ready
- `internal/admission` cached current admission-token inspection using local JSON metadata and distinguishing:
  - missing
  - usable
  - expired
- `internal/node` bootstrap aggregation so `transitloom-node` reports combined readiness without collapsing identity and admission into one concept
- early startup rejection for the incoherent local state where a cached current admission token exists but ready node identity material does not
- targeted package tests plus command-level startup verification

The task still intentionally stops short of:

- live enrollment
- certificate issuance
- token issuance or refresh
- coordinator-side live authorization
- control-session authentication

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- manual startup of `transitloom-node` with:
  - valid identity/admission placeholder state
  - missing identity material cases
  - missing token material cases
  - intentionally inconsistent state cases, if such cases are modeled

If tests are added, prefer:
- table-driven tests
- state/validation-focused tests
- no oversized test harness

---

## Risks to watch

### Risk 1: collapsing identity and admission
This is the most important risk in this task.

Do not let:
- identity material presence
- token material presence
- future participation success

become one blurred concept.

### Risk 2: pretending cached token state is authoritative truth
A local cached token is not the same thing as live coordinator-confirmed participation.

Be explicit about this in code and comments.

### Risk 3: widening into enrollment/control-session implementation
This task is scaffolding. It should prepare later work, not implement later work prematurely.

### Risk 4: burying persisted-state boundaries
Operators and future agents need clear reasoning about:
- static config
- local persisted identity state
- local cached token state
- future distributed truth

Do not hide these boundaries.

---

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0005 — minimal node-to-coordinator control session`

unless implementation reveals that a narrower enrollment/issuance-specific prerequisite should be split out first.

The important outcome is that the repository now has a clean, explicit node identity/admission foundation that future auth/control work can build on safely.

---
