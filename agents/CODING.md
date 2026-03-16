# agents/CODING.md

## Purpose

This file defines coding expectations for Transitloom.

It exists so coding agents and humans do not need to restate the same implementation standards in every task prompt. It should be treated as a practical coding baseline for v1 work unless a task explicitly says otherwise.

This file is not a replacement for:

- `AGENTS.md`
- the files under `agents/`
- the specs under `spec/`

It is the coding discipline layer that sits on top of them.

---

## Primary objective

Write code that helps Transitloom become:

- correct
- reviewable
- maintainable
- measurable
- architecturally consistent
- useful for the next real implementation step

Do not optimize for code volume.  
Do not optimize for cleverness.  
Do not optimize for speculative future flexibility at the cost of present correctness and clarity.

---

## Architectural obedience

Before writing code, align with the existing architecture.

You must preserve the core boundaries already chosen unless the task explicitly requires a design change and the related specs are updated.

Especially protect these distinctions:

- identity vs admission
- service vs service binding
- local target vs local ingress
- relay candidate vs path candidate
- config vs distributed state
- authoritative objects vs discovery hints
- control-plane legality vs data-plane forwarding behavior

Do not collapse these distinctions just because it makes the immediate code shorter.

---

## Scope discipline

Implement only what the current task requires.

Avoid adding unrelated “helpful” changes such as:

- broad refactors
- new framework layers
- speculative plugin systems
- future transport support that is not yet in scope
- deep abstraction for code that is not yet proven necessary

If a task appears to require broader architectural change, stop and record the conflict clearly rather than silently widening scope.

---

## Code style principles

### Prefer explicitness
- prefer explicit data structures
- prefer explicit validation
- prefer explicit control flow
- prefer explicit package boundaries
- prefer explicit naming that matches the specs

### Prefer simplicity
- keep functions small and purposeful
- keep types understandable
- keep package APIs narrow
- choose the simplest practical design that preserves the important requirements

### Avoid hidden behavior
- do not hide important control flow in surprising abstractions
- do not make defaults do too much silently
- do not bury architectural decisions in utility code

### Match the vocabulary
Use terminology already established in:
- `spec/`
- `docs/glossary.md`
- `agents/MEMORY.md`

Do not invent new names for existing concepts unless there is a strong reason and the docs/specs are updated accordingly.

---

## Comments

Write comments when they preserve meaning that would otherwise be easy to lose.

Good comments explain:

- why the code exists
- why a design choice was made
- why a boundary matters
- why a validation rule is strict
- why something intentionally does **not** do more
- non-obvious assumptions or invariants

Avoid comments that only restate obvious syntax.

### Good examples of comment targets
- role separation
- object-model boundaries
- non-obvious validation constraints
- security-sensitive behavior
- scheduler behavior or limitations
- reasons for intentionally constrained v1 behavior

### Bad examples of comment targets
- line-by-line narration of obvious code
- comments that merely rename the variable in prose
- noisy comments that add no durable understanding

The standard is:

**write enough detail that a future agent or human can understand the intent without re-deriving it from scratch.**

---

## Error handling

Error handling must be useful and specific.

### Rules
- fail early on invalid inputs or invalid startup state
- return specific errors with enough context
- include the relevant role, object, field, or path when possible
- do not hide invalid state behind vague fallback behavior
- do not silently continue on broken assumptions unless the behavior is clearly intentional and safe

### Avoid
- generic “invalid config” with no field context
- swallowing important startup errors
- silently defaulting security-sensitive behavior
- imprecise error messages that force unnecessary debugging

---

## Logging and status

Transitloom should be operable and debuggable as it grows.

### Rules
- log meaningful startup state
- log validation failures clearly
- log role-relevant status in a structured and reviewable way
- prefer predictable, concise status output over noisy logs

### Avoid
- excessive debug noise by default
- logging secrets
- burying the important error in a wall of text
- mixing human-facing startup messages with unstable internal dump formats unless useful

---

## Testing

Testing is important for necessary code.

### Minimum rule
If code introduces non-trivial behavior, write tests for it.

### Especially test
- config loading
- validation
- parsing
- object mapping
- state transitions
- boundary conditions
- obvious invalid cases
- logic that is easy to regress silently

### Expectations
- run the tests you add
- do not claim success without running verification
- test both valid and invalid cases where relevant
- prefer table-driven tests when they improve coverage and clarity
- keep tests readable and targeted

### Avoid
- giant brittle integration tests as the first step
- meaningless tests that assert implementation trivia instead of behavior
- adding complex test harnesses before simpler direct tests are sufficient

---

## Benchmarks

Benchmarks are important when performance-relevant code is introduced.

Transitloom is a performance-sensitive project, especially in the data plane, so measurement matters.

### When to add benchmarks
Add benchmarks when code affects:
- parsing or validation hot paths that may matter operationally
- scheduler logic
- path scoring logic
- packet-handling logic
- data-plane state lookup
- relay/path selection logic
- any code likely to be called frequently enough that performance matters

### When not to add benchmarks
Do not add benchmarks as ritual noise for code where performance is obviously not yet meaningful.

### Benchmark rule
If you add a benchmark:
- make it realistic enough to be informative
- run it
- record the result in your task output or work log
- use it to support an improvement or establish a baseline

### Avoid
- artificial microbenchmarks with no clear value
- benchmarks that measure setup noise more than the target logic
- performance claims without numbers

---

## Verification

After making a change, verify it.

At minimum, where applicable:

- run `go build ./...`
- run relevant tests
- run relevant benchmarks if added
- check startup behavior if you changed command startup paths
- inspect whether nearby or structurally similar behavior may also be affected

Do not claim completion without verification.

If verification cannot be completed, say exactly:
- what was verified
- what was not verified
- why it could not be verified
- what the smallest useful next verification step is

---

## Package design

Keep package boundaries aligned with the object model and architecture.

### Prefer
- packages with clear responsibilities
- small public surfaces
- types placed where their meaning is most stable
- explicit conversion/mapping logic when crossing layers

### Avoid
- “misc” utility dumping grounds
- overly generic helper packages too early
- circular dependencies
- embedding architectural meaning in random helper functions
- letting one package quietly become the owner of too many concepts

### Strong caution
Do not use convenience as a reason to merge:
- config and runtime state
- identity and admission
- service and transport
- control plane and data plane

---

## Configuration code

Configuration code should be strict, explicit, and unsurprising.

### Rules
- treat config as local intent, not distributed truth
- validate role-specific constraints clearly
- distinguish static config from persisted runtime state
- distinguish local service declaration from distributed association/policy state
- make invalid config fail fast

### Avoid
- silent acceptance of malformed config
- magical field inference when it reduces clarity
- mixing future speculative config into the first working implementation without a real need

---

## Security-sensitive code

Code touching trust, admission, certificates, tokens, or revoke behavior must be treated carefully.

### Rules
- keep identity and participation permission separate
- do not let valid certs imply valid current participation
- make validation logic explicit
- prefer clarity over convenience
- document non-obvious security-relevant behavior

### Avoid
- vague auth/admission shortcuts
- hidden fallback logic
- “temporary” shortcuts that weaken revoke or trust boundaries
- comments or code that blur the difference between identity and authorization

---

## Performance-sensitive code

Transitloom’s flagship use case depends on practical raw UDP performance and multi-WAN aggregation value.

### Rules
- do not over-optimize too early
- do not ignore performance structure in clearly hot code
- keep room for later optimization by preserving clean boundaries now
- measure before making strong performance claims

### Avoid
- premature micro-optimization in non-hot code
- architecture damage in the name of speed
- unmeasured performance assertions
- hidden algorithmic cost in code likely to sit on the hot path later

---

## Progress reporting

When completing a task or meaningful checkpoint, report:

1. what changed
2. what you verified
3. what tests were added and run
4. what benchmarks were added and run, if any
5. what blockers or tensions were discovered
6. which `agents/` files were updated

If a task was partially completed, say so plainly.

Do not present partial progress as full completion.

---

## Updating agent memory and context

The `agents/` workspace is part of the working system.

If you make meaningful progress, learn something durable, or discover a recurring pitfall, update the appropriate files:

- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`
- `agents/tasks/*.md`
- `agents/context/*.md`
- `agents/memory/*.md`
- `agents/logs/`

### Minimum rule
If a future agent would benefit from knowing it, and it is not already captured clearly, write it down.

---

## Commit discipline

Prefer commits that are:

- small
- coherent
- easy to review
- limited to one logical change set

Do not mix:
- unrelated refactors
- architecture changes
- formatting-only noise
- unrelated docs changes

unless there is a strong reason.

Commit messages should accurately describe the change.

---

## When to stop and escalate

Stop and clearly report instead of pushing through if:

- the task conflicts with the specs
- the task appears to require widening v1 scope
- the architecture no longer seems internally consistent
- a missing decision materially affects correctness
- the same failed path has been tried twice with no new information
- required credentials/access are missing
- external blockers prevent safe progress

When blocked, report:
- what you verified
- what you ruled out
- the current best explanation
- the exact blocker
- the smallest useful next step

---

## One-sentence coding standard

Write code that makes Transitloom more correct, more measurable, more maintainable, and easier to extend in the right direction — without violating the architecture or pretending unfinished features already exist.

---
