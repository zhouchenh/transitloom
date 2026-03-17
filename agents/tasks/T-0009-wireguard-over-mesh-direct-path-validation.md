# agents/tasks/T-0009-wireguard-over-mesh-direct-path-validation.md

## Task ID

T-0009

## Title

WireGuard-over-mesh direct-path validation

## Status

Queued

## Purpose

Implement the first direct-path WireGuard-over-mesh validation for Transitloom.

This task comes after:

- T-0002 — config loading scaffolding
- T-0003 — root/coordinator bootstrap scaffolding
- T-0004 — node identity and admission-token scaffolding
- T-0005 — minimal node-to-coordinator control session
- T-0006 — service registration basics
- T-0007 — association basics
- T-0008 — direct raw UDP carriage basics

Its job is to prove the flagship v1 use case against the new direct raw UDP carriage path:

- standard WireGuard remains unchanged
- Transitloom provides the local ingress endpoints
- Transitloom delivers traffic to the real WireGuard local target
- the path is direct only
- the validation is real end-to-end, not just conceptual

This task should create the first practical proof that WireGuard can operate over Transitloom local ingress ports on a direct path.

---

## Why this task matters

Transitloom now already has:

- control-plane/bootstrap scaffolding
- service registration
- association basics
- direct raw UDP carriage primitives

But the project’s flagship value is not “raw UDP can move.”  
It is:

**standard WireGuard can use Transitloom as the underlying mesh transport**

Without this task, the project still lacks its first serious flagship validation.

The goal here is not “complete WireGuard support forever.”  
The goal is:

**prove one direct-path WireGuard-over-mesh slice cleanly and honestly**

---

## Objective

Add the minimum useful runtime integration and validation needed so that standard WireGuard can use Transitloom local ingress endpoints on a direct path.

The implementation should:

- wire the direct carriage primitives into node startup/runtime
- allocate or honor local ingress endpoints needed for peer-facing use
- preserve the distinction between:
  - WireGuard real local `ListenPort` / local target
  - Transitloom local ingress endpoint used as peer endpoint
- validate the end-to-end direct-path flow

This task should remain tightly scoped to direct-path validation.

---

## Scope

This task includes:

- wiring direct raw UDP carriage primitives into node runtime/startup
- supporting local ingress behavior needed for WireGuard-over-mesh direct-path use
- mapping a configured WireGuard-style UDP service onto:
  - local target = actual WireGuard listen endpoint
  - local ingress = Transitloom peer-facing local endpoint
- validating that standard WireGuard can point peers at Transitloom local ingress ports
- adding focused tests and/or reproducible validation steps for direct-path WireGuard-over-mesh

This task may include:

- focused helpers under `internal/node`
- focused helpers under `internal/dataplane`
- focused helpers under `internal/service`
- small config clarifications if strictly necessary
- small runtime/state reporting improvements
- small integration-style tests if they remain reviewable

---

## Non-goals

This task does **not** include:

- relay support
- multi-WAN scheduling
- weighted burst/flowlet-aware scheduling
- path scoring
- encrypted generic data-plane support
- generic TCP carriage
- broad WireGuard management automation
- rewriting or generating full WireGuard configs automatically
- full production runtime hardening
- service discovery expansion
- association lifecycle expansion

Do not accidentally turn this task into “make Transitloom production-ready for all WireGuard cases.”

---

## Design constraints

This task must preserve these architectural rules:

- WireGuard remains standard
- Transitloom remains generic in the core model
- WireGuard is the flagship use case, not a privileged core-only semantic
- local target is not the same thing as local ingress
- service binding is not the same thing as local ingress binding
- direct raw UDP carriage must remain association-bound
- zero in-band overhead must remain preserved
- this task is direct-path only

Especially important:

- WireGuard’s actual local `ListenPort` remains the service local target
- Transitloom local ingress ports are separate peer-facing endpoints
- do not collapse those two concepts “for convenience”
- do not add relay logic here
- do not let WireGuard-specific shortcuts damage the generic model

---

## Expected outputs

This task should produce, at minimum:

1. Node runtime wiring that makes direct raw UDP carriage usable for a WireGuard-style direct-path service
2. Clear local ingress handling suitable for WireGuard peer endpoint use
3. Clear delivery to the actual WireGuard local target
4. An end-to-end direct-path validation showing standard WireGuard can operate over Transitloom local ingress ports
5. Clear placeholder/reporting text about what is validated and what is still not implemented
6. Focused tests or repeatable validation steps

---

## Acceptance criteria

This task is complete when all of the following are true:

1. a node can expose a Transitloom local ingress endpoint for a direct-path associated service
2. a peer can send WireGuard UDP traffic to that local ingress endpoint
3. Transitloom can deliver that traffic to the correct WireGuard local target
4. the implementation preserves the distinction between:
   - local target
   - local ingress
   - service binding
   - association
5. the implementation does not claim relay, scheduler, or multi-WAN support
6. the implementation does not require WireGuard protocol changes
7. the code remains aligned with:
   - `spec/v1-wireguard-over-mesh.md`
   - `spec/v1-data-plane.md`
   - `spec/v1-service-model.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
8. `go build ./...` succeeds
9. tests pass, and/or a clear reproducible validation path is documented and exercised

---

## Files likely touched

Expected primary files:

- `internal/node/...`
- `internal/dataplane/...`
- `internal/service/...`
- `cmd/transitloom-node/main.go`

Possibly:
- `internal/config/...` if a small ingress-allocation/config clarification is necessary
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`

Potentially:
- a small validation note under `agents/context/` if the WireGuard direct-path validation flow becomes important enough to preserve

---

## Suggested implementation approach

A good first approach is:

1. identify the minimum node-runtime wiring needed to activate direct carriage for associated services
2. make local ingress behavior usable for WireGuard peer endpoint use
3. preserve explicit mapping:
   - local ingress -> Transitloom peer-facing UDP endpoint
   - local target -> actual WireGuard listen endpoint
4. validate direct-path traffic delivery end to end
5. add focused tests and/or a reproducible validation path
6. update task/context/memory files as needed

Keep the implementation narrow and explicit.

Do **not** prematurely add:
- automatic full WireGuard config generation
- relay support
- scheduler support
- transport abstraction expansion
- generic “application integration framework”

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- direct-path validation that shows:
  - a node local ingress receives peer-targeted UDP
  - Transitloom forwards it correctly
  - the correct WireGuard local target receives it
- if practical, a small end-to-end loopback/lab validation proving standard WireGuard can use the Transitloom ingress endpoint directly

If tests are added, prefer:
- focused unit/integration tests that stay small and reviewable
- minimal but real validation rather than large brittle harnesses

---

## Risks to watch

### Risk 1: collapsing local ingress and local target
This is the biggest risk in this task.

Do not let WireGuard convenience erase the architectural distinction.

### Risk 2: making WireGuard special in the core
Do not let the direct-path validation force WireGuard-specific semantics into the generic service model.

### Risk 3: overclaiming validation
A successful direct-path validation is not yet:
- relay support
- scheduler support
- multi-WAN support
- production hardening

### Risk 4: runtime wiring creep
Do not let this task expand into broad lifecycle or orchestration machinery.

---

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0010 — single relay hop basics`

unless implementation reveals that a narrower ingress-allocation/runtime-hardening prerequisite must be split out first.

The important outcome is that Transitloom now has a real flagship direct-path validation showing standard WireGuard can operate over Transitloom local ingress ports without violating the architecture.

---
