# agents/tasks/T-0015-external-endpoint-advertisement-and-dnat-aware-reachability-basics.md

## Task ID

T-0015

## Title

External endpoint advertisement and DNAT-aware reachability basics

## Status

Completed

## Purpose

Implement the first explicit external-endpoint advertisement and DNAT-aware reachability model for Transitloom.

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

Its job is to establish the first explicit model for how Transitloom represents and uses externally reachable endpoints, especially in cases involving:

- dynamic public IPs
- SNAT-only nodes
- DNAT-configured inbound ports
- optional UPnP / PCP / NAT-PMP discovery
- targeted external reachability probing
- endpoint staleness and revalidation after unhealthy/down events

This task should create the first honest foundation for DNAT-aware direct-path reachability without pretending that the whole NAT traversal problem is solved.

---

## Why this task matters

Transitloom already supports or plans:

- direct raw UDP carriage
- WireGuard-over-mesh direct-path validation
- single relay hop
- scheduler/path use decisions

But real internet-facing deployments often involve asymmetric reachability such as:

- one node with dynamic public IP plus DNATed inbound ports
- another node with only internal address and outbound SNAT
- changing public IP after link churn
- changing usable inbound ports after router reconfiguration or dynamic mapping refresh

Without an explicit reachability model, later direct-path and relay decisions risk relying on vague or overloaded notions such as:

- “node address”
- “listen port”
- “service port”

That is not sufficient.

The goal here is not “implement full NAT traversal forever.”  
The goal is:

**make externally reachable endpoint knowledge explicit, DNAT-aware, freshness-aware, and usable by later path logic**

---

## Objective

Add the minimum useful implementation scaffolding for external endpoint advertisement and DNAT-aware reachability modeling.

The implementation should explicitly represent:

- configured external endpoints
- router-discovered external endpoints where available
- probe-verified external endpoints
- endpoint freshness / staleness
- the distinction between:
  - local target
  - local ingress
  - mesh/runtime port
  - external advertised endpoint

This task should not yet become a full NAT traversal system or a broad discovery engine.

---

## Scope

This task includes:

- defining explicit types for externally reachable endpoint knowledge
- defining source-of-truth/source-of-knowledge categories such as:
  - configured
  - router-discovered
  - probe-discovered
  - coordinator-observed public IP combined with configured forwarded ports
- defining endpoint verification/freshness state such as:
  - unverified
  - verified
  - stale
  - failed
- representing DNAT-aware external endpoint advertisement in a way that later direct-path logic can use
- supporting explicit config for forwarded external ports / published external endpoints
- optionally defining narrow integration points for:
  - UPnP / PCP / NAT-PMP discovery
  - targeted external probing
  - stale-after-unhealthy/down revalidation triggers
- adding focused tests for non-trivial endpoint modeling and validation behavior

This task may include:

- focused helpers under `internal/transport`
- focused helpers under `internal/node`
- focused helpers under `internal/controlplane`
- narrow config additions under `internal/config`
- small supporting helpers under `internal/service` or `internal/dataplane` if clearly justified
- small documentation/context updates under `agents/context/`

---

## Non-goals

This task does **not** include:

- full generic NAT traversal
- broad STUN/TURN/ICE-style machinery
- full blind port scanning as default behavior
- production-complete UPnP/PCP/NAT-PMP implementation
- arbitrary automatic public endpoint inference with no explicit model
- full path-candidate distribution redesign
- relay redesign
- scheduler redesign
- broad security policy redesign

Do not accidentally turn this into “solve internet NAT traversal in one task.”

---

## Design constraints

This task must preserve these architectural rules:

- local target is **not** the same thing as local ingress
- local ingress is **not** the same thing as external advertised endpoint
- service binding is **not** the same thing as public reachability
- node-local runtime state is distinct from distributed coordinator-visible state
- direct-path eligibility must not rely on overloaded address fields
- endpoint knowledge must be explicit about source and freshness
- stale endpoint information must not be treated as timeless truth

Especially important:

- do **not** assume a node can reliably auto-discover unknown DNAT rules from host-local observation alone
- do **not** assume observed public IP is enough to infer usable inbound ports
- do **not** make blind full-range probing the default behavior
- do **not** collapse “configured forwarded port” and “verified reachable port” into one undifferentiated field

The precedence model should remain:

1. explicit config
2. UPnP / PCP / NAT-PMP when available
3. targeted external probing
4. broad probing only as controlled fallback

And endpoint knowledge should become stale after unhealthy/down events until revalidated.

---

## Expected outputs

This task should produce, at minimum:

1. Explicit data structures for external endpoint advertisement and reachability state
2. Config or runtime modeling for DNAT-aware externally reachable endpoints
3. Explicit source/freshness/verification state for endpoint knowledge
4. Focused tests for non-trivial modeling and validation behavior
5. A safe foundation for later direct-path and relay/path-candidate refinement

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can explicitly represent externally reachable endpoints without overloading local target or local ingress concepts
2. endpoint knowledge explicitly records source-of-knowledge and freshness/verification state
3. explicit DNAT-aware configured endpoints are modeled cleanly
4. the model supports future use of:
   - configured forwarded ports
   - router-discovered mappings
   - targeted probe verification
   - stale-after-unhealthy/down revalidation
5. the implementation does not falsely claim full NAT traversal or complete automatic endpoint discovery
6. the implementation remains aligned with:
   - `spec/v1-data-plane.md`
   - `spec/v1-service-model.md`
   - `spec/v1-object-model.md`
   - `spec/implementation-plan-v1.md`
   - `spec/v1-architecture.md`
7. `go build ./...` succeeds
8. tests pass

---

## Files likely touched

Expected primary files:

- `internal/transport/...`
- `internal/node/...`
- `internal/controlplane/...`
- `internal/config/...`

Possibly:
- `internal/service/...`
- `internal/dataplane/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`
- `agents/context/...`

---

## Suggested implementation approach

A good first approach is:

1. define explicit external-endpoint and reachability-state types
2. define source-of-knowledge enums/fields
3. define verification/freshness state enums/fields
4. add minimal config support for explicit forwarded/public endpoint information
5. add narrow modeling hooks for future router-discovery and probe-verification inputs
6. define stale/revalidation semantics for unhealthy/down-triggered endpoint invalidation
7. add focused tests
8. update task/context/memory files as needed

Keep the model explicit and conservative.

Do **not** prematurely add:
- a giant NAT traversal framework
- hidden endpoint inference logic
- full scanning engines
- broad coordinator/state redesign
- speculative complexity that current runtime code cannot yet use

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - configured external endpoint data validates correctly
  - local target / local ingress / external endpoint distinctions remain intact
  - source-of-knowledge and verification-state semantics are preserved
  - stale/revalidation state transitions are represented clearly
  - the model is usable without claiming more automation than actually exists

If tests are added, prefer:
- focused table-driven tests where appropriate
- small integration-style tests only if they remain simple and reviewable

---

## Risks to watch

### Risk 1: overloading endpoint meaning
This is the biggest risk in this task.

Do not let one field pretend to mean:
- local target
- local ingress
- public endpoint
- verified direct path

at the same time.

### Risk 2: false certainty
Do not let guessed or stale endpoint data look authoritative.

### Risk 3: excessive probing ambition
Do not make blind broad probing the default path.

### Risk 4: premature NAT-traversal expansion
Do not let this task turn into a full NAT traversal or peer-discovery subsystem.

### Risk 5: weak freshness model
Do not ignore the fact that dynamic public IP and DNAT reachability can change after unhealthy/down events.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower path-candidate distribution refinement
- or a narrower external probing / endpoint-verification task
- or a later direct-path reachability improvement task

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has an explicit, DNAT-aware, freshness-aware external endpoint model that future direct-path and relay decisions can build on safely.

---
