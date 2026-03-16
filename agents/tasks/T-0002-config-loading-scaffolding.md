# agents/tasks/T-0002-config-loading-scaffolding.md

## Task ID

T-0002

## Title

Config loading scaffolding

## Status

Queued

## Purpose

Implement the first real code slice for Transitloom by building object-model-aligned config loading and validation scaffolding for the root authority, coordinator, and node roles.

This is intended to be the first implementation task after the agent workspace baseline is complete.

---

## Why this task matters

Transitloom already has:

- an architecture baseline
- a control-plane spec
- a data-plane spec
- a service model
- a PKI/admission model
- an object model
- a config specification
- an implementation plan
- an initial code skeleton

The cleanest next implementation step is not networking yet.

It is config loading and validation, because that:

- forces the first concrete implementation choices
- respects the object model
- gives every command a real startup path
- avoids premature transport complexity
- prepares root/coordinator/node bootstrap work
- provides a stable place for future PKI/control/data code to attach

---

## Objective

Add initial configuration types, config loading, and validation scaffolding for:

- `transitloom-root`
- `transitloom-coordinator`
- `transitloom-node`

while keeping the implementation aligned with:
- `spec/v1-config.md`
- `spec/v1-object-model.md`
- `spec/implementation-plan-v1.md`

---

## Scope

This task includes:

- choosing an initial config file format for implementation scaffolding
- implementing package structure under `internal/config`
- defining role-specific config types
- implementing config loading from file
- implementing validation with useful error messages
- wiring config load/startup path into:
  - `cmd/transitloom-root`
  - `cmd/transitloom-coordinator`
  - `cmd/transitloom-node`
- adding minimal status/logging output around startup and validation

---

## Non-goals

This task does **not** include:

- real PKI issuance logic
- real admission-token validation logic
- control-plane transport
- service registration
- association creation
- data-plane forwarding
- full distributed policy logic
- advanced hot-reload behavior
- final config syntax lock for all future features

---

## Design constraints

This task must preserve these architectural rules:

- config is **not** the same thing as distributed state
- config must not try to encode all coordinator-managed truth locally
- local target and local ingress are different concepts
- service model remains generic
- WireGuard must not become a privileged core-only special case
- the config model should support root/coordinator/node roles separately
- configuration should align with persisted runtime state instead of inlining every secret directly in the main config by default

---

## Expected outputs

This task should produce:

1. `internal/config` package with:
   - root config types
   - coordinator config types
   - node config types
   - service declaration config types
   - validation functions
   - config loading entrypoints

2. command startup wiring so that:
   - each main command can accept a config path
   - config can be loaded
   - config can be validated
   - startup either proceeds to placeholder runtime or exits with useful errors

3. initial examples or sample config files if useful, though that can be separate if needed

---

## Acceptance criteria

This task is complete when all of the following are true:

1. `transitloom-root` can:
   - accept a config path
   - load config
   - validate config
   - report clear validation errors or a clear startup placeholder

2. `transitloom-coordinator` can:
   - accept a config path
   - load config
   - validate config
   - report clear validation errors or a clear startup placeholder

3. `transitloom-node` can:
   - accept a config path
   - load config
   - validate config
   - report clear validation errors or a clear startup placeholder

4. role-specific config types reflect the conceptual model from `spec/v1-config.md`

5. the code structure does not collapse:
   - service vs service binding
   - local target vs local ingress
   - config vs distributed truth

6. `go build ./...` succeeds

---

## Files likely touched

Expected primary files:

- `internal/config/...`
- `cmd/transitloom-root/main.go`
- `cmd/transitloom-coordinator/main.go`
- `cmd/transitloom-node/main.go`

Potential supporting files:
- `internal/objectmodel/...` if shared identifiers/types are needed
- `internal/status/...`
- sample config files or fixtures if introduced

---

## Suggested implementation approach

A good first approach is:

1. choose a simple structured config format for now
2. define role-specific config structs
3. add a loader function per role or a role-aware loader entrypoint
4. add validation functions
5. wire command-line startup to:
   - parse config path
   - load config
   - validate config
   - print or log a success placeholder
6. verify build and startup behavior

Keep it simple and explicit.

Do **not** build a giant dynamic config framework first.

---

## Verification

Minimum verification should include:

- `go build ./...`
- startup with a valid example config for each role
- startup with intentionally invalid config to confirm useful error handling

If test scaffolding is introduced, targeted config validation tests are good, but are not required if they would slow the first slice too much.

---

## Risks to watch

### Risk 1: over-engineering the config system
Avoid building a giant future-proof abstraction layer before real usage exists.

### Risk 2: encoding distributed truth into local config
Avoid making local config the source of truth for admission, associations, or distributed state.

### Risk 3: collapsing object distinctions
Avoid shortcutting the object model just because the first config slice is small.

### Risk 4: making WireGuard special in the config core
The config model must support WireGuard well without turning the entire configuration model into a WireGuard-specific schema.

---

## Completion handoff

When this task is complete, the next likely task should be:

- `T-0003 — root/coordinator bootstrap scaffolding`

unless implementation reveals that a narrower prerequisite task should be split out.

---
