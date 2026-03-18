# v1 Stable-Link Segmented Aggregation Mode

## Status

Draft

## Purpose

Define an optional advanced Transitloom dataplane mode for **true multi-WAN bandwidth aggregation across measured-stable WAN sets**.

This mode is distinct from Transitloom’s existing conservative whole-packet mode.

Its purpose is to allow a single original payload packet to be carried as multiple Transitloom transport segments across multiple eligible WAN paths, then reassembled into the original payload packet at the receiver.

This mode is intended for environments where:

- multiple WAN paths are available
- those paths are sufficiently stable
- the operator explicitly opts in
- the additional transport overhead and receiver complexity are acceptable
- stronger bandwidth aggregation is worth the trade-offs

This mode is **not** intended to replace the existing conservative whole-packet mode.

---

## Why this mode exists

Transitloom’s existing whole-packet mode is well suited for:

- variable WAN quality
- unstable path conditions
- low transformation
- simpler failure handling
- preserving whole-packet carriage behavior

But whole-packet path selection has a structural limitation:

- one original packet is sent on one selected path at a time

That works well for robustness, but it limits true aggregation across multiple stable WANs.

This mode exists to support a different operating point:

- use multiple WANs simultaneously for one original packet
- exploit measured-stable WAN sets for stronger aggregate throughput
- accept transport-level segmentation, reassembly, timeout, pressure, and observability complexity as the cost

---

## Relationship to existing mode

Transitloom should support at least two distinct dataplane modes.

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
- a complete FEC implementation in the first minimal phase
- a broad measurement platform or worker framework

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

## Activation model

This mode should activate only when all of the following are true:

1. the operator has explicitly enabled it
2. the selected WAN/path set is measured stable enough
3. the service or association policy allows it
4. the packet is eligible for segmentation
5. the system has not exited the mode due to sustained degradation

### Operator enablement

The preferred enablement model is:

- Transitloom may recommend this mode based on measured path stability
- the operator must still explicitly opt in
- the mode may be configured per-service
- association-specific settings may override service-level defaults

### Config precedence

For this mode, config precedence should be:

1. system defaults
2. service-level mode and policy settings
3. association-level override

That keeps reusable defaults while preserving precise per-association control.

---

## Stable-path eligibility

A WAN/path set is eligible only when path variation is sufficiently bounded.

The exact measured thresholds may evolve, but eligibility should consider at least:

- RTT spread
- jitter spread
- loss behavior
- freshness of measurements
- enough evidence to avoid weak conclusions from sparse data

If the path set is not stable enough, this mode should not activate.

### Exit rule

If stability degrades after the mode is active, Transitloom should not leave the mode immediately.

Instead:

- detect degraded stability
- apply bounded degradation dwell / confirmation
- then revert to conservative whole-packet mode if stability remains insufficient

This keeps exit behavior sane and avoids thrashing.

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

### Segmentation threshold

The initial threshold model should be:

- configurable per service
- overridable per association

Adaptive thresholding may come later, but the first implementation should use explicit configured thresholds.

### Keep-whole classification

Even when the mode is enabled, some packets should remain whole.

At minimum, v1 should support keeping these whole:

- packets below the segmentation threshold
- explicitly classified control-sensitive packets
- packets on associations or services where operator policy says to remain whole

This is especially important for flagship use cases such as:

- WireGuard over Transitloom

---

## Sender model

For each eligible payload packet:

1. existing Transitloom scheduler chooses the eligible WAN/path set
2. sender confirms segmented mode is still allowed
3. sender computes effective segment size from current path MTU constraints
4. sender segments the original payload packet into Transitloom transport segments
5. sender assigns segments across WANs using weighted scheduling
6. sender transmits segments
7. sender records operator-visible runtime statistics

### Important architectural rule

The existing scheduler should still determine the **eligible WAN/path set**.

This mode should then apply its own **local segment scheduler** within that eligible set.

This is a hybrid model:

- existing scheduler chooses the allowed stable set
- segmented mode decides how to distribute segments inside that set

This mode does **not** replace the existing scheduler.

---

## Packet identity model

Original packet identity must be scoped to:

- association
- direction
- sender epoch/session
- packet sequence within that scope

This is the recommended packet identity model for v1.

### Why this scope exists

This scope avoids:

- restart ambiguity
- accidental packet ID reuse across opposite directions
- stale incomplete packets surviving sender restart confusion

### Sender restart rule

When sender epoch changes:

- receiver must immediately discard incomplete packets from the old sender epoch

This keeps reassembly semantics clean and bounded.

---

## Segment header expectations

This document does not lock the final wire format yet, but the transport header must be able to represent at least:

- mode/version
- association or path context sufficient for reassembly scope
- sender epoch/session
- original packet ID
- segment index
- segment count
- original payload length
- flags indicating whether parity/FEC is present if enabled

### Integrity for v1

For v1:

- segment headers should rely on the **outer secure Transitloom transport layer** for integrity/authentication

Additional per-segment integrity fields may be introduced later if needed, but should not be required for the first implementation phase.

---

## Segment sizing

Segment sizing must be **MTU-aware from day one**.

The sender must account for:

- smallest usable path MTU in the currently selected WAN set
- Transitloom segment header overhead
- any optional FEC overhead if enabled later

Segment sizing should not assume one global fixed payload size unless that size is safely derived from the currently eligible WAN set.

This is required, not optional.

---

## Segment distribution model

Segments should not be distributed equally by default.

Instead, assignment should be **weighted** by a combination of:

- configured capacity hints
- measured effective throughput / usable capacity
- current measured path quality
- stability of the paths in the selected set

### Weight update cadence

Weights should not be recomputed for every segment.

Instead, weights should update per **short scheduling window**.

This keeps behavior more stable and avoids excessive jitter or complexity.

### Degraded-but-not-dead paths

If one WAN becomes slower but is not failed, the sender should:

- reduce its weight dynamically
- not necessarily remove it immediately
- continue bounded operation unless higher-level policy decides otherwise

This mode is intended for **stable** WAN sets, but not necessarily equal WANs.

---

## Receiver model

The receiver-side pipeline is:

1. parse segment header
2. identify the original packet using packet ID scope
3. place segment into bounded reassembly buffer
4. reorder segments within configurable reorder window
5. reconstruct the original packet if enough data arrives in time
6. emit the reconstructed original payload packet upward
7. if incomplete, timeout or pressure handling drops the packet
8. update operator-visible observability

---

## Reassembly model

Reassembly must be:

- bounded in memory
- bounded in time
- explicit in observability
- distinct from mode-selection logic

### Reassembly timeout

Timeout should be **adaptive**, based on measured path spread and current observed behavior.

It should not be a single fixed hardcoded timeout if bounded adaptation is possible.

### Reorder window

Reordering should use a **configurable reorder window**.

This window exists to tolerate stable-but-not-identical paths without unbounded waiting.

---

## Reassembly bounds

Reassembly must be bounded by all three of these:

- **max incomplete packets per association**
- **max bytes per association**
- **global max reassembly bytes**

This provides:

- fairness per association
- global memory protection
- bounded behavior under pressure

All three bounds should be considered required in the design.

---

## Pressure handling

When reassembly pressure becomes too high, the immediate v1 pressure response should be:

- **drop oldest incomplete packets first**

This is the first explicit pressure policy.

Why:
- it bounds memory growth
- it is operationally simple
- it avoids requiring forced mode-switch behavior in the first implementation

### Sustained pressure behavior

If pressure remains sustained:

- raise an operator-visible **degraded state**
- recommend fallback to conservative whole-packet mode

The first implementation should **not** automatically force fallback purely from sustained pressure unless later policy work explicitly adds that behavior.

---

## Loss handling

If one required segment is lost or too late, the original packet cannot be emitted unless enough repair data exists.

### Initial baseline

The first phase should support:

- dropping the original packet when reassembly fails
- relying on upper-layer recovery when repair is unavailable

### Absent vs failed data

This mode must preserve the distinction between:

- absent measurement or absent repair
- explicit failed repair or failed reassembly

These are not the same thing and should not be collapsed in observability or policy.

---

## FEC direction

### Long-term direction

The intended future FEC family is:

- **Reed-Solomon style block coding**

### Coding granularity

The intended coding window is:

- across **multiple original packets**
- not just within one original packet’s segment set

This is a significant capability and should be treated as a later implementation phase.

### Implementation recommendation

Even though the design direction includes multi-packet FEC, implementation should be phased.

---

## Recommended implementation phases

### Phase 1 — segmented aggregation baseline

The first implementation phase should include:

- explicit transport header
- original-packet segmentation
- weighted multi-WAN segment assignment
- MTU-aware segment sizing
- bounded receiver reassembly
- adaptive timeout
- configurable reorder window
- oldest-incomplete drop under pressure
- observability for weights / completion / drops / reorder / pressure
- mode entry/exit behavior without FEC

### Phase 2 — optional block-coded FEC extension

The later implementation phase should include:

- optional Reed-Solomon style coding
- coding windows spanning multiple original packets
- repair/recovery logic
- FEC counters and visibility
- coding block timeout/flush behavior
- explicit operator controls for FEC parameters

This is the recommended engineering decomposition.

---

## Observability requirements

This mode must expose enough state that operators can understand whether it is helping or hurting.

At minimum, status surfaces should show:

- whether segmented mode is active
- why it is active or inactive
- why a packet was segmented or kept whole
- the eligible WAN/path set
- per-WAN segment weights
- packet completion count
- packet drop count
- timeout-related drop count
- pressure-related drop count
- reorder statistics
- degraded-state indicator
- recommendation to fall back if persistent pressure exists

### Later FEC observability

If FEC is enabled later, status should also show:

- parity overhead
- repaired packet count
- unrecoverable packet count
- coding window statistics

This mode should be explainable, not opaque.

---

## Operator controls

At minimum, the mode should support operator control over:

- enable/disable of segmented aggregation mode
- per-service activation
- per-association override
- segmentation threshold
- reorder window
- reassembly pressure limits
- degraded-state reporting
- later FEC enablement and basic redundancy settings

Association-level override should be allowed.

---

## Suggested operator-visible language

Operator-visible status should avoid vague wording.

Preferred phrasing patterns:

- `segmented-aggregation: active`
- `segmented-aggregation: inactive (stability not met)`
- `segment-weight[path-X]: ...`
- `packet classification: kept whole (control)`
- `packet classification: segmented`
- `reassembly: pressure drop`
- `reassembly: timeout drop`
- `mode state: degraded`
- `fallback recommendation: whole-packet mode`

Avoid vague summaries like:

- `health good`
- `bonding active`
- `optimized`

This mode should be inspectable and honest.

---

## Security and correctness notes

Because this mode adds Transitloom transport metadata and receiver reassembly behavior, implementations must treat these carefully:

- packet IDs must be unambiguous
- segment identity must be explicit
- incomplete and duplicate segment handling must be bounded
- reassembly buffers must not grow unbounded
- stale sender epochs must be discarded explicitly
- observability must not overclaim success under stale or missing data

These concerns should be addressed in later implementation tasks and detailed transport design work.

---

## Open items for later detailed design

This document establishes direction, but later detailed design still needs to define:

- exact segment wire format
- exact sequence numbering and packet ID encoding
- exact weighted segment scheduler algorithm
- exact adaptive timeout formula
- exact reorder window defaults and bounds
- exact reassembly memory defaults and bounds
- exact degraded-state thresholds and fallback recommendation logic
- exact Reed-Solomon coding parameters and block structure
- exact packet classification rules for keep-whole traffic
- exact mode recommendation thresholds
- exact observability field names and report structures

---

## Summary

Stable-link segmented aggregation mode is an explicit, advanced, non-default Transitloom dataplane mode for stable multi-WAN environments.

It exists to provide true multi-WAN bandwidth aggregation by:

- segmenting eligible original payload packets
- distributing segments across stable eligible WANs using weighted scheduling
- reassembling the original packet at the receiver
- handling timeout, reorder, restart, and pressure explicitly
- exposing enough runtime state for operators to understand what is happening

It should be built as a separate mode, not as a silent extension of conservative whole-packet behavior.

The recommended engineering path is:

- **Phase 1:** segmented aggregation baseline without FEC
- **Phase 2:** optional Reed-Solomon style coding across multiple original packets

---
