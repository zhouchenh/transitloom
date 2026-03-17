package node

import (
	"context"
	"fmt"
	"time"

	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/transport"
)

// DefaultProbeInterval is the default minimum time between active probe rounds.
// A round fires at most once per interval. This bound prevents uncontrolled
// probe fan-out and keeps the probe loop low-overhead for background operation.
const DefaultProbeInterval = 30 * time.Second

// DefaultMaxProbeTargetsPerRound is the default maximum number of endpoints
// probed per round. Bounded to prevent a large endpoint registry from producing
// a runaway probe sweep. Probe rounds are narrow and targeted, not full scans.
const DefaultMaxProbeTargetsPerRound = 10

// ProbeSchedulerConfig configures the bounded active probe loop.
//
// Both fields are explicit bounds on the probe loop's behavior. They ensure that
// the loop remains:
//   - bounded in frequency (ProbeInterval)
//   - bounded in fan-out (MaxTargetsPerRound)
//
// This is not a policy tuning surface for hysteresis or switching behavior.
// Switching policy belongs in DirectRelayFallbackPolicy (T-0024 scope).
type ProbeSchedulerConfig struct {
	// ProbeInterval is the minimum time between successive probe rounds.
	// The loop fires no more than once per interval.
	ProbeInterval time.Duration

	// MaxTargetsPerRound limits how many endpoints are probed in one round.
	// When more targets exist, the highest-priority ones are probed first.
	MaxTargetsPerRound int
}

// DefaultProbeSchedulerConfig returns conservative v1 defaults.
func DefaultProbeSchedulerConfig() ProbeSchedulerConfig {
	return ProbeSchedulerConfig{
		ProbeInterval:      DefaultProbeInterval,
		MaxTargetsPerRound: DefaultMaxProbeTargetsPerRound,
	}
}

// ProbeTarget describes one endpoint to probe in the next round, along with
// the quality-store path IDs that should be updated when a result is received.
//
// ProbeTarget is the ephemeral probe work item that bridges two distinct layers:
//   - EndpointRegistry (address-level reachability): always updated via ApplyProbeResult.
//   - PathQualityStore (RTT/jitter/loss for scheduling): updated only when PathIDs is non-empty.
//
// PathIDs links a reachability probe (host:port → Reachable/RTT) to the
// path-quality-store entries for paths that use this endpoint. Without this
// linkage, a successful probe would update endpoint freshness but leave the
// scheduler's quality inputs unmeasured.
//
// Empty PathIDs is valid: some endpoints (e.g., coordinator relay addresses)
// may not correspond to any quality-store candidate, or the quality-store
// linkage may not yet be established. In that case endpoint freshness is still
// updated, but quality measurements are not recorded.
//
// Note: absent measurement (PathIDs empty or quality store not yet updated) is
// explicitly distinct from failed measurement (Reachable=false with PathIDs set).
type ProbeTarget struct {
	Host   string
	Port   uint16
	Reason transport.CandidateReason

	// PathIDs are the quality-store path IDs corresponding to this endpoint.
	// Quality measurements are updated for each ID in this slice.
	// An empty slice means no quality update occurs for this target —
	// not the same as a failed probe, where quality IS updated to reflect failure.
	PathIDs []string
}

// ProbeRoundDetail is the per-target result from one probe attempt in a round.
type ProbeRoundDetail struct {
	Host    string
	Port    uint16
	Reason  transport.CandidateReason
	PathIDs []string

	Reachable bool
	RTT       time.Duration // zero when Reachable=false or when an error occurred

	// Error is non-empty only for unexpected executor errors (e.g., context
	// cancellation, nonce generation failure). "Endpoint not reachable" is not
	// an error here — it comes back as Reachable=false with a nil error.
	Error string
}

// ProbeRoundResult summarizes the outcome of one bounded probe round.
//
// This is the primary operator-facing observability surface for the active
// probe loop. It records how many targets were selected, how many probes
// succeeded or failed, and per-target details including RTT and path IDs.
//
// TargetsSelected is set from the input slice before execution, so it always
// reflects what was attempted even when the context is cancelled mid-round.
type ProbeRoundResult struct {
	TargetsSelected int
	Reachable       int
	Unreachable     int
	Errors          int // unexpected executor errors; not "endpoint not reachable"
	Details         []ProbeRoundDetail
}

// ReportLines produces human-readable log lines for operator observability.
//
// The format makes the probe loop explicitly inspectable: operators can see
// which endpoints were targeted, what the outcome was, and which quality-store
// path IDs were updated. This is required for the "not uninspectable" constraint.
func (r ProbeRoundResult) ReportLines() []string {
	lines := make([]string, 0, len(r.Details)+1)
	lines = append(lines, fmt.Sprintf(
		"probe-round: selected=%d reachable=%d unreachable=%d errors=%d (targeted-first; registry+quality wired)",
		r.TargetsSelected, r.Reachable, r.Unreachable, r.Errors,
	))
	for _, d := range r.Details {
		if d.Error != "" {
			lines = append(lines, fmt.Sprintf(
				"  %s:%d [%s]: executor-error: %s",
				d.Host, d.Port, d.Reason, d.Error,
			))
			continue
		}
		if d.Reachable {
			lines = append(lines, fmt.Sprintf(
				"  %s:%d [%s]: reachable rtt=%s quality-path-ids=%v",
				d.Host, d.Port, d.Reason, d.RTT.Round(time.Millisecond), d.PathIDs,
			))
		} else {
			lines = append(lines, fmt.Sprintf(
				"  %s:%d [%s]: unreachable quality-path-ids=%v",
				d.Host, d.Port, d.Reason, d.PathIDs,
			))
		}
	}
	return lines
}

// BuildPathIDMap constructs a mapping from "host:port" to path-quality-store path IDs.
//
// When a probe result arrives for "1.2.3.4:4500", we need to know which quality-store
// entries (path IDs) to update. This map provides that linkage.
//
// Two sources are used:
//  1. Config-derived scheduled activation inputs (from BuildScheduledActivationInputs):
//     each direct endpoint string → path ID "assocID:direct".
//  2. Coordinator-distributed candidates (from CandidateStore):
//     each candidate's RemoteEndpoint → its CandidateID.
//
// When both sources map the same host:port, both path IDs are included.
// The result is passed to SelectProbeTargets so probe results can update
// quality for all paths sharing that endpoint.
//
// Endpoint freshness (EndpointRegistry) and path quality (PathQualityStore)
// remain distinct: this map only provides the linkage for quality updates;
// endpoint freshness is always updated by ApplyProbeResult regardless of this map.
func BuildPathIDMap(inputs []ScheduledActivationInput, candidateStore *CandidateStore) map[string][]string {
	m := make(map[string][]string)

	// Config-derived paths: direct endpoint string (already "host:port") → assocID:direct.
	// These are the static-config path IDs used by buildPathCandidatesFromEndpoints.
	for _, input := range inputs {
		if input.DirectEndpoint != "" {
			pathID := input.AssociationID + ":direct"
			m[input.DirectEndpoint] = appendUniquePathID(m[input.DirectEndpoint], pathID)
		}
	}

	// Coordinator-distributed candidates: RemoteEndpoint → CandidateID.
	// CandidateID is the quality-store key for distributed candidates
	// (applied by RefineCandidates → PathQualityStore.FreshQuality(candidateID)).
	if candidateStore != nil {
		for _, set := range candidateStore.Snapshot() {
			for _, c := range set.Candidates {
				if c.RemoteEndpoint != "" && c.CandidateID != "" {
					m[c.RemoteEndpoint] = appendUniquePathID(m[c.RemoteEndpoint], c.CandidateID)
				}
			}
		}
	}

	return m
}

// appendUniquePathID appends s to slice only if not already present.
// Avoids duplicate path IDs when the same endpoint appears in multiple sources.
func appendUniquePathID(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}

// SelectProbeTargets builds a bounded, targeted list of endpoints to probe
// in the next round.
//
// Target selection is targeted-first: candidates come only from endpoints
// already in the registry — no new host:port combinations are invented.
// Selection priority:
//  1. Unverified endpoints (never probed; need initial verification)
//  2. Stale and failed endpoints (need revalidation after health/IP events)
//
// pathIDMap maps "host:port" → []pathID for quality-store linkage. A nil or
// empty map means no quality updates occur; endpoint freshness is still updated.
//
// The result is bounded by maxTargets. When more candidates exist, excess
// (lower-priority) targets are dropped. This is the structural guard against
// runaway probe fan-out.
//
// This function is pure and does not perform any network activity.
// Callers drive execution via ExecuteProbeRound.
//
// Probe scheduling remains distinct from hysteresis/switching policy:
// this function selects targets for freshness verification and quality
// measurement, not to make path-switching decisions. Switching decisions
// belong to DirectRelayFallbackPolicy (T-0024 scope).
func SelectProbeTargets(
	registry *transport.EndpointRegistry,
	pathIDMap map[string][]string,
	maxTargets int,
) []ProbeTarget {
	if registry == nil || maxTargets <= 0 {
		return nil
	}

	// Derive targeted candidates using the existing BuildCandidatesFromEndpoints
	// helper, which enforces targeted-first discipline (no port-range invention).
	// Unverified first, then stale/failed for revalidation.
	unverified := registry.SelectForInitialVerification()
	revalidation := registry.SelectForRevalidation()

	unverifiedCandidates := transport.BuildCandidatesFromEndpoints(unverified, false)
	revalidationCandidates := transport.BuildCandidatesFromEndpoints(revalidation, false)

	// Merge: unverified candidates have higher priority (probed first).
	allCandidates := append(unverifiedCandidates, revalidationCandidates...) //nolint:gocritic

	seen := make(map[string]bool)
	targets := make([]ProbeTarget, 0, min(maxTargets, len(allCandidates)))

	for _, c := range allCandidates {
		if len(targets) >= maxTargets {
			break
		}

		key := fmt.Sprintf("%s:%d", c.Host, c.Port)
		if seen[key] {
			continue // deduplicate: same address from multiple endpoint records
		}
		seen[key] = true

		var pathIDs []string
		if pathIDMap != nil {
			// Copy to avoid sharing the caller's underlying slice.
			src := pathIDMap[key]
			if len(src) > 0 {
				pathIDs = make([]string, len(src))
				copy(pathIDs, src)
			}
		}

		targets = append(targets, ProbeTarget{
			Host:    c.Host,
			Port:    c.Port,
			Reason:  c.Reason,
			PathIDs: pathIDs,
		})
	}

	return targets
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ExecuteProbeRound executes a bounded probe round for the given targets.
//
// For each target, this function:
//  1. Executes a UDP probe via the executor (UDPProbeExecutor in production).
//  2. Applies the probe result to the endpoint registry — this updates
//     VerificationState (address-level reachability truth).
//  3. Applies the probe result to the path-quality store for each PathID in
//     the target — this updates RTT/jitter/loss/confidence for scheduling.
//
// Endpoint freshness and path quality are updated as distinct layers:
//   - registry.ApplyProbeResult: always called when registry is non-nil.
//     Updates VerificationState: verified (reachable) or failed (unreachable).
//   - qualityStore.RecordProbeResult: called per PathID in target.PathIDs.
//     Updates RTT/jitter/loss/confidence for the scheduler's quality inputs.
//
// These two updates serve different consumers downstream:
//   - The refinement layer (RefineCandidates) reads the registry for endpoint
//     exclusion (failed → exclude candidate; stale → degrade health).
//   - The scheduler (Scheduler.Decide) reads quality-store inputs via
//     ApplyCandidates / FreshQuality for fine-grained scoring.
//
// Absent measurement vs failed measurement:
//   - A target with empty PathIDs has its endpoint freshness updated but no
//     quality update occurs. The quality store has no entry for that path.
//     FreshQuality returns (zero, false) — the path is "unmeasured", not failed.
//   - A target with non-empty PathIDs that gets Reachable=false has quality
//     updated (confidence decreases, loss fraction rises). The path is "measured
//     and failed", not merely "unmeasured".
//
// The loop is bounded by the length of the targets slice. Callers use
// SelectProbeTargets(maxTargets) to control the bound. Context cancellation
// stops probing after the current in-flight probe completes.
//
// A nil registry or qualityStore is accepted; the corresponding update is skipped.
func ExecuteProbeRound(
	ctx context.Context,
	targets []ProbeTarget,
	executor transport.ProbeExecutor,
	registry *transport.EndpointRegistry,
	qualityStore *scheduler.PathQualityStore,
) ProbeRoundResult {
	result := ProbeRoundResult{
		TargetsSelected: len(targets),
		Details:         make([]ProbeRoundDetail, 0, len(targets)),
	}

	for _, target := range targets {
		// Check context before each probe to allow clean cancellation between probes.
		if ctx.Err() != nil {
			break
		}

		candidate := transport.ProbeCandidate{
			Host:   target.Host,
			Port:   target.Port,
			Reason: target.Reason,
		}

		probeResult, execErr := executor.Execute(ctx, candidate)

		detail := ProbeRoundDetail{
			Host:    target.Host,
			Port:    target.Port,
			Reason:  target.Reason,
			PathIDs: target.PathIDs,
		}

		if execErr != nil {
			// Unexpected executor error (context cancelled, nonce generation failed,
			// etc.). This is not "endpoint not reachable" — that comes back as
			// Reachable=false with a nil error from UDPProbeExecutor.
			// Do not update the registry or quality store for executor errors:
			// we do not know whether the endpoint is reachable or not.
			detail.Error = execErr.Error()
			result.Errors++
			result.Details = append(result.Details, detail)
			continue
		}

		detail.Reachable = probeResult.Reachable
		detail.RTT = probeResult.RoundTripTime

		if probeResult.Reachable {
			result.Reachable++
		} else {
			result.Unreachable++
		}

		// Wire 1: endpoint freshness update (address-level reachability truth).
		// Always applied when registry is non-nil, regardless of path ID linkage.
		// This update drives the refinement layer's endpoint-exclusion decisions:
		//   Reachable=true  → VerificationStateVerified
		//   Reachable=false → VerificationStateFailed
		// Endpoint freshness is a distinct layer from path quality (separate stores,
		// separate concerns, separate downstream consumers).
		if registry != nil {
			registry.ApplyProbeResult(probeResult)
		}

		// Wire 2: path quality update (RTT/jitter/loss for scheduler scoring).
		// Applied per PathID that uses this endpoint. Absent path IDs mean no
		// quality update — the path is "unmeasured" (FreshQuality returns false),
		// not "failed" (which would require a recorded probe failure in the store).
		//
		// When Reachable=false and PathIDs is non-empty, this records a probe
		// failure into the quality store: confidence decreases, loss rises.
		// This is "failed measurement" — the opposite of "absent measurement".
		if qualityStore != nil {
			for _, pathID := range target.PathIDs {
				qualityStore.RecordProbeResult(pathID, probeResult.RoundTripTime, probeResult.Reachable)
			}
		}

		result.Details = append(result.Details, detail)
	}

	return result
}

// RunProbeLoop runs a bounded active probe scheduling loop until ctx is cancelled.
//
// On each interval, RunProbeLoop:
//  1. Calls SelectProbeTargets to derive bounded targeted probe candidates.
//  2. Calls ExecuteProbeRound to probe and wire results into registry + quality store.
//  3. Returns if ctx is cancelled.
//
// The loop is bounded in two dimensions:
//   - Frequency: at most one round per cfg.ProbeInterval.
//   - Fan-out: at most cfg.MaxTargetsPerRound endpoints per round.
//
// The loop is explicit and inspectable: each round's results are passed to the
// optional onRound callback for logging or observability. Passing nil for
// onRound is valid.
//
// This function blocks until ctx is cancelled and is intended to be run as a
// goroutine: go RunProbeLoop(ctx, cfg, registry, inputs, candidateStore, executor, qualityStore, onRound).
//
// Probe scheduling is explicitly separate from:
//   - hysteresis/switching policy (DirectRelayFallbackPolicy, T-0024)
//   - candidate refresh automation (ExecuteCandidateRefresh)
//   - scheduler path decisions (Scheduler.Decide)
//
// The loop only drives probe execution and wires results into freshness and
// quality stores. Downstream consumers (RefineCandidates, Scheduler.Decide,
// DirectRelayFallbackPolicy) observe the updated state independently.
func RunProbeLoop(
	ctx context.Context,
	cfg ProbeSchedulerConfig,
	registry *transport.EndpointRegistry,
	inputs []ScheduledActivationInput,
	candidateStore *CandidateStore,
	executor transport.ProbeExecutor,
	qualityStore *scheduler.PathQualityStore,
	onRound func(ProbeRoundResult),
) {
	interval := cfg.ProbeInterval
	if interval <= 0 {
		interval = DefaultProbeInterval
	}
	maxTargets := cfg.MaxTargetsPerRound
	if maxTargets <= 0 {
		maxTargets = DefaultMaxProbeTargetsPerRound
	}

	// Run immediately on first tick rather than waiting a full interval.
	// This ensures the first probe round happens without a cold-start delay.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runRound := func() {
		// Build the path ID map from the current inputs and candidate store.
		// This provides quality-store linkage for probe results.
		pathIDMap := BuildPathIDMap(inputs, candidateStore)

		// Select bounded targeted probe targets.
		targets := SelectProbeTargets(registry, pathIDMap, maxTargets)
		if len(targets) == 0 {
			if onRound != nil {
				onRound(ProbeRoundResult{})
			}
			return
		}

		// Execute the probe round and wire results.
		result := ExecuteProbeRound(ctx, targets, executor, registry, qualityStore)

		if onRound != nil {
			onRound(result)
		}
	}

	// Fire immediately (cold-start probe) before waiting for the first tick.
	runRound()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runRound()
		}
	}
}

// BuildDistributedProbeTargets builds probe targets from coordinator-distributed
// candidates that have a known RemoteEndpoint.
//
// This supplements SelectProbeTargets (which targets endpoints in the
// EndpointRegistry) with candidates from the CandidateStore that may not yet
// have a corresponding EndpointRegistry entry.
//
// For distributed candidates, we want to probe their RemoteEndpoint to verify
// whether the coordinator-provided path is actually usable. Probe results wire
// into the quality store (via PathIDs from the candidate's CandidateID) and
// can also be used to add verified entries to the EndpointRegistry.
//
// Returns an empty slice when candidateStore is nil or no usable candidates exist.
// The result is bounded by maxTargets.
//
// This function is distinct from SelectProbeTargets: that function reads the
// EndpointRegistry (address-level); this reads the CandidateStore (coordinator
// knowledge). Both are targeted-first — no port-range guessing occurs.
func BuildDistributedProbeTargets(
	candidateStore *CandidateStore,
	maxTargets int,
) []ProbeTarget {
	if candidateStore == nil || maxTargets <= 0 {
		return nil
	}

	seen := make(map[string]bool)
	var targets []ProbeTarget

	for _, set := range candidateStore.Snapshot() {
		for _, c := range set.Candidates {
			if len(targets) >= maxTargets {
				return targets
			}
			if c.RemoteEndpoint == "" {
				// No usable endpoint: informational candidate only. Skip.
				continue
			}
			if seen[c.RemoteEndpoint] {
				continue
			}
			seen[c.RemoteEndpoint] = true

			host, port, err := parseHostPort(c.RemoteEndpoint)
			if err != nil {
				continue // malformed address; skip rather than panic
			}

			reason := candidateClassToReason(c)
			var pathIDs []string
			if c.CandidateID != "" {
				pathIDs = []string{c.CandidateID}
			}

			targets = append(targets, ProbeTarget{
				Host:    host,
				Port:    port,
				Reason:  reason,
				PathIDs: pathIDs,
			})
		}
	}

	return targets
}

// parseHostPort parses a "host:port" string into its components.
// Returns an error for malformed input.
func parseHostPort(hostport string) (host string, port uint16, err error) {
	var portStr string
	// net.SplitHostPort would be ideal but we avoid importing net just for this.
	// Use a simple last-colon split, which handles IPv4 and hostnames.
	// IPv6 literal addresses ([::1]:4500) are handled by the leading '[' check.
	lastColon := -1
	for i := len(hostport) - 1; i >= 0; i-- {
		if hostport[i] == ':' {
			lastColon = i
			break
		}
	}
	if lastColon < 0 {
		return "", 0, fmt.Errorf("no colon in host:port %q", hostport)
	}
	host = hostport[:lastColon]
	portStr = hostport[lastColon+1:]

	var p int
	if _, scanErr := fmt.Sscanf(portStr, "%d", &p); scanErr != nil || p <= 0 || p > 65535 {
		return "", 0, fmt.Errorf("invalid port in %q", hostport)
	}
	return host, uint16(p), nil
}

// candidateClassToReason maps a DistributedPathCandidate class to a CandidateReason.
// Relay candidates (coordinator-relay, node-relay) map to CoordinatorObserved;
// direct candidates map to their effective source reason.
func candidateClassToReason(c controlplane.DistributedPathCandidate) transport.CandidateReason {
	if c.IsRelayAssisted {
		// Coordinator-provided relay endpoint: the coordinator knows the relay address.
		return transport.CandidateReasonCoordinatorObserved
	}
	// Direct candidate: could be configured or coordinator-observed.
	// Use CoordinatorObserved as a conservative default (coordinator provided it).
	return transport.CandidateReasonCoordinatorObserved
}
