# v1 Stable-Link Segmented Aggregation Mode

## Status

Draft

## Purpose

Define a new optional Transitloom dataplane mode for **stable multi-WAN bandwidth aggregation**.

This mode is distinct from Transitloom’s existing whole-packet path selection model.

Its goal is to allow a single original payload packet to be carried as multiple Transitloom transport segments across multiple stable WAN paths, then reassembled into the original payload packet at the receiver.

This mode is intended for environments where:

- multiple WAN paths are available
- those paths are sufficiently stable
- the operator explicitly opts in
- the additional complexity and overhead are acceptable in exchange for stronger bandwidth aggregation

This mode is **not** intended to replace the existing conservative whole-packet mode.

---

## Why this mode exists

Transitloom’s existing whole-packet mode is well suited for:

- variable WAN quality
- unstable path conditions
- low transformation
- simpler failure handling
- preserving whole-packet carriage behavior

But whole-packet path selection has an inherent limit:

- one original packet is sent on one chosen path at a time

That works well for robustness, but it limits true aggregation across multiple stable WANs.

This mode exists to support a different operating point:

- use multiple WANs simultaneously for one original packet
- exploit stable WAN sets for stronger aggregate throughput
- accept transport-level segmentation, reassembly, and observability complexity as the price

---

## Relationship to existing mode

Transitloom should support at least two distinct dataplane modes:

### Mode A — conservative whole-packet mode

Properties:
- existing/default mode
- one original packet chooses one path
- optimized for variable paths
- lower transformation and lower complexity
- better suited for unstable environments

### Mode B — stable-link segmented aggregation mode

Properties:
- explicit opt-in advanced mode
- one original payload packet may be segmented into multiple Transitloom transport segments
- multiple WANs can be used simultaneously for one original packet
- optimized for measured-stable multi-WAN environments
- higher transport overhead and higher receiver complexity
- does **not** preserve “raw unchanged UDP carriage” semantics

This document defines **Mode B**.

---

## Non-goals

This mode does **not** aim to provide:

- default behavior for all deployments
- blind automatic activation on every multi-WAN node
- broad internet scanning or path discovery logic
- full dynamic-routing semantics
- arbitrary multi-hop overlay segmentation
- guaranteed delivery or reliability
- TCP-like retransmission behavior in v1
- a replacement for all existing scheduler/fallback/hysteresis logic
- a full FEC framework in the first minimal implementation

---

## Architectural boundaries

This mode must preserve these boundaries:

- candidate existence is not the same as candidate usability
- candidate usability is not the same as chosen runtime path
- endpoint freshness is not the same as path-quality measurement
- path-quality measurement is not the same as segmentation scheduling
- segmentation scheduling is not the same as higher-level path policy
- reassembly pressure handling is not the same as mode-switch policy
- this mode is distinct from the conservative whole-packet mode

Especially important:

- this mode is an **additional transport mode**
- this mode explicitly adds Transitloom transport metadata
- this mode should not be described as “raw unchanged UDP carriage”
- this mode should not silently become the default

---

## Applicability and eligibility

This mode should only be used when all of the following are true:

1. the operator has explicitly enabled it
2. the path set is measured stable enough
3. the selected WAN set is eligible for stable-link segmented aggregation
4. the original packet is large enough to justify segmentation
5. the packet is not classified as a control/small packet that should remain whole

### Operator enablement

The preferred activation model is:

- Transitloom may recommend this mode based on measured path stability
- the operator must still explicitly opt in
- the mode may be configured per-service or per-association
- association-specific settings may override service-level settings

### Stable-path eligibility

A WAN set is eligible only when path variation is sufficiently bounded.

The precise measured thresholds may evolve, but the model should consider at least:

- RTT spread
- jitter spread
- loss behavior
- freshness of measurements
- enough evidence to avoid weak conclusions from sparse data

If the path set is not stable enough, this mode should not activate.

---

## Packet eligibility

The input unit is the **original payload packet**.

This mode is packet-based, not stream-based.

### Eligible packets

A packet may be segmented only if:
- the mode is enabled
- the selected path set is eligible
- the packet is above a configured segmentation threshold
- the packet is not in a class that should remain whole

### Non-segmented packets

Some packets should remain whole even when the mode is enabled.

At minimum, the mode should support keeping these whole:
- small packets
- control-sensitive packets
- other operator-selected traffic classes

This is especially important for flagship use cases such as:
- WireGuard over Transitloom

---

## Sender-side model

The sender-side pipeline is:

1. choose or confirm the eligible WAN set using existing path-selection logic
2. classify the packet as segmented-eligible or whole-packet-only
3. determine effective segment sizing from current path MTU constraints
4. segment the original payload packet into Transitloom transport segments
5. assign segments across WANs using weighted scheduling
6. optionally add FEC/parity segments if enabled
7. transmit segments across the chosen WAN set
8. expose operator-visible runtime statistics

### Important note

The existing scheduler should still determine the **eligible WAN/path set**.

This mode should then apply its own **local segment scheduler** within that eligible set.

This is a hybrid model:

- existing scheduler chooses the allowed stable set
- segmented mode decides how to distribute segments inside that set

---

## Receiver-side model

The receiver-side pipeline is:

1. identify the original packet via packet ID
2. accept segments into a bounded reassembly buffer
3. reorder segments within a configurable reorder window
4. reassemble the original packet if all required data arrives in time
5. emit the reconstructed original payload packet upward
6. discard incomplete or irrecoverable packets after timeout or pressure limits
7. expose operator-visible completion/drop/reorder statistics

---

## Segment sizing

Segment sizing must be **MTU-aware from day one**.

This means the sender must account for:

- smallest currently usable effective path MTU in the selected WAN set
- Transitloom segment header overhead
- any optional parity/FEC overhead if enabled

Segment sizing should not assume one global fixed payload size unless that size is derived safely from the currently eligible WAN set.

This prevents segmentation behavior that would otherwise create downstream MTU problems.

---

## Segment distribution model

Segments should not be assigned equally by default.

Instead, assignment should be **weighted** by a combination of:

- configured capacity hints
- measured effective throughput / current usable capacity
- current measured path quality
- stability of the paths in the selected set

### Dynamic adjustment

If one WAN becomes slower but is not failed, the sender should:
- reduce its weight dynamically
- not necessarily remove it immediately
- continue bounded operation without full failover unless higher-level policy decides otherwise

This mode is intended for **stable** WAN sets, but not necessarily equal WANs.

---

## Reassembly model

Reassembly must be:

- bounded in memory
- bounded in time
- explicit in observability
- distinct from mode-selection logic

### Reassembly timeout

Timeout should be **adaptive**, based on measured path spread and current observed behavior.

It should not be a single hardcoded fixed timeout if better bounded adaptation is possible.

### Reorder window

Reordering should use a **configurable reorder window**.

This window exists to tolerate stable-but-not-identical paths without unbounded waiting.

---

## Pressure handling

When reassembly pressure becomes too high, the receiver should:

- **drop oldest incomplete packets first**

This is the initial explicit pressure policy.

Why:
- it bounds memory growth
- it is operationally simple
- it avoids requiring v1 mode-switch behavior in the first implementation

Future refinements may add broader pressure-aware behavior, but v1 pressure handling should remain simple and explicit.

---

## Loss handling

If one required segment is lost or too late, the original packet cannot be emitted unless sufficient repair data exists.

### Initial behavior baseline

The first minimal mode should support:
- dropping the original packet when reassembly fails
- relying on upper-layer recovery when repair is unavailable

### Optional FEC

FEC should be:
- optional
- encouraged
- not required for the first minimal implementation

That means the long-term design should support parity/redundancy, but the system should also remain meaningful without requiring FEC for the initial implementation.

---

## FEC model direction

FEC is not required to define the mode, but the mode should be designed so that FEC can be added naturally.

When enabled, FEC/parity should:
- be explicitly configured
- add observable overhead
- expose recovery statistics
- remain bounded

At minimum, future observability should distinguish:
- packets completed without repair
- packets completed with repair
- packets unrecoverable even with parity present

---

## Observability requirements

This mode must expose enough state that operators can understand whether it is helping or hurting.

At minimum, status surfaces should show:

- whether segmented aggregation mode is active
- the eligible WAN/path set
- per-WAN segment weights
- packet completion count
- packet drop count
- timeout-related drop count
- pressure-related drop count
- reorder / wait behavior

If FEC is enabled, status should also show:
- parity overhead
- repaired packet count
- unrecoverable packet count

This mode should be explainable, not opaque.

---

## Operator controls

At minimum, the mode should support operator control over:

- enable/disable of segmented aggregation mode
- per-service or per-association activation
- segmentation threshold
- reorder window
- reassembly pressure limits
- FEC enablement and basic redundancy settings
- observability detail level if needed

Association-level override should be allowed.

---

## Recommended v1 implementation phases

This mode is large enough that implementation should be phased.

### Phase 1 — segmented aggregation baseline
- explicit transport header
- original-packet segmentation
- weighted multi-WAN segment assignment
- MTU-aware segment sizing
- bounded receiver reassembly
- adaptive timeout
- configurable reorder window
- oldest-incomplete drop under pressure
- observability for weights / completion / drops / reorder / pressure

### Phase 2 — optional FEC enhancement
- optional parity segments
- recovery counters
- bounded repair logic
- operator controls for redundancy

This document describes the mode direction; initial implementation should likely begin with Phase 1.

---

## Suggested status language

Operator-visible status should avoid vague language.

Preferred phrasing patterns:

- `segmented-aggregation: active`
- `eligible-wan-set: stable`
- `segment-weight[path-X]: ...`
- `reassembly: pressure drop`
- `packet completion: ...`
- `packet unrecoverable: ...`
- `fec: enabled/disabled`

Avoid vague summaries like:
- “health good”
- “bonding active”
- “optimized”

The mode should be inspectable and honest.

---

## Security and correctness notes

Because this mode adds Transitloom transport metadata and receiver reassembly behavior, implementations must treat these carefully:

- packet IDs must be unambiguous
- segment identity must be explicit
- incomplete and duplicate segment handling must be bounded
- reassembly buffers must not grow unbounded
- observability must not overclaim success under stale or missing data

These concerns should be addressed in the later implementation tasks and detailed transport spec.

---

## Open items for future detailed spec work

This document establishes direction, but later detailed design still needs to define:

- exact transport header format
- exact packet ID and segment numbering scheme
- exact weighted segment scheduler algorithm
- exact adaptive timeout formula
- exact reassembly buffer limits
- exact FEC/parity format if enabled
- exact packet classification rules for “keep whole”
- exact mode-selection and recommendation thresholds
- exact observability field names and report format

---

## Summary

Stable-link segmented aggregation mode is an explicit, advanced, non-default Transitloom dataplane mode for stable multi-WAN environments.

It exists to provide true multi-WAN bandwidth aggregation by:
- segmenting eligible original payload packets
- distributing segments across stable eligible WANs using weighted scheduling
- reassembling the original packet at the receiver
- handling timeout, reorder, and pressure explicitly
- exposing enough runtime state for operators to understand what is happening

It should be built as a separate mode, not as a silent extension of conservative whole-packet behavior.

---
