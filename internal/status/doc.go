// Package status provides runtime status summary helpers for Transitloom.
//
// It defines narrow, explicit summary types for each major state category,
// and provides methods that produce human-readable report lines for logging
// and operator inspection.
//
// # Design principles
//
// Each summary type covers one clear state category. Categories are never
// merged into a vague "status" — doing so would erase important architectural
// distinctions such as the difference between configured state and applied
// state, or between bootstrap/cached state and authoritative coordinator truth.
//
// Applied runtime state is always distinguished from candidate or computed
// state. A ScheduledEgressSummary reports what carrier was actually activated,
// not just what the scheduler computed. These are different claims with
// different confidence levels.
//
// Bootstrap/cached state is never labeled as stronger truth than it actually
// is. A "ready" BootstrapSummary reflects local material coherence only —
// not coordinator authorization.
//
// The package depends on snapshot data passed by the caller, not on live
// goroutines or runtime state machines. Summary construction is a pure
// function over snapshot data, which keeps this package testable with
// synthetic inputs and prevents hidden state coupling.
//
// # State categories covered
//
//   - Node bootstrap/readiness: local identity and admission token coherence.
//     Not to be confused with coordinator authorization. "Ready" means local
//     material looks coherent, not that the coordinator has accepted the node.
//
//   - Coordinator service registry: what services are registered.
//     Bootstrap-only records are placeholders, not final authenticated state.
//     "Registered" does not mean "authorized" or "discoverable to all nodes."
//
//   - Coordinator association store: what association records exist.
//     Pending/bootstrap-only associations do not imply forwarding-state
//     installation or that traffic can currently flow.
//
//   - Scheduled egress: what the scheduler decided and which carrier was
//     actually activated. "Activated" is stronger than "decided" — a carrier
//     must start successfully to be reported as active. The summary also
//     exposes live traffic counters per activated carrier so an operator can
//     confirm that data-plane behavior matches scheduler intent.
package status
