# AGENTS.md

## Purpose

This repository is designed to be worked on by coding agents and humans together.

Transitloom is a coordinator-managed overlay mesh transport platform focused first on:

- high-performance raw UDP service carriage
- practical multi-WAN aggregation
- WireGuard-over-mesh as the flagship v1 use case

Agents working in this repository must optimize for **correctness, architectural consistency, maintainability, measurability, and end-to-end usefulness**, not just local code changes.

This file defines the required working model for agents.

---

## Read this first

Before making changes, read these files in order:

1. `agents/BOOTSTRAP.md`
2. `agents/IDENTITY.md`
3. `agents/SOUL.md`
4. `agents/CONTEXT.md`
5. `agents/MEMORY.md`
6. `agents/TASKS.md`
7. `agents/CODING.md`
8. `agents/REPORTING.md`

Then read any task files referenced from:

- `agents/tasks/`

If a task touches architecture or invariants, also read the relevant files under:

- `spec/`

Do not skip the reading order unless the task is trivial and clearly isolated.

`agents/README.md` is a useful overview file, especially for humans, but the files above are the operational minimum.

---

## Repository priorities

Transitloom v1 is currently centered on these priorities:

1. strong architecture and object-model consistency
2. correct trust/admission behavior
3. correct control-plane boundaries
4. practical raw UDP data-plane behavior
5. WireGuard-over-mesh as flagship validation path
6. multi-WAN aggregation performance
7. disciplined scope control

Do not optimize for secondary features at the expense of these priorities.

---

## Non-negotiable v1 invariants

Agents must preserve these invariants unless a task explicitly changes the architecture and the corresponding specs are updated.

### Data plane

- raw UDP is the primary v1 data-plane transport
- raw UDP v1 requires **zero in-band overhead**
- v1 raw UDP data plane allows:
  - direct public paths
  - direct intranet/private paths
  - single coordinator relay hop
  - single node relay hop
- v1 raw UDP data plane does **not** allow arbitrary multi-hop forwarding
- data-plane scheduling is **endpoint-owned**
- v1 default scheduler is **weighted burst/flowlet-aware**
- per-packet striping is allowed only when paths are **closely matched**

### Control plane

- control plane is more flexible than data plane
- QUIC + TLS 1.3 mTLS is the primary control transport
- TCP + TLS 1.3 mTLS is the fallback transport
- the application control protocol should remain logically consistent across QUIC and TCP

### Trust and admission

- node identity and current participation permission are separate
- a valid node certificate alone is **not** enough for normal participation
- normal participation requires:
  - valid node certificate
  - valid admission token
- revoke is **hard in operational effect**
- the root authority is **not** a normal node-facing coordinator target

### Service model

- Transitloom core remains **generic**
- WireGuard is the flagship documented use case, but not a privileged core-only concept
- services and associations are first-class objects
- local target and local ingress are different concepts and must not be collapsed

---

## How to think while working

Use these decision principles:

- prefer evidence over guessing
- prefer simpler practical solutions over elegant but fragile ones
- preserve architecture boundaries
- do not widen v1 scope casually
- do not introduce implementation shortcuts that contradict the specs
- do not optimize one subsystem in a way that breaks the flagship end-to-end use case
- if a path fails twice, stop repeating it with small variations and switch approach

When trade-offs appear, prefer:

- correctness over speed of patching
- maintainability over cleverness
- practical compatibility over abstraction purity
- end-to-end usefulness over local optimization

---

## Required workflow for agents

### Before changing code

1. Read the required agent context files.
2. Read the relevant spec files.
3. Identify the exact objective and acceptance criteria.
4. Check whether the task is:
   - architectural
   - implementation
   - cleanup/refactor
   - documentation
5. Avoid starting broad refactors unless required.

### During work

- make small, coherent changes
- keep package boundaries aligned with the object model
- avoid mixing unrelated changes in one patch
- update docs/specs if behavior or architecture meaning changes
- do not silently invent new terminology when existing terms already exist

### After making changes

Verify what you changed.

At minimum, where applicable:

- run builds
- run tests
- run lint/format tools if configured
- inspect for nearby breakage
- confirm the implementation still matches relevant specs

Do not claim success without verification.

Follow `agents/REPORTING.md` when the run ends.

---

## Documentation and spec discipline

Transitloom is currently spec-heavy by design.

Agents must treat the files under `spec/` as architectural guidance, not optional prose.

If code diverges from spec, do one of the following:

- update the code to match the spec, or
- update the spec deliberately if the design has truly changed

Do not leave silent contradictions.

If you introduce a meaningful architectural change, update the relevant files in `spec/` and, if helpful, the relevant files under `agents/`.

---

## Where to look for truth

Use these sources in roughly this order:

### Stable architectural truth
- `spec/`
- `agents/IDENTITY.md`
- `agents/SOUL.md`
- `agents/MEMORY.md`

### Current working truth
- `agents/CONTEXT.md`
- `agents/TASKS.md`
- `agents/tasks/`

### Coding and reporting standards
- `agents/CODING.md`
- `agents/REPORTING.md`

### Supporting human-facing explanation
- `docs/`

Do not rely on `README.md` alone for implementation decisions.

---

## Expected repository structure awareness

Agents should expect this repository to grow around these major areas:

- `cmd/` for binaries
- `internal/` for implementation packages
- `spec/` for engineering specifications
- `docs/` for human-facing documentation
- `agents/` for agent operating context and task continuity

Do not create new top-level structure casually.

---

## Package-boundary expectations

Keep concepts separate unless there is a strong reason to merge them.

In particular, avoid collapsing these concepts too early:

- identity vs admission
- service vs service binding
- local target vs local ingress
- relay candidate vs path candidate
- config vs distributed state
- authoritative registry/catalog objects vs hints
- control-plane legality vs data-plane forwarding logic

If in doubt, preserve separation.

---

## Scope control rules

Transitloom v1 is intentionally constrained.

Agents must not casually introduce:

- arbitrary multi-hop raw UDP forwarding
- generic encrypted data plane as if it already exists
- generic TCP data plane as if it already exists
- uncontrolled routing flexibility beyond v1 boundaries
- deep WireGuard-specific core semantics that break the generic model

If a task appears to require such changes, stop and record the conflict clearly instead of silently widening scope.

---

## Task handling rules

### Use the task system

Active work should be reflected in:

- `agents/TASKS.md`
- `agents/tasks/*.md` where appropriate

If you complete a meaningful task, update task state.

If you discover an important blocker, record it clearly.

### When blocked

Do not give vague excuses.

State clearly:

- what you verified
- what you ruled out
- the current best explanation
- the exact blocker
- the smallest useful next step

---

## Memory and continuity rules

Use the `agents/` workspace to preserve continuity for future agents.

### Update `agents/CONTEXT.md` when:
- current phase changes
- immediate priorities change
- important implementation status changes

### Update `agents/MEMORY.md` when:
- a decision should remain durable across tasks
- a naming or architectural rule is settled
- a rejected approach should stay rejected

### Update `agents/TASKS.md` when:
- tasks start
- tasks complete
- priorities reorder
- blockers appear

### Use `agents/logs/` for:
- work logs
- handoff notes
- investigation notes
- partial progress worth preserving

---

## Critical requirement: maintain agent memory and context

Coding agents working in this repository are context-limited. They must assume that anything not written down clearly in the `agents/` workspace may be lost between sessions, forgotten during long tasks, or unavailable to future agents.

Because of that, maintaining the `agents/` workspace is a core part of the work, not optional documentation.

### Required mindset

Treat these files as persistent operational memory:

- `agents/CONTEXT.md`
- `agents/context/*.md`
- `agents/MEMORY.md`
- `agents/memory/*.md`
- `agents/TASKS.md`
- `agents/tasks/*.md`
- `agents/logs/`

Agents must update them whenever necessary, appropriate, or when meaningful progress is made.

### Especially important

Agents should be especially careful to update:

- `agents/CONTEXT.md`
- `agents/context/*.md`

when:
- implementation progress is made
- current priorities change
- design understanding becomes sharper
- a blocker is found
- a workaround is chosen
- an assumption is verified or disproven
- a new implementation constraint is discovered
- an important repo/runtime fact is learned

### Memory is not optional

Agents should treat `agents/MEMORY.md` and `agents/memory/*.md` as important places to preserve durable knowledge such as:

- settled design decisions
- naming conventions
- rejected approaches
- invariants that must not drift
- lessons learned that would be costly to rediscover

### Minimum rule

If a future agent would benefit from knowing it, and it is not already clearly captured, write it down.

### Failure mode to avoid

Do not assume:
- “I will remember this later”
- “the next agent will infer this from code”
- “this is too small to record”

Small unrecorded facts often become repeated confusion, wasted work, or architectural drift.

### Practical rule

When finishing a meaningful task or reaching a meaningful checkpoint, ask:

1. What changed?
2. What was learned?
3. What should persist for future agents?
4. Which `agents/` files should be updated right now?

Then update them before considering the work complete.

---

## Coding and reporting requirements

Agents must follow:

- `agents/CODING.md` for coding practices
- `agents/REPORTING.md` for end-of-run reporting

These are part of the required workflow, not optional references.

In particular:

- write tests for non-trivial behavior
- run verification before claiming success
- add benchmarks when performance-relevant code is introduced and measurement is useful
- write enough comments to preserve non-obvious intent
- report clearly what changed, what was verified, what is incomplete, and what the next step is

---

## Coding style expectations

Until a more detailed coding standard exists, and in addition to `agents/CODING.md`:

- keep code straightforward and readable
- favor explicitness over hidden magic
- keep functions and types aligned with the object model
- avoid premature framework-building
- avoid unnecessary abstraction layers
- make package APIs small and purposeful
- prefer stable naming over clever naming

---

## Verification expectations

For implementation tasks, verify changes as locally as possible.

Examples:
- `go build ./...`
- targeted tests
- config validation runs
- command startup checks
- basic functional checks for changed paths

If verification cannot be completed, say exactly why.

---

## Commit and push policy

Transitloom currently uses a staged repository workflow policy.

### Before v1.0.0

Before the first stable `v1.0.0` release, agents should commit directly and push directly to `master` by default when a task run is completed and all of the following are true:

- the change is coherent and task-aligned
- the work has been verified appropriately
- the end-of-run report is complete
- relevant `agents/` files were updated
- the commit message accurately describes the change
- the repo is not being left in a confusing or partially broken state unless the checkpoint is intentional and clearly documented

Before `v1.0.0`, commit-and-push is the normal expected end state for a completed, verified, well-reported task run unless the task explicitly says otherwise or a real blocker prevents it.

### At and after v1.0.0

Starting at `v1.0.0`, agents must switch to a branch-based workflow:

- work on task/feature branches
- push branches, not direct pushes to `master`
- merge through review workflow

Agents must not assume the pre-`v1.0.0` direct-push policy still applies after that milestone.

### What agents must not do

Even before `v1.0.0`, agents must not commit or push when:

- the change is unverified
- unrelated changes are mixed together
- the task is mid-flight and the checkpoint would confuse the next human/agent
- there is unresolved architecture drift
- the repo state contradicts the specs in a way that has not been intentionally documented

---

## Commit discipline

Prefer commits that are:

- small
- coherent
- easy to review
- aligned to one task or one logical change set

Avoid mixing:
- refactors
- architecture changes
- formatting-only changes
- unrelated docs updates

in one commit unless that combination is truly necessary.

---

## What agents should optimize for

The correct optimization target for this repository is:

**a working, coherent, maintainable Transitloom v1 that proves the flagship WireGuard-over-mesh and multi-WAN raw UDP aggregation use case without violating the architecture.**

Do not optimize for looking busy.  
Do not optimize for maximal code output.  
Do not optimize for speculative future features before the current layer is real.

Build the right thing in the right order.

---
