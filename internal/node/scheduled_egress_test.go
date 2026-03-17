package node

import (
	"context"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/scheduler"
	"github.com/zhouchenh/transitloom/internal/service"
	"github.com/zhouchenh/transitloom/internal/status"
	"github.com/zhouchenh/transitloom/internal/transport"
)

// --- Unit tests: buildPathCandidatesFromEndpoints ---

func TestBuildPathCandidatesFromEndpoints_DirectOnly(t *testing.T) {
	candidates := buildPathCandidatesFromEndpoints("assoc-1", "192.0.2.1:51830", "")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	c := candidates[0]
	if c.Class != scheduler.PathClassDirectPublic {
		t.Fatalf("expected PathClassDirectPublic, got %s", c.Class)
	}
	if c.Health != scheduler.HealthStateActive {
		t.Fatalf("expected HealthStateActive, got %s", c.Health)
	}
	if c.AssociationID != "assoc-1" {
		t.Fatalf("expected assoc-1, got %s", c.AssociationID)
	}
	if c.ID != "assoc-1:direct" {
		t.Fatalf("expected assoc-1:direct, got %s", c.ID)
	}
	if c.Quality.Measured() {
		t.Fatal("quality must be unmeasured for static config candidates")
	}
}

func TestBuildPathCandidatesFromEndpoints_RelayOnly(t *testing.T) {
	candidates := buildPathCandidatesFromEndpoints("assoc-2", "", "10.0.0.1:40001")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	c := candidates[0]
	if c.Class != scheduler.PathClassCoordinatorRelay {
		t.Fatalf("expected PathClassCoordinatorRelay, got %s", c.Class)
	}
	if c.Health != scheduler.HealthStateActive {
		t.Fatalf("expected HealthStateActive, got %s", c.Health)
	}
	if c.ID != "assoc-2:relay" {
		t.Fatalf("expected assoc-2:relay, got %s", c.ID)
	}
}

func TestBuildPathCandidatesFromEndpoints_Both(t *testing.T) {
	candidates := buildPathCandidatesFromEndpoints("assoc-3", "192.0.2.1:51830", "10.0.0.1:40001")
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	// Direct comes first.
	if candidates[0].Class != scheduler.PathClassDirectPublic {
		t.Fatalf("expected direct candidate first, got %s", candidates[0].Class)
	}
	if candidates[1].Class != scheduler.PathClassCoordinatorRelay {
		t.Fatalf("expected relay candidate second, got %s", candidates[1].Class)
	}
}

func TestBuildPathCandidatesFromEndpoints_Neither(t *testing.T) {
	candidates := buildPathCandidatesFromEndpoints("assoc-4", "", "")
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates, got %d", len(candidates))
	}
}

// --- Unit tests: scheduler integration via ScheduledEgressRuntime ---

// TestSchedulerPrefersDirectOverRelay verifies that when both a direct and a
// relay path candidate are offered, the scheduler chooses the direct path.
// This is required by the spec: relay paths incur a scoring penalty so that
// direct paths are preferred when they are healthy and competitively useful.
func TestSchedulerPrefersDirectOverRelay(t *testing.T) {
	runtime := NewScheduledEgressRuntime()
	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	candidates := []scheduler.PathCandidate{
		{
			ID:            associationID + ":direct",
			AssociationID: associationID,
			Class:         scheduler.PathClassDirectPublic,
			Health:        scheduler.HealthStateActive,
			AdminWeight:   100,
		},
		{
			ID:            associationID + ":relay",
			AssociationID: associationID,
			Class:         scheduler.PathClassCoordinatorRelay,
			Health:        scheduler.HealthStateActive,
			AdminWeight:   100,
		},
	}

	decision := runtime.Scheduler.Decide(associationID, candidates)

	if decision.Mode == scheduler.ModeNoEligiblePath {
		t.Fatalf("expected eligible path, got ModeNoEligiblePath; reason: %s", decision.Reason)
	}
	if len(decision.ChosenPaths) == 0 {
		t.Fatal("expected at least one chosen path")
	}

	best := decision.ChosenPaths[0]
	if best.Class.IsRelay() {
		t.Fatalf("scheduler should prefer direct over relay, but chose %s; reason: %s",
			best.Class, decision.Reason)
	}
}

// TestSchedulerNoEligiblePathWhenAllFailed verifies that failed-health candidates
// produce ModeNoEligiblePath.
func TestSchedulerNoEligiblePathWhenAllFailed(t *testing.T) {
	runtime := NewScheduledEgressRuntime()
	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	candidates := []scheduler.PathCandidate{
		{
			ID:            associationID + ":direct",
			AssociationID: associationID,
			Class:         scheduler.PathClassDirectPublic,
			Health:        scheduler.HealthStateFailed,
			AdminWeight:   100,
		},
		{
			ID:            associationID + ":relay",
			AssociationID: associationID,
			Class:         scheduler.PathClassCoordinatorRelay,
			Health:        scheduler.HealthStateFailed,
			AdminWeight:   100,
		},
	}

	decision := runtime.Scheduler.Decide(associationID, candidates)

	if decision.Mode != scheduler.ModeNoEligiblePath {
		t.Fatalf("expected ModeNoEligiblePath for all-failed candidates, got %s", decision.Mode)
	}
	if decision.Reason == "" {
		t.Fatal("Reason must be non-empty even for ModeNoEligiblePath")
	}
}

// TestSchedulerStripingNotActivatedForUnmeasuredPaths verifies that
// ModePerPacketStripe is not selected when all paths have unmeasured quality
// (confidence=0). The striping gate must block striping for unmeasured paths.
func TestSchedulerStripingNotActivatedForUnmeasuredPaths(t *testing.T) {
	runtime := NewScheduledEgressRuntime()
	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	// Two candidates with zero-value Quality (unmeasured).
	candidates := []scheduler.PathCandidate{
		{
			ID:            associationID + ":direct1",
			AssociationID: associationID,
			Class:         scheduler.PathClassDirectPublic,
			Health:        scheduler.HealthStateActive,
			AdminWeight:   100,
			// Quality is zero-value: unmeasured.
		},
		{
			ID:            associationID + ":direct2",
			AssociationID: associationID,
			Class:         scheduler.PathClassDirectIntranet,
			Health:        scheduler.HealthStateActive,
			AdminWeight:   100,
			// Quality is zero-value: unmeasured.
		},
	}

	decision := runtime.Scheduler.Decide(associationID, candidates)

	if decision.Mode == scheduler.ModePerPacketStripe {
		t.Fatalf("striping must not activate for unmeasured paths (confidence=0); got ModePerPacketStripe; reason: %s",
			decision.Reason)
	}
	if decision.Reason == "" {
		t.Fatal("Reason must be non-empty")
	}
}

// TestSchedulerDecisionReasonAlwaysNonEmpty verifies that every scheduler
// decision carries a non-empty Reason, regardless of outcome. This is required
// by the spec: Reason is always set for observability.
func TestSchedulerDecisionReasonAlwaysNonEmpty(t *testing.T) {
	runtime := NewScheduledEgressRuntime()
	associationID := "assoc-reason-test"

	cases := []struct {
		name       string
		candidates []scheduler.PathCandidate
	}{
		{
			name:       "no candidates",
			candidates: nil,
		},
		{
			name: "single active direct",
			candidates: []scheduler.PathCandidate{
				{
					ID:            associationID + ":direct",
					AssociationID: associationID,
					Class:         scheduler.PathClassDirectPublic,
					Health:        scheduler.HealthStateActive,
					AdminWeight:   100,
				},
			},
		},
		{
			name: "all failed",
			candidates: []scheduler.PathCandidate{
				{
					ID:            associationID + ":direct",
					AssociationID: associationID,
					Class:         scheduler.PathClassDirectPublic,
					Health:        scheduler.HealthStateFailed,
					AdminWeight:   100,
				},
			},
		},
		{
			name: "direct and relay",
			candidates: []scheduler.PathCandidate{
				{
					ID:            associationID + ":direct",
					AssociationID: associationID,
					Class:         scheduler.PathClassDirectPublic,
					Health:        scheduler.HealthStateActive,
					AdminWeight:   100,
				},
				{
					ID:            associationID + ":relay",
					AssociationID: associationID,
					Class:         scheduler.PathClassCoordinatorRelay,
					Health:        scheduler.HealthStateActive,
					AdminWeight:   100,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := runtime.Scheduler.Decide(associationID, tc.candidates)
			if decision.Reason == "" {
				t.Fatalf("Reason must be non-empty for case %q (mode=%s)", tc.name, decision.Mode)
			}
		})
	}
}

// --- Integration tests: ActivateScheduledEgress with real sockets ---

// makeDirectScheduledCfg constructs a NodeConfig for scheduled egress tests
// using a direct path. The wgPort is the local service (WireGuard) listen port.
// The ingressPort is the local ingress port for the scheduled carrier.
func makeDirectScheduledCfg(wgPort, ingressPort uint16) config.NodeConfig {
	return config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    wgPort,
				},
				Ingress: &config.ServiceIngressConfig{
					Mode:       config.IngressModeStatic,
					StaticPort: ingressPort,
				},
			},
		},
	}
}

// TestActivateScheduledEgress_DirectChosen verifies that when the scheduler
// chooses a direct path, the direct carrier is activated and CarrierActivated="direct".
func TestActivateScheduledEgress_DirectChosen(t *testing.T) {
	// Allocate real sockets.
	wgConn, wgAddr := allocLoopbackUDP(t)
	defer wgConn.Close()

	meshPreConn, meshPreAddr := allocLoopbackUDP(t)
	meshPreConn.Close()

	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	cfg := makeDirectScheduledCfg(uint16(wgAddr.Port), uint16(ingressPreAddr.Port))

	rt := NewScheduledEgressRuntime()
	defer rt.Direct.Carrier.StopAll()
	defer rt.Relay.Carrier.StopAll()

	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	// Direct-only candidate: scheduler must choose direct.
	inputs := []ScheduledActivationInput{
		{
			AssociationID:  associationID,
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshPreAddr.String(),
			MeshListenPort: uint16(meshPreAddr.Port),
			PathCandidates: buildPathCandidatesFromEndpoints(associationID, meshPreAddr.String(), ""),
		},
	}

	ctx := context.Background()
	result := ActivateScheduledEgress(ctx, cfg, rt, inputs)

	if result.TotalActive != 1 {
		t.Fatalf("expected 1 active, got active=%d failed=%d no-eligible=%d",
			result.TotalActive, result.TotalFailed, result.TotalNoEligible)
	}

	act := result.Activations[0]
	if act.CarrierActivated != "direct" {
		t.Fatalf("expected carrier=direct, got %q (reason: %s)", act.CarrierActivated, act.Decision.Reason)
	}
	if act.ActivationError != "" {
		t.Fatalf("unexpected activation error: %s", act.ActivationError)
	}
	if act.Decision.Reason == "" {
		t.Fatal("decision Reason must be non-empty")
	}
}

// TestActivateScheduledEgress_NoEligiblePath verifies that an input with only
// failed-health candidates results in TotalNoEligible=1 and CarrierActivated="none".
func TestActivateScheduledEgress_NoEligiblePath(t *testing.T) {
	wgConn, wgAddr := allocLoopbackUDP(t)
	defer wgConn.Close()

	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	cfg := makeDirectScheduledCfg(uint16(wgAddr.Port), uint16(ingressPreAddr.Port))

	rt := NewScheduledEgressRuntime()
	defer rt.Direct.Carrier.StopAll()
	defer rt.Relay.Carrier.StopAll()

	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	// All candidates are failed — scheduler must return ModeNoEligiblePath.
	inputs := []ScheduledActivationInput{
		{
			AssociationID: associationID,
			SourceNode:    "node-a",
			SourceService: service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:      "node-b",
			DestService:   service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			PathCandidates: []scheduler.PathCandidate{
				{
					ID:            associationID + ":direct",
					AssociationID: associationID,
					Class:         scheduler.PathClassDirectPublic,
					Health:        scheduler.HealthStateFailed,
					AdminWeight:   100,
				},
			},
		},
	}

	ctx := context.Background()
	result := ActivateScheduledEgress(ctx, cfg, rt, inputs)

	if result.TotalNoEligible != 1 {
		t.Fatalf("expected TotalNoEligible=1, got active=%d failed=%d no-eligible=%d",
			result.TotalActive, result.TotalFailed, result.TotalNoEligible)
	}

	act := result.Activations[0]
	if act.CarrierActivated != "none" {
		t.Fatalf("expected carrier=none for ModeNoEligiblePath, got %q", act.CarrierActivated)
	}
	if act.Decision.Mode != scheduler.ModeNoEligiblePath {
		t.Fatalf("expected ModeNoEligiblePath, got %s", act.Decision.Mode)
	}
	if act.Decision.Reason == "" {
		t.Fatal("decision Reason must be non-empty even for ModeNoEligiblePath")
	}
}

// TestActivateScheduledEgress_RelayOnlyPath verifies that when only a relay
// endpoint is configured, the scheduler picks the relay path and the relay
// egress carrier is activated (CarrierActivated="relay").
func TestActivateScheduledEgress_RelayOnlyPath(t *testing.T) {
	// Set up a relay endpoint (loopback coordinator relay simulation).
	relayConn, relayAddr := allocLoopbackUDP(t)
	defer relayConn.Close()

	// WireGuard local target (service binding port).
	wgConn, wgAddr := allocLoopbackUDP(t)
	defer wgConn.Close()

	// Local ingress port for the relay egress carrier.
	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	cfg := makeDirectScheduledCfg(uint16(wgAddr.Port), uint16(ingressPreAddr.Port))

	rt := NewScheduledEgressRuntime()
	defer rt.Direct.Carrier.StopAll()
	defer rt.Relay.Carrier.StopAll()

	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	// Relay-only candidate: scheduler must choose relay.
	inputs := []ScheduledActivationInput{
		{
			AssociationID: associationID,
			SourceNode:    "node-a",
			SourceService: service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:      "node-b",
			DestService:   service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			RelayEndpoint: relayAddr.String(),
			PathCandidates: buildPathCandidatesFromEndpoints(associationID, "", relayAddr.String()),
		},
	}

	ctx := context.Background()
	result := ActivateScheduledEgress(ctx, cfg, rt, inputs)

	if result.TotalActive != 1 {
		t.Fatalf("expected 1 active relay activation, got active=%d failed=%d no-eligible=%d",
			result.TotalActive, result.TotalFailed, result.TotalNoEligible)
	}

	act := result.Activations[0]
	if act.CarrierActivated != "relay" {
		t.Fatalf("expected carrier=relay, got %q (reason: %s)", act.CarrierActivated, act.Decision.Reason)
	}
	if act.ActivationError != "" {
		t.Fatalf("unexpected activation error: %s", act.ActivationError)
	}
}

// TestActivateScheduledEgress_StripingNotActivated verifies that when the
// scheduler is given two unmeasured paths, the mode is not ModePerPacketStripe,
// and activation proceeds with the best single path ("direct" here).
func TestActivateScheduledEgress_StripingNotActivated(t *testing.T) {
	wgConn, wgAddr := allocLoopbackUDP(t)
	defer wgConn.Close()

	meshPreConn, meshPreAddr := allocLoopbackUDP(t)
	meshPreConn.Close()

	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	cfg := makeDirectScheduledCfg(uint16(wgAddr.Port), uint16(ingressPreAddr.Port))

	rt := NewScheduledEgressRuntime()
	defer rt.Direct.Carrier.StopAll()
	defer rt.Relay.Carrier.StopAll()

	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	// Two candidates: direct + relay, both unmeasured.
	inputs := []ScheduledActivationInput{
		{
			AssociationID:  associationID,
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshPreAddr.String(),
			MeshListenPort: uint16(meshPreAddr.Port),
			PathCandidates: buildPathCandidatesFromEndpoints(associationID, meshPreAddr.String(), "10.0.0.1:40001"),
		},
	}

	ctx := context.Background()
	result := ActivateScheduledEgress(ctx, cfg, rt, inputs)

	if len(result.Activations) != 1 {
		t.Fatalf("expected 1 activation, got %d", len(result.Activations))
	}

	act := result.Activations[0]
	if act.Decision.Mode == scheduler.ModePerPacketStripe {
		t.Fatalf("per-packet striping must not activate for unmeasured paths; reason: %s", act.Decision.Reason)
	}
	// Scheduler should not have returned no-eligible when a direct active path exists.
	if act.Decision.Mode == scheduler.ModeNoEligiblePath {
		t.Fatalf("unexpected ModeNoEligiblePath; reason: %s", act.Decision.Reason)
	}
}

// TestActivateScheduledEgress_DecisionAlignedWithActivation verifies that the
// Decision.Mode and CarrierActivated fields are consistent:
//   - ModeNoEligiblePath → CarrierActivated="none"
//   - other mode + direct best path → CarrierActivated="direct"
//   - other mode + relay best path → CarrierActivated="relay"
func TestActivateScheduledEgress_DecisionAlignedWithActivation(t *testing.T) {
	wgConn, wgAddr := allocLoopbackUDP(t)
	defer wgConn.Close()

	meshPreConn, meshPreAddr := allocLoopbackUDP(t)
	meshPreConn.Close()

	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	cfg := makeDirectScheduledCfg(uint16(wgAddr.Port), uint16(ingressPreAddr.Port))

	rt := NewScheduledEgressRuntime()
	defer rt.Direct.Carrier.StopAll()
	defer rt.Relay.Carrier.StopAll()

	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	inputs := []ScheduledActivationInput{
		{
			AssociationID:  associationID,
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshPreAddr.String(),
			MeshListenPort: uint16(meshPreAddr.Port),
			PathCandidates: buildPathCandidatesFromEndpoints(associationID, meshPreAddr.String(), ""),
		},
	}

	ctx := context.Background()
	result := ActivateScheduledEgress(ctx, cfg, rt, inputs)

	if len(result.Activations) != 1 {
		t.Fatalf("expected 1 activation, got %d", len(result.Activations))
	}

	act := result.Activations[0]

	// Verify alignment rule.
	switch {
	case act.Decision.Mode == scheduler.ModeNoEligiblePath:
		if act.CarrierActivated != "none" {
			t.Fatalf("alignment violation: ModeNoEligiblePath but CarrierActivated=%q", act.CarrierActivated)
		}
	case act.CarrierActivated == "direct":
		// Verify the decision's chosen path is not a relay.
		if len(act.Decision.ChosenPaths) > 0 && act.Decision.ChosenPaths[0].Class.IsRelay() {
			t.Fatalf("alignment violation: CarrierActivated=direct but ChosenPaths[0].Class=%s",
				act.Decision.ChosenPaths[0].Class)
		}
	case act.CarrierActivated == "relay":
		// Verify the decision's chosen path is a relay.
		if len(act.Decision.ChosenPaths) > 0 && !act.Decision.ChosenPaths[0].Class.IsRelay() {
			t.Fatalf("alignment violation: CarrierActivated=relay but ChosenPaths[0].Class=%s",
				act.Decision.ChosenPaths[0].Class)
		}
	default:
		if act.ActivationError == "" {
			t.Fatalf("unexpected CarrierActivated=%q with no error and non-no-eligible mode=%s",
				act.CarrierActivated, act.Decision.Mode)
		}
	}

	// Report lines must be non-empty.
	lines := result.ReportLines()
	if len(lines) == 0 {
		t.Fatal("expected non-empty report lines")
	}
}

// TestBuildScheduledActivationInputs_FiltersControlPlaneOnly verifies that
// associations with neither direct nor relay endpoints are excluded (they are
// control-plane records only, no data-plane path configured).
func TestBuildScheduledActivationInputs_FiltersControlPlaneOnly(t *testing.T) {
	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    51820,
				},
			},
		},
		Associations: []config.AssociationConfig{
			{
				SourceService:      "wg0",
				DestinationNode:    "node-b",
				DestinationService: "wg0",
				DirectEndpoint:     "192.0.2.1:51830",
				MeshListenPort:     51830,
			},
			{
				SourceService:      "wg0",
				DestinationNode:    "node-c",
				DestinationService: "wg0",
				// No endpoints — control-plane record only; must be skipped.
			},
		},
	}

	results := []AssociationResultEntry{
		{
			AssociationID:      "node-a/wg0:raw-udp->node-b/wg0:raw-udp",
			SourceServiceName:  "wg0",
			DestinationNode:    "node-b",
			DestinationService: "wg0",
			Accepted:           true,
		},
		{
			AssociationID:      "node-a/wg0:raw-udp->node-c/wg0:raw-udp",
			SourceServiceName:  "wg0",
			DestinationNode:    "node-c",
			DestinationService: "wg0",
			Accepted:           true,
		},
	}

	inputs := BuildScheduledActivationInputs(cfg, results)

	if len(inputs) != 1 {
		t.Fatalf("expected 1 input (only the one with a direct_endpoint), got %d", len(inputs))
	}
	if inputs[0].AssociationID != "node-a/wg0:raw-udp->node-b/wg0:raw-udp" {
		t.Fatalf("wrong association ID: %s", inputs[0].AssociationID)
	}
	if inputs[0].DirectEndpoint != "192.0.2.1:51830" {
		t.Fatalf("wrong direct endpoint: %s", inputs[0].DirectEndpoint)
	}
	// PathCandidates must be populated.
	if len(inputs[0].PathCandidates) == 0 {
		t.Fatal("PathCandidates must be non-empty for inputs with a direct endpoint")
	}
}

// TestBuildScheduledActivationInputs_BothEndpoints verifies that when both
// direct and relay endpoints are configured, both PathCandidates are present
// so the scheduler can choose between them.
func TestBuildScheduledActivationInputs_BothEndpoints(t *testing.T) {
	cfg := config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "node-a"},
		Services: []config.ServiceConfig{
			{
				Name: "wg0",
				Type: config.ServiceTypeRawUDP,
				Binding: config.ServiceBindingConfig{
					Address: "127.0.0.1",
					Port:    51820,
				},
			},
		},
		Associations: []config.AssociationConfig{
			{
				SourceService:      "wg0",
				DestinationNode:    "node-b",
				DestinationService: "wg0",
				DirectEndpoint:     "192.0.2.1:51830",
				MeshListenPort:     51830,
				RelayEndpoint:      "10.0.0.1:40001",
			},
		},
	}

	results := []AssociationResultEntry{
		{
			AssociationID:      "node-a/wg0:raw-udp->node-b/wg0:raw-udp",
			SourceServiceName:  "wg0",
			DestinationNode:    "node-b",
			DestinationService: "wg0",
			Accepted:           true,
		},
	}

	inputs := BuildScheduledActivationInputs(cfg, results)

	if len(inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(inputs))
	}

	inp := inputs[0]
	if inp.DirectEndpoint != "192.0.2.1:51830" {
		t.Fatalf("wrong direct endpoint: %s", inp.DirectEndpoint)
	}
	if inp.RelayEndpoint != "10.0.0.1:40001" {
		t.Fatalf("wrong relay endpoint: %s", inp.RelayEndpoint)
	}

	// Must have two candidates: one direct, one relay.
	if len(inp.PathCandidates) != 2 {
		t.Fatalf("expected 2 PathCandidates (direct+relay), got %d", len(inp.PathCandidates))
	}
	hasDirectCandidate := false
	hasRelayCandidate := false
	for _, c := range inp.PathCandidates {
		if c.Class == scheduler.PathClassDirectPublic {
			hasDirectCandidate = true
		}
		if c.Class == scheduler.PathClassCoordinatorRelay {
			hasRelayCandidate = true
		}
	}
	if !hasDirectCandidate {
		t.Fatal("expected a direct PathCandidate")
	}
	if !hasRelayCandidate {
		t.Fatal("expected a relay PathCandidate")
	}
}

// --- QualityStore integration tests ---

// TestScheduledEgressRuntime_QualityStore_InitiallyEmpty verifies that a fresh
// ScheduledEgressRuntime has a non-nil QualityStore with no entries. This
// ensures measurement is ready to use but does not pre-populate fake quality.
func TestScheduledEgressRuntime_QualityStore_InitiallyEmpty(t *testing.T) {
	runtime := NewScheduledEgressRuntime()
	if runtime.QualityStore == nil {
		t.Fatal("QualityStore must be non-nil after NewScheduledEgressRuntime")
	}
	snaps := runtime.QualityStore.Snapshot()
	if len(snaps) != 0 {
		t.Errorf("QualityStore should start empty, got %d entries", len(snaps))
	}
}

// TestScheduledEgressRuntime_QualitySnapshot_Empty verifies QualitySnapshot
// returns an empty summary when no measurements have been recorded.
func TestScheduledEgressRuntime_QualitySnapshot_Empty(t *testing.T) {
	runtime := NewScheduledEgressRuntime()
	summary := runtime.QualitySnapshot()
	if summary.TotalMeasured != 0 || summary.TotalStale != 0 {
		t.Errorf("QualitySnapshot should be empty before any measurement, got %+v", summary)
	}
}

// TestScheduledEgressRuntime_QualityStoreApplied verifies that when the QualityStore
// has a fresh measurement, ApplyCandidates enriches the candidates before Decide().
// This tests the measurement-to-scheduler integration path.
func TestScheduledEgressRuntime_QualityStoreApplied(t *testing.T) {
	runtime := NewScheduledEgressRuntime()

	// Record quality for the direct path of "assoc-q1".
	runtime.QualityStore.Update(
		"assoc-q1:direct",
		scheduler.PathQuality{RTT: 10 * time.Millisecond, Confidence: 0.8},
	)

	// Build candidates: one direct (measured), one relay (unmeasured).
	candidates := []scheduler.PathCandidate{
		{
			ID:            "assoc-q1:direct",
			AssociationID: "assoc-q1",
			Class:         scheduler.PathClassDirectPublic,
			Health:        scheduler.HealthStateActive,
		},
		{
			ID:            "assoc-q1:relay",
			AssociationID: "assoc-q1",
			Class:         scheduler.PathClassCoordinatorRelay,
			Health:        scheduler.HealthStateActive,
		},
	}

	// ApplyCandidates should enrich the direct candidate with stored quality.
	enriched := runtime.QualityStore.ApplyCandidates(candidates)

	directEnriched := enriched[0]
	if !directEnriched.Quality.Measured() {
		t.Error("direct candidate should have measured quality after ApplyCandidates")
	}
	if directEnriched.Quality.RTT != 10*time.Millisecond {
		t.Errorf("direct candidate RTT: got %v want 10ms", directEnriched.Quality.RTT)
	}
	if directEnriched.Quality.Confidence != 0.8 {
		t.Errorf("direct candidate Confidence: got %v want 0.8", directEnriched.Quality.Confidence)
	}

	// Relay candidate has no measurement — should remain unmeasured.
	relayEnriched := enriched[1]
	if relayEnriched.Quality.Measured() {
		t.Error("relay candidate should remain unmeasured (no quality recorded in store)")
	}
}

// --- CandidateStore and EndpointRegistry integration tests ---

// TestScheduledEgressRuntime_CandidateStore_NilByDefault verifies that
// NewScheduledEgressRuntime does not pre-populate CandidateStore or
// EndpointRegistry. Callers must explicitly wire these in when needed.
// Nil = prior behavior (config-only candidates), which must remain correct.
func TestScheduledEgressRuntime_CandidateStore_NilByDefault(t *testing.T) {
	rt := NewScheduledEgressRuntime()
	if rt.CandidateStore != nil {
		t.Error("CandidateStore must be nil by default; callers wire it in explicitly")
	}
	if rt.EndpointRegistry != nil {
		t.Error("EndpointRegistry must be nil by default; callers wire it in explicitly")
	}
}

// TestActivateScheduledEgress_InformationalDistributedCandidates verifies that
// informational distributed candidates (no RemoteEndpoint) are excluded by the
// refinement layer and do not interfere with config-derived candidate scoring.
// The config-derived direct path must still be chosen.
func TestActivateScheduledEgress_InformationalDistributedCandidates(t *testing.T) {
	wgConn, wgAddr := allocLoopbackUDP(t)
	defer wgConn.Close()

	meshPreConn, meshPreAddr := allocLoopbackUDP(t)
	meshPreConn.Close()

	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	cfg := makeDirectScheduledCfg(uint16(wgAddr.Port), uint16(ingressPreAddr.Port))
	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	// Store informational-only distributed candidates (no RemoteEndpoint).
	// These must be excluded by RefineCandidates and must not break the config path.
	store := NewCandidateStore()
	store.Store(associationID, []controlplane.DistributedPathCandidate{
		{
			CandidateID:   "dist-c1",
			AssociationID: associationID,
			Class:         controlplane.DistributedPathClassDirectPublic,
			// No RemoteEndpoint: informational only.
			Note: "direct candidate pending node endpoint advertisement",
		},
	})

	rt := NewScheduledEgressRuntime()
	rt.CandidateStore = store
	defer rt.Direct.Carrier.StopAll()
	defer rt.Relay.Carrier.StopAll()

	inputs := []ScheduledActivationInput{
		{
			AssociationID:  associationID,
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshPreAddr.String(),
			MeshListenPort: uint16(meshPreAddr.Port),
			PathCandidates: buildPathCandidatesFromEndpoints(associationID, meshPreAddr.String(), ""),
		},
	}

	ctx := context.Background()
	result := ActivateScheduledEgress(ctx, cfg, rt, inputs)

	if result.TotalActive != 1 {
		t.Fatalf("informational distributed candidates must not block config direct path; "+
			"got active=%d failed=%d no-eligible=%d reason=%q",
			result.TotalActive, result.TotalFailed, result.TotalNoEligible,
			func() string {
				if len(result.Activations) > 0 {
					return result.Activations[0].Decision.Reason
				}
				return "(no activations)"
			}())
	}

	act := result.Activations[0]
	if act.CarrierActivated != "direct" {
		t.Fatalf("expected direct carrier, got %q (reason: %s)", act.CarrierActivated, act.Decision.Reason)
	}
}

// TestActivateScheduledEgress_FailedEndpointDistributedExcluded verifies that
// when the EndpointRegistry marks a distributed candidate's endpoint as failed,
// the refinement layer excludes it. The config-derived direct path is still chosen.
func TestActivateScheduledEgress_FailedEndpointDistributedExcluded(t *testing.T) {
	wgConn, wgAddr := allocLoopbackUDP(t)
	defer wgConn.Close()

	meshPreConn, meshPreAddr := allocLoopbackUDP(t)
	meshPreConn.Close()

	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	cfg := makeDirectScheduledCfg(uint16(wgAddr.Port), uint16(ingressPreAddr.Port))
	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	// Distributed candidate with a remote endpoint marked failed in the registry.
	// The refinement layer must exclude it (Usable=false, failed endpoint).
	distributedEndpoint := "9.9.9.9:4500"
	store := NewCandidateStore()
	store.Store(associationID, []controlplane.DistributedPathCandidate{
		{
			CandidateID:   "dist-direct-failed",
			AssociationID: associationID,
			Class:         controlplane.DistributedPathClassDirectPublic,
			RemoteEndpoint: distributedEndpoint,
			AdminWeight:   100,
		},
	})

	// Registry marks the distributed endpoint as failed.
	registry := transport.NewEndpointRegistry()
	registry.Add(transport.ExternalEndpoint{
		Host:         "9.9.9.9",
		Port:         4500,
		Source:       transport.EndpointSourceConfigured,
		Verification: transport.VerificationStateFailed,
		RecordedAt:   time.Now(),
		StaleAt:      time.Now(),
	})

	rt := NewScheduledEgressRuntime()
	rt.CandidateStore = store
	rt.EndpointRegistry = registry
	defer rt.Direct.Carrier.StopAll()
	defer rt.Relay.Carrier.StopAll()

	// Config-derived direct candidate is healthy — must be chosen.
	inputs := []ScheduledActivationInput{
		{
			AssociationID:  associationID,
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshPreAddr.String(),
			MeshListenPort: uint16(meshPreAddr.Port),
			PathCandidates: buildPathCandidatesFromEndpoints(associationID, meshPreAddr.String(), ""),
		},
	}

	ctx := context.Background()
	result := ActivateScheduledEgress(ctx, cfg, rt, inputs)

	if result.TotalActive != 1 {
		t.Fatalf("failed distributed candidate must not prevent config direct path from being chosen; "+
			"got active=%d failed=%d no-eligible=%d reason=%q",
			result.TotalActive, result.TotalFailed, result.TotalNoEligible,
			func() string {
				if len(result.Activations) > 0 {
					return result.Activations[0].Decision.Reason
				}
				return "(no activations)"
			}())
	}

	act := result.Activations[0]
	if act.CarrierActivated != "direct" {
		t.Fatalf("expected direct carrier (config-derived), got %q (reason: %s)",
			act.CarrierActivated, act.Decision.Reason)
	}
}

// TestActivateScheduledEgress_StaleEndpointDistributedDegraded verifies that
// a distributed candidate with a stale endpoint is degraded (not excluded) by
// the refinement layer. The config-derived direct path (active health) must
// still be preferred by the scheduler over the degraded distributed candidate.
func TestActivateScheduledEgress_StaleEndpointDistributedDegraded(t *testing.T) {
	wgConn, wgAddr := allocLoopbackUDP(t)
	defer wgConn.Close()

	meshPreConn, meshPreAddr := allocLoopbackUDP(t)
	meshPreConn.Close()

	ingressPreConn, ingressPreAddr := allocLoopbackUDP(t)
	ingressPreConn.Close()

	cfg := makeDirectScheduledCfg(uint16(wgAddr.Port), uint16(ingressPreAddr.Port))
	associationID := "node-a/wg0:raw-udp->node-b/wg0:raw-udp"

	// Distributed candidate with a stale endpoint — will be degraded, not excluded.
	store := NewCandidateStore()
	store.Store(associationID, []controlplane.DistributedPathCandidate{
		{
			CandidateID:    "dist-direct-stale",
			AssociationID:  associationID,
			Class:          controlplane.DistributedPathClassDirectPublic,
			RemoteEndpoint: "9.9.9.9:4500",
			AdminWeight:    100,
		},
	})

	registry := transport.NewEndpointRegistry()
	registry.Add(transport.ExternalEndpoint{
		Host:         "9.9.9.9",
		Port:         4500,
		Source:       transport.EndpointSourceConfigured,
		Verification: transport.VerificationStateStale,
		RecordedAt:   time.Now(),
		StaleAt:      time.Now(),
	})

	rt := NewScheduledEgressRuntime()
	rt.CandidateStore = store
	rt.EndpointRegistry = registry
	defer rt.Direct.Carrier.StopAll()
	defer rt.Relay.Carrier.StopAll()

	// Config-derived direct candidate is active — scheduler must prefer it over
	// the degraded distributed candidate (degradedPenalty in scheduler scoring).
	inputs := []ScheduledActivationInput{
		{
			AssociationID:  associationID,
			SourceNode:     "node-a",
			SourceService:  service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:       "node-b",
			DestService:    service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DirectEndpoint: meshPreAddr.String(),
			MeshListenPort: uint16(meshPreAddr.Port),
			PathCandidates: buildPathCandidatesFromEndpoints(associationID, meshPreAddr.String(), ""),
		},
	}

	ctx := context.Background()
	result := ActivateScheduledEgress(ctx, cfg, rt, inputs)

	// The config direct path (active health) must be activated.
	// The stale distributed candidate is degraded but still usable as a fallback.
	if result.TotalActive != 1 {
		t.Fatalf("stale distributed candidate must not prevent config direct activation; "+
			"got active=%d failed=%d no-eligible=%d",
			result.TotalActive, result.TotalFailed, result.TotalNoEligible)
	}

	act := result.Activations[0]
	if act.CarrierActivated != "direct" {
		t.Fatalf("expected direct carrier (active, config-derived) preferred over stale; "+
			"got %q (reason: %s)", act.CarrierActivated, act.Decision.Reason)
	}
}

// TestActivateScheduledEgress_DistributedQualityEnrichment verifies that when
// a CandidateStore has usable distributed candidates and the QualityStore has
// fresh measurements for those candidates, the quality is applied before the
// scheduler makes its decision. This tests the three-layer integration:
// CandidateStore → RefineCandidates → QualityStore → scheduler.
func TestActivateScheduledEgress_DistributedQualityEnrichment(t *testing.T) {
	associationID := "assoc-quality-test"

	store := NewCandidateStore()
	store.Store(associationID, []controlplane.DistributedPathCandidate{
		{
			CandidateID:    "dist-c-measured",
			AssociationID:  associationID,
			Class:          controlplane.DistributedPathClassDirectPublic,
			RemoteEndpoint: "1.2.3.4:4500",
			AdminWeight:    100,
		},
		{
			CandidateID:     "dist-c-relay",
			AssociationID:   associationID,
			Class:           controlplane.DistributedPathClassCoordinatorRelay,
			IsRelayAssisted: true,
			RemoteEndpoint:  "5.6.7.8:5000",
			RelayNodeID:     "relay-1",
			AdminWeight:     100,
		},
	})

	// Record fresh quality for the direct distributed candidate.
	store2 := scheduler.NewPathQualityStore(scheduler.DefaultQualityMaxAge)
	store2.RecordProbeResult("dist-c-measured", 15*time.Millisecond, true)

	rt := NewScheduledEgressRuntime()
	rt.CandidateStore = store
	rt.QualityStore = store2
	defer rt.Direct.Carrier.StopAll()
	defer rt.Relay.Carrier.StopAll()

	// Input with no config-derived PathCandidates: scheduler must use distributed candidates.
	// The quality-enriched direct candidate must be preferred over the relay.
	inputs := []ScheduledActivationInput{
		{
			AssociationID: associationID,
			SourceNode:    "node-a",
			SourceService: service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			DestNode:      "node-b",
			DestService:   service.Identity{Name: "wg0", Type: config.ServiceTypeRawUDP},
			// PathCandidates empty: no config-derived candidates.
			// The scheduler will only see distributed refined candidates.
		},
	}

	ctx := context.Background()
	result := ActivateScheduledEgress(ctx, cfg_stub(), rt, inputs)

	// The scheduler must have decided on a path (not ModeNoEligiblePath).
	if len(result.Activations) == 0 {
		t.Fatal("expected at least one activation result")
	}
	act := result.Activations[0]

	// The scheduler should have found at least one eligible path from the
	// distributed candidates (even if carrier activation fails because
	// input.DirectEndpoint is not wired to distributed RemoteEndpoint yet).
	if act.Decision.Mode == scheduler.ModeNoEligiblePath {
		t.Fatalf("scheduler must find eligible distributed candidates; "+
			"got ModeNoEligiblePath (reason: %s)", act.Decision.Reason)
	}

	// Verify the quality was enriched: the direct candidate should have been
	// preferred (direct class, measured quality, no relay penalty).
	if len(act.Decision.ChosenPaths) == 0 {
		t.Fatal("expected chosen paths in scheduler decision")
	}
	best := act.Decision.ChosenPaths[0]
	if best.Class.IsRelay() {
		t.Errorf("quality-enriched direct candidate should be preferred over relay; "+
			"scheduler chose class=%s (reason: %s)", best.Class, act.Decision.Reason)
	}
	// Verify quality was applied: the direct candidate ID should be the one
	// that had a probe result recorded. Quality enrichment caused it to score
	// higher than the unmeasured relay candidate.
	if best.CandidateID != "dist-c-measured" {
		t.Errorf("expected quality-enriched direct candidate 'dist-c-measured' chosen; "+
			"got %q (class=%s, reason: %s)", best.CandidateID, best.Class, act.Decision.Reason)
	}
}

// cfg_stub returns a minimal NodeConfig for tests that don't need real service config.
func cfg_stub() config.NodeConfig {
	return config.NodeConfig{
		Identity: config.IdentityMetadata{Name: "test-node"},
	}
}

// TestScheduledEgressRuntime_QualityStore_MeasurementDistinctFromCandidate verifies
// that measurement state is independent of candidate existence and scheduler decisions.
// Adding quality to the store does not create or modify PathCandidates.
func TestScheduledEgressRuntime_QualityStore_MeasurementDistinctFromCandidate(t *testing.T) {
	runtime := NewScheduledEgressRuntime()

	// Record quality for a path that has no corresponding PathCandidate.
	runtime.QualityStore.Update(
		"nonexistent-assoc:direct",
		scheduler.PathQuality{RTT: 20 * time.Millisecond, Confidence: 0.5},
	)

	// The store knows about the quality, but this does not create a candidate.
	snaps := runtime.QualityStore.Snapshot()
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot entry, got %d", len(snaps))
	}

	// Decide with no candidates: ModeNoEligiblePath regardless of quality store contents.
	decision := runtime.Scheduler.Decide("nonexistent-assoc", nil)
	if decision.Mode != scheduler.ModeNoEligiblePath {
		t.Errorf("Decide with nil candidates should return ModeNoEligiblePath, got %s", decision.Mode)
	}

	// Measurement state in the store is NOT the same as candidate existence.
	// An operator can inspect both separately.
	qualSummary := runtime.QualitySnapshot()
	if qualSummary.TotalMeasured != 1 {
		t.Errorf("QualitySnapshot should show 1 measured entry, got %d", qualSummary.TotalMeasured)
	}
}

func TestRecordActivationEvents(t *testing.T) {
	history := status.NewEventHistory(10)

	oldActs := []ScheduledEgressActivation{
		{
			AssociationID:    "assoc-1",
			CarrierActivated: "direct",
			Decision: scheduler.SchedulerDecision{
				Reason: "direct is good",
				ChosenPaths: []scheduler.ChosenPath{
					{CandidateID: "cand-A"},
				},
			},
			Candidates: []status.PathCandidateStatus{
				{ID: "cand-A", Usable: true, EndpointState: "usable"},
				{ID: "cand-B", Usable: true, EndpointState: "stale"},
			},
		},
	}

	newActs := []ScheduledEgressActivation{
		{
			AssociationID:    "assoc-1",
			CarrierActivated: "relay", // Fallback to relay
			Decision: scheduler.SchedulerDecision{
				Reason: "direct failed, falling back to relay",
				ChosenPaths: []scheduler.ChosenPath{
					{CandidateID: "cand-C"},
				},
			},
			Candidates: []status.PathCandidateStatus{
				{ID: "cand-A", Usable: false, ExcludeReason: "endpoint failed", EndpointState: "failed"},
				{ID: "cand-B", Usable: true, EndpointState: "usable"},
			},
		},
	}

	recordActivationEvents(history, oldActs, newActs)

	events := history.Snapshot()
	if len(events) == 0 {
		t.Fatal("expected events to be recorded")
	}

	var fallbackFound, excludedFound, stateStaleToUsable, stateUsableToFailed bool
	for _, e := range events {
		switch e.Type {
		case status.EventFallbackToRelay:
			fallbackFound = true
		case status.EventCandidateExcluded:
			excludedFound = true
		case status.EventEndpointVerified:
			stateStaleToUsable = true
		case status.EventEndpointFailed:
			stateUsableToFailed = true
		}
	}

	if !fallbackFound {
		t.Error("missing EventFallbackToRelay")
	}
	if !excludedFound {
		t.Error("missing EventCandidateExcluded")
	}
	if !stateStaleToUsable {
		t.Error("missing EventEndpointVerified for stale->usable transition")
	}
	if !stateUsableToFailed {
		t.Error("missing EventEndpointFailed for usable->failed transition")
	}
}

func TestStartProbeLoop_BlockedWithoutEndpointRegistry(t *testing.T) {
	rt := NewScheduledEgressRuntime()
	rt.EndpointRegistry = nil

	inputs := []ScheduledActivationInput{
		{AssociationID: "assoc-1"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := rt.StartProbeLoop(ctx, DefaultProbeSchedulerConfig(), inputs, transport.UDPProbeExecutor{})
	if started {
		t.Fatal("expected probe loop start to fail without endpoint registry")
	}

	summary := rt.Snapshot()
	if summary.ProbeLoop.State != "blocked" {
		t.Fatalf("expected probe loop state=blocked, got %q", summary.ProbeLoop.State)
	}
	if summary.ProbeLoop.Reason == "" {
		t.Fatal("expected blocked reason to be populated")
	}
}

func TestStartProbeLoop_ReportsWaitingWhenNoTargets(t *testing.T) {
	rt := NewScheduledEgressRuntime()
	rt.EndpointRegistry = transport.NewEndpointRegistry()

	inputs := []ScheduledActivationInput{
		{AssociationID: "assoc-1"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := rt.StartProbeLoop(
		ctx,
		ProbeSchedulerConfig{ProbeInterval: 10 * time.Millisecond, MaxTargetsPerRound: 1},
		inputs,
		newFakeProbeExecutor(),
	)
	if !started {
		t.Fatal("expected probe loop to start")
	}

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		summary := rt.Snapshot()
		if summary.ProbeLoop.State == "waiting-prerequisites" {
			if summary.ProbeLoop.LastRound.TargetsSelected != 0 {
				t.Fatalf("expected zero selected targets, got %d", summary.ProbeLoop.LastRound.TargetsSelected)
			}
			if summary.ProbeLoop.LastRoundAt.IsZero() {
				t.Fatal("expected last round timestamp to be set")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected probe loop state to reach waiting-prerequisites, got %q", rt.Snapshot().ProbeLoop.State)
}
