package transport

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"time"
)

// Probe wire protocol.
//
// A Transitloom probe datagram is 13 bytes:
//
//	bytes 0–3: probe magic "TLPR" (0x54 0x4C 0x50 0x52)
//	byte    4: type byte (ProbeTypeRequest or ProbeTypeResponse)
//	bytes 5–12: uint64 challenge nonce, little-endian
//
// The probe responder echoes the nonce back with ProbeTypeResponse.
// The prober matches the response nonce to its request to confirm that it
// received a genuine echo from the remote endpoint.
//
// This protocol is intentionally minimal: its purpose is to verify that a
// remote UDP endpoint can receive and respond — not to negotiate configuration
// or carry application data. Full NAT traversal is intentionally out of scope.
const probeDatagramLen = 13

const (
	probeMagic0 = 0x54 // 'T'
	probeMagic1 = 0x4C // 'L'
	probeMagic2 = 0x50 // 'P'
	probeMagic3 = 0x52 // 'R'

	// ProbeTypeRequest is the type byte for probe request datagrams.
	ProbeTypeRequest byte = 0x01

	// ProbeTypeResponse is the type byte for probe response datagrams.
	ProbeTypeResponse byte = 0x02
)

// encodeProbeRequest encodes a 13-byte probe request datagram with the given nonce.
func encodeProbeRequest(nonce uint64) [probeDatagramLen]byte {
	var d [probeDatagramLen]byte
	d[0], d[1], d[2], d[3] = probeMagic0, probeMagic1, probeMagic2, probeMagic3
	d[4] = ProbeTypeRequest
	binary.LittleEndian.PutUint64(d[5:], nonce)
	return d
}

// encodeProbeResponse encodes a 13-byte probe response datagram with the given nonce.
func encodeProbeResponse(nonce uint64) [probeDatagramLen]byte {
	var d [probeDatagramLen]byte
	d[0], d[1], d[2], d[3] = probeMagic0, probeMagic1, probeMagic2, probeMagic3
	d[4] = ProbeTypeResponse
	binary.LittleEndian.PutUint64(d[5:], nonce)
	return d
}

// parseProbeDatagram parses a probe datagram and returns its type byte and nonce.
// Returns an error if buf is too short or the magic header is absent.
func parseProbeDatagram(buf []byte) (probeType byte, nonce uint64, err error) {
	if len(buf) < probeDatagramLen {
		return 0, 0, fmt.Errorf("probe: datagram too short (%d bytes, need %d)", len(buf), probeDatagramLen)
	}
	if buf[0] != probeMagic0 || buf[1] != probeMagic1 || buf[2] != probeMagic2 || buf[3] != probeMagic3 {
		return 0, 0, fmt.Errorf("probe: missing magic header")
	}
	return buf[4], binary.LittleEndian.Uint64(buf[5:]), nil
}

// IsProbeDatagram reports whether buf is a Transitloom probe datagram: it
// must begin with the probe magic bytes and be at least probeDatagramLen bytes.
//
// Listeners that multiplex probe and application traffic on one UDP port can
// use this to identify probe datagrams before dispatching to HandleProbeDatagram.
func IsProbeDatagram(buf []byte) bool {
	return len(buf) >= probeDatagramLen &&
		buf[0] == probeMagic0 &&
		buf[1] == probeMagic1 &&
		buf[2] == probeMagic2 &&
		buf[3] == probeMagic3
}

// CandidateReason identifies why a specific endpoint was chosen as a probe
// candidate.
//
// Every probe candidate must have an explicit reason that traces it to a
// known source. This enforces the targeted-first probing discipline:
// Transitloom does not probe arbitrary port ranges. Candidates must come
// from configured endpoints, router-protocol reports, coordinator-observed
// addresses, or previously verified endpoints.
type CandidateReason string

const (
	// CandidateReasonConfigured means the endpoint was explicitly declared
	// in operator configuration. Highest-confidence source.
	CandidateReasonConfigured CandidateReason = "configured"

	// CandidateReasonRouterDiscovered means a router protocol (UPnP, PCP,
	// NAT-PMP) reported this external mapping. The router confirmed the
	// forwarding rule, but the rule may have since expired or changed.
	CandidateReasonRouterDiscovered CandidateReason = "router-discovered"

	// CandidateReasonCoordinatorObserved means the coordinator observed this
	// node's public IP during the control session, and the port comes from an
	// explicitly configured forwarded port. Coordinator source-address
	// observation alone does not imply any specific inbound port is accessible.
	CandidateReasonCoordinatorObserved CandidateReason = "coordinator-observed"

	// CandidateReasonPreviouslyVerified means this endpoint was confirmed
	// reachable in a prior probe run but has since become stale or failed.
	// Prior success is a meaningful indicator worth retrying.
	CandidateReasonPreviouslyVerified CandidateReason = "previously-verified"
)

// ProbeCandidate describes a specific targeted endpoint to probe.
//
// Every candidate must originate from a known, deliberate source. Do not
// generate candidates by scanning port ranges; that is explicitly out of
// scope and contradicts the targeted-first probing model.
type ProbeCandidate struct {
	// Host is the external address to probe (IP or hostname).
	Host string

	// Port is the external port to probe.
	Port uint16

	// Reason records why this candidate was selected. Must be a known
	// CandidateReason constant. Required for targeted-probing discipline.
	Reason CandidateReason
}

// Validate reports whether the ProbeCandidate is structurally valid.
func (c ProbeCandidate) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("probe candidate: host must be set")
	}
	if c.Port == 0 {
		return fmt.Errorf("probe candidate: port must be greater than zero")
	}
	switch c.Reason {
	case CandidateReasonConfigured, CandidateReasonRouterDiscovered,
		CandidateReasonCoordinatorObserved, CandidateReasonPreviouslyVerified:
	default:
		return fmt.Errorf("probe candidate: unknown reason %q", c.Reason)
	}
	return nil
}

// BuildCandidatesFromEndpoints derives a targeted probe candidate list from
// a slice of known ExternalEndpoints.
//
// Candidates are generated for endpoints that need verification:
//   - Unverified: newly recorded, not yet probed
//   - Stale: was valid, needs revalidation after a health or IP-change event
//   - Failed: prior probe found unreachable; retry to see if situation improved
//
// Already-verified endpoints are skipped unless includeVerified is true
// (to force a full refresh of all known endpoints).
//
// This function never invents new host or port combinations. All returned
// candidates are strictly derived from the provided endpoints. Targeted
// probing is not port scanning.
func BuildCandidatesFromEndpoints(endpoints []ExternalEndpoint, includeVerified bool) []ProbeCandidate {
	candidates := make([]ProbeCandidate, 0, len(endpoints))
	for _, ep := range endpoints {
		var reason CandidateReason
		switch ep.Verification {
		case VerificationStateStale, VerificationStateFailed:
			// Needs revalidation. Use PreviouslyVerified if ever verified,
			// otherwise fall back to the source-based reason.
			if !ep.VerifiedAt.IsZero() {
				reason = CandidateReasonPreviouslyVerified
			} else {
				reason = endpointSourceToCandidateReason(ep.Source)
			}
		case VerificationStateUnverified:
			reason = endpointSourceToCandidateReason(ep.Source)
		case VerificationStateVerified:
			if !includeVerified {
				continue
			}
			reason = CandidateReasonPreviouslyVerified
		default:
			continue
		}
		candidates = append(candidates, ProbeCandidate{
			Host:   ep.Host,
			Port:   ep.Port,
			Reason: reason,
		})
	}
	return candidates
}

// endpointSourceToCandidateReason maps an EndpointSource to the corresponding
// CandidateReason.
func endpointSourceToCandidateReason(s EndpointSource) CandidateReason {
	switch s {
	case EndpointSourceConfigured:
		return CandidateReasonConfigured
	case EndpointSourceRouterDiscovered:
		return CandidateReasonRouterDiscovered
	case EndpointSourceCoordinatorObserved:
		return CandidateReasonCoordinatorObserved
	case EndpointSourceProbeDiscovered:
		return CandidateReasonPreviouslyVerified
	default:
		return CandidateReasonConfigured
	}
}

// BuildCoordinatorObservedCandidates builds probe candidates from the
// combination of a coordinator-observed public IP and explicitly configured
// forwarded ports.
//
// This is used when the coordinator has observed this node's source address
// during the control session. The coordinator cannot infer forwarded ports
// from source-address observation alone; forwardedPorts must come from
// explicit operator configuration, not port-range guessing.
//
// Never call this with a speculative or broad port list. Candidates must
// correspond to ports the operator has explicitly declared as forwarded.
func BuildCoordinatorObservedCandidates(observedHost string, forwardedPorts []uint16) []ProbeCandidate {
	candidates := make([]ProbeCandidate, 0, len(forwardedPorts))
	for _, port := range forwardedPorts {
		candidates = append(candidates, ProbeCandidate{
			Host:   observedHost,
			Port:   port,
			Reason: CandidateReasonCoordinatorObserved,
		})
	}
	return candidates
}

// ProbeExecutor executes a targeted probe against a single candidate endpoint.
//
// Implementations must:
//   - probe only the specific candidate given (never expand to a port range)
//   - respect context deadline/cancellation
//   - return a ProbeResult for both reachable and unreachable outcomes
//   - return a non-nil error only for unexpected internal failures (e.g.,
//     nonce generation failed), not for "endpoint did not respond"
type ProbeExecutor interface {
	Execute(ctx context.Context, candidate ProbeCandidate) (ProbeResult, error)
}

// DefaultProbeTimeout is the default per-probe deadline for UDPProbeExecutor.
// Short enough to avoid blocking on unreachable endpoints, long enough for
// typical internet paths.
const DefaultProbeTimeout = 3 * time.Second

// UDPProbeExecutor implements ProbeExecutor using the Transitloom UDP
// probe protocol (challenge/response with a magic-prefixed nonce datagram).
//
// Probing steps:
//  1. Open a connected UDP socket to candidate.Host:candidate.Port.
//  2. Send a 13-byte probe request (magic + type + nonce).
//  3. Wait for a 13-byte response with matching nonce.
//  4. Valid matching response → Reachable=true; timeout or mismatch → Reachable=false.
//
// The remote endpoint must run a ProbeResponder (or integrate
// HandleProbeDatagram) to answer probe requests. UDPProbeExecutor never
// probes beyond the single candidate given by the caller.
type UDPProbeExecutor struct {
	// Timeout overrides DefaultProbeTimeout when positive.
	Timeout time.Duration
}

// Execute probes a single candidate endpoint using the UDP probe protocol.
//
// Context cancellation is respected. A cancelled context returns an error;
// a timed-out probe (no response within deadline) returns Reachable=false.
func (e UDPProbeExecutor) Execute(ctx context.Context, candidate ProbeCandidate) (ProbeResult, error) {
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}

	addr := net.JoinHostPort(candidate.Host, strconv.Itoa(int(candidate.Port)))
	probeStart := time.Now()

	// DialContext establishes a connected UDP socket. For UDP this does not
	// send anything on the wire; it just associates the socket with the
	// remote address so Read/Write can be used without specifying the peer.
	var d net.Dialer
	dialCtx, dialCancel := context.WithTimeout(ctx, timeout)
	defer dialCancel()

	conn, dialErr := d.DialContext(dialCtx, "udp", addr)
	if dialErr != nil {
		// DNS resolution failure, network error, or context cancelled.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ProbeResult{}, ctxErr
		}
		// Connection failure: endpoint address could not be reached.
		return ProbeResult{
			TargetHost: candidate.Host,
			TargetPort: candidate.Port,
			Reachable:  false,
			ProbedAt:   probeStart,
		}, nil
	}
	defer conn.Close()

	// Set an absolute deadline covering both the write and read phases.
	conn.SetDeadline(time.Now().Add(timeout))

	// Generate a random nonce for matching the request to its response.
	// crypto/rand is used to avoid predictable nonces even though this is
	// not a security-critical challenge; it avoids replay ambiguity.
	var nonceBytes [8]byte
	if _, err := cryptorand.Read(nonceBytes[:]); err != nil {
		return ProbeResult{}, fmt.Errorf("probe: generate nonce: %w", err)
	}
	nonce := binary.LittleEndian.Uint64(nonceBytes[:])

	req := encodeProbeRequest(nonce)
	if _, writeErr := conn.Write(req[:]); writeErr != nil {
		// Write failure: local socket issue. Treat as not reachable.
		return ProbeResult{
			TargetHost: candidate.Host,
			TargetPort: candidate.Port,
			Reachable:  false,
			ProbedAt:   probeStart,
		}, nil
	}

	buf := make([]byte, 64)
	n, readErr := conn.Read(buf)
	if readErr != nil {
		// Deadline exceeded or context cancelled.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ProbeResult{}, ctxErr
		}
		// No response within deadline: endpoint not reachable.
		return ProbeResult{
			TargetHost: candidate.Host,
			TargetPort: candidate.Port,
			Reachable:  false,
			ProbedAt:   probeStart,
		}, nil
	}

	// Validate the response: must be a probe response with the matching nonce.
	probeType, responseNonce, parseErr := parseProbeDatagram(buf[:n])
	if parseErr != nil || probeType != ProbeTypeResponse || responseNonce != nonce {
		// Got a datagram but it is not a valid matching probe response.
		return ProbeResult{
			TargetHost: candidate.Host,
			TargetPort: candidate.Port,
			Reachable:  false,
			ProbedAt:   probeStart,
		}, nil
	}

	return ProbeResult{
		TargetHost:    candidate.Host,
		TargetPort:    candidate.Port,
		Reachable:     true,
		RoundTripTime: time.Since(probeStart),
		ProbedAt:      probeStart,
	}, nil
}
