# agents/tasks/T-0030-live-node-probe-loop-lifecycle-integration-basics.md

## Task ID

T-0030

## Title

Live node probe-loop lifecycle integration basics

## Status

Completed

## Objective

Wire the bounded active probe loop into live node runtime lifecycle, ensure clean stop on cancellation/shutdown, and expose useful runtime probe-loop state for operator inspection.

## Outcome summary

- Integrated probe-loop startup into `transitloom-node` live runtime flow after scheduled egress activation.
- Bound probe-loop stop behavior to the runtime context cancellation path.
- Added explicit probe-loop runtime state tracking in node runtime with inspectable state transitions and last-round summary.
- Exposed probe-loop state through status surfaces used by the node status endpoint and `tlctl node status`.

## What changed

- `cmd/transitloom-node/main.go`
  - Starts probe loop during active runtime with bounded defaults.
  - Logs explicit started/not-started probe-loop lifecycle state.
- `internal/node/scheduled_egress.go`
  - Added probe-loop lifecycle ownership/state on `ScheduledEgressRuntime`.
  - Added `StartProbeLoop(...)` integration method with prerequisite gating.
  - Added probe-loop state transitions: `disabled`, `blocked`, `active`, `waiting-prerequisites`, `stopped`.
  - Added last-round status capture for operator visibility.
  - Included probe-loop summary in runtime snapshot output.
- `internal/node/probe_scheduler.go`
  - Calls `onRound` for zero-target rounds so waiting state is visible.
- `internal/status/summary.go`
  - Added `ProbeLoopSummary` and `ProbeLoopRoundSummary`.
  - Added probe-loop summary to `ScheduledEgressSummary`.
- `internal/status/report.go`
  - Prints probe-loop state, bounds, reason, and last-round counters/timestamp.

## Tests

- Added lifecycle/status-focused tests:
  - `internal/node/scheduled_egress_test.go`
    - blocked start without endpoint registry
    - waiting state and zero-target last-round reporting
  - `internal/node/probe_scheduler_test.go`
    - zero-target rounds still trigger `onRound`
  - `internal/status/summary_test.go`
    - probe-loop status lines and reason-line output
- Verification:
  - `go test ./...` passed
  - `go build ./...` passed

## Architecture/spec alignment

- Keeps probe scheduling bounded by interval and max-target limits.
- Preserves separation among endpoint freshness, measured path quality, and scheduler choice.
- Preserves separation between probe-loop lifecycle handling and fallback/stickiness/scheduler policy decisions.
- Keeps lifecycle ownership explicit and inspectable in node runtime.

## Files changed

- `cmd/transitloom-node/main.go`
- `internal/node/probe_scheduler.go`
- `internal/node/probe_scheduler_test.go`
- `internal/node/scheduled_egress.go`
- `internal/node/scheduled_egress_test.go`
- `internal/status/report.go`
- `internal/status/summary.go`
- `internal/status/summary_test.go`
- `agents/TASKS.md`
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
