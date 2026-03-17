# agents/tasks/T-0017-targeted-external-endpoint-probing-and-freshness-revalidation-basics.md

## Task ID

T-0017

## Title

Targeted external endpoint probing and freshness revalidation basics

## Status

Queued

## Purpose

Implement the first targeted external endpoint probing and freshness revalidation baseline for Transitloom.

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

Its job is to make Transitloom’s external endpoint model operationally useful by adding:

- targeted verification of candidate external endpoints
- explicit freshness / staleness handling
- revalidation behavior after unhealthy/down events
- narrow coordinator-assisted probing support

This task should create a practical first verification loop for externally reachable endpoints without turning Transitloom into a full NAT-traversal framework or a broad internet scanning engine.

---

## Why this task matters

After T-0015, Transitloom can explicitly model:

- configured external endpoints
- router-discovered endpoints
- probe-discovered endpoints
- source-of-knowledge and freshness state

But a model alone is not enough.

In real deployments, externally reachable endpoint knowledge can become wrong because of:

- dynamic public IP changes
- router reconfiguration
- changed DNAT rules
- expired or unstable UPnP / PCP / NAT-PMP mappings
- link flap / unhealthy path events
- stale assumptions after previously valid direct reachability

Without targeted probing and freshness revalidation:
- direct-path decisions may trust stale endpoint data too long
- operator confidence in direct reachability will be misplaced
- later path-candidate logic will rest on weak verification

The goal here is not “solve NAT traversal forever.”  
The goal is:

**make external endpoint knowledge verifiable, freshness-aware, and operationally safer**

---

## Objective

Add the minimum useful implementation scaffolding for targeted external endpoint probing and freshness revalidation.

The implementation should:

- verify candidate endpoints in a narrow, explicit way
- avoid blind broad probing by default
- update endpoint verification/freshness state based on probe results
- support stale-after-unhealthy/down semantics
- prepare later direct-path and reachability improvements

This task should not become a general internet scanning platform or a broad NAT traversal engine.

---

## Scope

This task includes:

- defining a targeted external probing model for candidate endpoints
- implementing probing of known candidate endpoints only, such as:
  - explicitly configured external endpoints
  - router-discovered mappings
  - previously known successful endpoints
  - coordinator-observed-IP + configured-forwarded-port combinations
- updating endpoint verification/freshness state based on probe outcomes
- supporting state transitions such as:
  - unverified
  - verified
  - stale
  - failed
- defining or implementing revalidation triggers after unhealthy/down events
- adding narrow coordinator-assisted probing support where useful
- adding clear reporting of probe results and freshness state
- adding focused tests for non-trivial probing / freshness behavior

This task may include:

- focused helpers under `internal/transport`
- focused helpers under `internal/node`
- focused helpers under `internal/controlplane`
- focused helpers under `internal/status`
- narrow config additions if strictly necessary
- small CLI/reporting hooks if useful for validation
- small fixture-driven tests

---

## Non-goals

This task does **not** include:

- blind full-range probing as the default behavior
- uncontrolled scanning of 0–65535 by default
- full STUN/TURN/ICE-style NAT traversal
- production-complete UPnP / PCP / NAT-PMP implementation
- broad path-candidate redesign
- scheduler redesign
- relay redesign
- generic peer-discovery machinery
- a broad security / abuse-prevention framework beyond what is needed to keep probing bounded and explicit

Do not accidentally turn this into “build a NAT traversal and scanning subsystem.”

---

## Design constraints

This task must preserve these architectural rules:

- local target is **not** the same as local ingress
- local ingress is **not** the same as external advertised endpoint
- configured endpoint data is **not** the same as verified reachability
- stale endpoint information must not be treated as timeless truth
- probing should be **targeted first**
- broad probing, if ever supported, should remain controlled fallback rather than default behavior
- endpoint verification state must be explicit and observable

Especially important:

- do **not** make blind broad probing the normal path
- do **not** assume observed public IP implies known reachable inbound ports
- do **not** assume host-local observation alone can reliably infer unknown DNAT rules
- do **not** collapse configured, discovered, verified, and stale endpoints into one field or status

The intended precedence remains:

1. explicit config
2. UPnP / PCP / NAT-PMP
3. targeted external probing
4. broad probing only as controlled fallback

And endpoint knowledge should become stale after unhealthy/down events until revalidated.

---

## Expected outputs

This task should produce, at minimum:

1. A targeted probing model for candidate external endpoints
2. Explicit freshness / verification state transitions driven by probe results
3. Explicit stale/revalidation handling after unhealthy/down events
4. Clear reporting of endpoint verification outcomes
5. Focused tests for non-trivial probing and freshness behavior
6. A safer operational foundation for future direct-path reachability use

---

## Acceptance criteria

This task is complete when all of the following are true:

1. Transitloom can probe targeted candidate external endpoints without defaulting to blind broad scans
2. probe results update endpoint verification/freshness state explicitly
3. stale-after-unhealthy/down behavior is represented and usable
4. the implementation keeps distinct:
   - configured endpoint
   - discovered endpoint
   - verified reachable endpoint
   - stale endpoint
   - failed endpoint
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
- `internal/status/...`

Possibly:
- `internal/config/...`
- `cmd/tlctl/...`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`
- `agents/context/...`

---

## Suggested implementation approach

A good first approach is:

1. define a narrow targeted probe input model
2. define probe result and freshness transition logic explicitly
3. wire probe results into endpoint verification state updates
4. add explicit stale/revalidation handling for unhealthy/down-triggered invalidation
5. add narrow coordinator-assisted probe support if needed
6. add focused reporting and tests
7. update task/context/memory files as needed

Keep the probing model narrow and explicit.

Do **not** prematurely add:
- broad internet scanning defaults
- giant NAT traversal frameworks
- hidden endpoint discovery magic
- large coordinator/state redesigns
- speculative complexity beyond current runtime needs

---

## Verification

Minimum verification should include:

- `go test ./...`
- `go build ./...`
- focused checks that:
  - targeted candidate probing works as intended
  - configured/discovered endpoints can become verified or failed explicitly
  - unhealthy/down-triggered staleness is represented clearly
  - stale endpoints require revalidation before reuse
  - the implementation does not default to blind broad scanning

If tests are added, prefer:
- focused table-driven tests where appropriate
- small fixture-driven or mock-driven tests for probing outcomes and state transitions

---

## Risks to watch

### Risk 1: broad-scan creep
This is the biggest risk in this task.

Do not let targeted probing quietly turn into default blind scanning behavior.

### Risk 2: false certainty
Do not let probe history or old success imply permanent truth.

### Risk 3: overloaded endpoint status
Do not merge configured, discovered, verified, stale, and failed states into vague status.

### Risk 4: too much NAT-traversal ambition
Do not let this task become a full automatic NAT traversal system.

### Risk 5: weak unhealthy/down semantics
Do not ignore the requirement that endpoint knowledge can become stale after link/path instability or public-IP changes.

---

## Completion handoff

When this task is complete, the next likely task should be:

- a narrower path-candidate distribution refinement
- or a narrower direct-path reachability optimization task
- or a later controlled broad-probing fallback task if truly justified

unless implementation reveals that a smaller prerequisite should be split out first.

The important outcome is that Transitloom now has a targeted, freshness-aware endpoint verification baseline that future direct-path and reachability decisions can build on safely.

---
