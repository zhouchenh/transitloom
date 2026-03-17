package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/dataplane"
	"github.com/zhouchenh/transitloom/internal/service"
)

// RelayPathRuntime holds the relay egress state for a source node using
// coordinator relay-assisted carriage.
//
// This runtime handles only the source node's relay egress path:
//
//	local app → local ingress → coordinator relay
//
// The destination node's delivery path (coordinator relay → local target)
// uses the existing DirectCarrier.StartDelivery infrastructure unchanged,
// because delivery behavior is functionally identical whether packets arrived
// via direct path or coordinator relay.
//
// Relay-assisted carriage and direct carriage are kept separate:
//   - direct: DirectPathRuntime (ForwardingTable + DirectCarrier)
//   - relay egress: RelayPathRuntime (RelayEgressTable + RelayEgressCarrier)
//
// This runtime does not implement scheduler logic, multi-WAN, encrypted
// carriage, or dynamic relay selection. Coordinator relay is the only relay
// role implemented in v1 T-0010; node relay is not yet implemented.
type RelayPathRuntime struct {
	Table   *dataplane.RelayEgressTable
	Carrier *dataplane.RelayEgressCarrier
}

// NewRelayPathRuntime creates a new relay-path runtime with an empty table
// and carrier.
func NewRelayPathRuntime() *RelayPathRuntime {
	table := dataplane.NewRelayEgressTable()
	return &RelayPathRuntime{
		Table:   table,
		Carrier: dataplane.NewRelayEgressCarrier(table),
	}
}

// RelayEgressActivation describes the result of activating relay egress
// carriage for a single association.
type RelayEgressActivation struct {
	AssociationID string
	SourceService string
	DestNode      string
	DestService   string
	LocalIngress  string // address where local app sends into mesh
	RelayEndpoint string // coordinator relay's per-association listen address
	EgressActive  bool
	Error         string
}

// RelayPathResult summarizes all relay egress activation outcomes.
type RelayPathResult struct {
	Activations []RelayEgressActivation
	TotalActive int
	TotalFailed int
}

// ReportLines produces human-readable log lines for the relay-path result.
func (r RelayPathResult) ReportLines() []string {
	lines := make([]string, 0, len(r.Activations)+3)

	lines = append(lines, fmt.Sprintf(
		"relay-egress activation: active=%d failed=%d (coordinator relay only; node relay, scheduler, multi-WAN not implemented)",
		r.TotalActive, r.TotalFailed,
	))

	for _, a := range r.Activations {
		if a.Error != "" {
			lines = append(lines, fmt.Sprintf(
				"  association %s: FAILED: %s",
				a.AssociationID, a.Error,
			))
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"  association %s: relay egress active",
			a.AssociationID,
		))
		lines = append(lines, fmt.Sprintf(
			"    local_ingress=%s -> relay=%s (outbound via coordinator relay)",
			a.LocalIngress, a.RelayEndpoint,
		))
	}

	return lines
}

// RelayEgressActivationInput provides the minimum context needed to activate
// relay egress carriage for one association. It bridges the control-plane
// association result to the relay egress data-plane activation step.
type RelayEgressActivationInput struct {
	AssociationID    string
	SourceNode       string
	SourceService    service.Identity
	DestNode         string
	DestService      service.Identity
	LocalIngressAddr string // where local app sends traffic into mesh
	RelayEndpoint    string // coordinator relay's per-association listen address
}

// BuildRelayEgressActivationInputs constructs relay egress activation inputs
// from the node config and coordinator association results. Only associations
// with a relay_endpoint are included.
func BuildRelayEgressActivationInputs(
	cfg config.NodeConfig,
	assocResults []AssociationResultEntry,
) []RelayEgressActivationInput {
	type configKey struct {
		sourceService string
		destNode      string
		destService   string
	}
	configMap := make(map[configKey]config.AssociationConfig, len(cfg.Associations))
	for _, ac := range cfg.Associations {
		k := configKey{
			sourceService: strings.TrimSpace(ac.SourceService),
			destNode:      strings.TrimSpace(ac.DestinationNode),
			destService:   strings.TrimSpace(ac.DestinationService),
		}
		configMap[k] = ac
	}

	var inputs []RelayEgressActivationInput
	for _, ar := range assocResults {
		if ar.AssociationID == "" || !ar.Accepted {
			continue
		}

		k := configKey{
			sourceService: strings.TrimSpace(ar.SourceServiceName),
			destNode:      strings.TrimSpace(ar.DestinationNode),
			destService:   strings.TrimSpace(ar.DestinationService),
		}

		ac, exists := configMap[k]
		if !exists || strings.TrimSpace(ac.RelayEndpoint) == "" {
			continue
		}

		// Resolve source service type from local config.
		var sourceType config.ServiceType
		for _, svc := range cfg.Services {
			if svc.Name == ar.SourceServiceName {
				sourceType = svc.Type
				break
			}
		}
		if sourceType == "" {
			sourceType = config.ServiceTypeRawUDP
		}

		// Resolve local ingress address for this service.
		localIngressAddr, err := resolveLocalIngressAddr(cfg, ar.SourceServiceName)
		if err != nil {
			// No valid ingress config for this service; skip.
			continue
		}

		inputs = append(inputs, RelayEgressActivationInput{
			AssociationID:    ar.AssociationID,
			SourceNode:       cfg.Identity.Name,
			SourceService:    service.Identity{Name: ar.SourceServiceName, Type: sourceType},
			DestNode:         ar.DestinationNode,
			DestService:      service.Identity{Name: ar.DestinationService, Type: config.ServiceTypeRawUDP},
			LocalIngressAddr: localIngressAddr,
			RelayEndpoint:    strings.TrimSpace(ac.RelayEndpoint),
		})
	}

	return inputs
}

// ActivateRelayEgressPaths wires relay egress carriage into the node runtime
// for each provided input.
func ActivateRelayEgressPaths(
	ctx context.Context,
	runtime *RelayPathRuntime,
	inputs []RelayEgressActivationInput,
) RelayPathResult {
	var result RelayPathResult

	for _, input := range inputs {
		activation := activateSingleRelayEgress(ctx, runtime, input)
		result.Activations = append(result.Activations, activation)
		if activation.Error != "" {
			result.TotalFailed++
		} else {
			result.TotalActive++
		}
	}

	return result
}

// activateSingleRelayEgress handles one association's relay egress activation.
func activateSingleRelayEgress(
	ctx context.Context,
	runtime *RelayPathRuntime,
	input RelayEgressActivationInput,
) RelayEgressActivation {
	activation := RelayEgressActivation{
		AssociationID: input.AssociationID,
		SourceService: input.SourceService.Name,
		DestNode:      input.DestNode,
		DestService:   input.DestService.Name,
		LocalIngress:  input.LocalIngressAddr,
		RelayEndpoint: input.RelayEndpoint,
	}

	assoc := service.AssociationRecord{
		AssociationID:      input.AssociationID,
		SourceNode:         input.SourceNode,
		SourceService:      input.SourceService,
		DestinationNode:    input.DestNode,
		DestinationService: input.DestService,
		State:              service.AssociationStatePending,
		BootstrapOnly:      true,
	}

	entry, err := dataplane.BuildRelayEgressEntry(assoc, input.LocalIngressAddr, input.RelayEndpoint)
	if err != nil {
		activation.Error = fmt.Sprintf("build relay egress entry: %v", err)
		return activation
	}

	if err := runtime.Table.Install(entry); err != nil {
		activation.Error = fmt.Sprintf("install relay egress entry: %v", err)
		return activation
	}

	if err := runtime.Carrier.StartEgress(ctx, input.AssociationID); err != nil {
		activation.Error = fmt.Sprintf("start relay egress: %v", err)
		return activation
	}
	activation.EgressActive = true

	return activation
}
