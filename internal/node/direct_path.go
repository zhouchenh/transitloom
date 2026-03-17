package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/dataplane"
	"github.com/zhouchenh/transitloom/internal/service"
)

// DirectPathRuntime holds the data-plane forwarding table and carrier for
// direct raw UDP carriage. It is the minimum node-runtime integration needed
// so that Transitloom local ingress endpoints can be used as WireGuard peer
// endpoints on a direct path.
//
// This runtime is direct-path only. It does not implement relay, scheduler,
// multi-WAN, or encrypted carriage behavior. A successful direct-path
// activation does not imply any of those capabilities.
//
// The runtime preserves these architectural distinctions:
//   - local ingress: Transitloom-provided port where the local application
//     (e.g., WireGuard) sends traffic into the mesh
//   - local target: the service binding's port where inbound carried traffic
//     is delivered to the local service (e.g., WireGuard ListenPort)
//   - service binding: maps the logical service to its local target
//   - association: the legal, policy-controlled relationship between services
//     that makes forwarding possible
type DirectPathRuntime struct {
	Table   *dataplane.ForwardingTable
	Carrier *dataplane.DirectCarrier
}

// NewDirectPathRuntime creates a new direct-path runtime with an empty
// forwarding table and carrier.
func NewDirectPathRuntime() *DirectPathRuntime {
	table := dataplane.NewForwardingTable()
	return &DirectPathRuntime{
		Table:   table,
		Carrier: dataplane.NewDirectCarrier(table),
	}
}

// DirectPathActivation describes the result of activating direct-path
// carriage for a single association.
type DirectPathActivation struct {
	AssociationID  string
	SourceService  string
	DestNode       string
	DestService    string
	LocalIngress   string // address where local app sends into mesh
	LocalTarget    string // address where inbound traffic is delivered
	RemoteEndpoint string // peer's direct mesh endpoint
	MeshListen     string // local mesh listen address (if delivery active)
	IngressActive  bool
	DeliveryActive bool
	Error          string
}

// DirectPathResult summarizes all direct-path activation outcomes.
type DirectPathResult struct {
	Activations []DirectPathActivation
	TotalActive int
	TotalFailed int
}

// ReportLines produces human-readable log lines for the direct-path result.
func (r DirectPathResult) ReportLines() []string {
	lines := make([]string, 0, len(r.Activations)+4)

	lines = append(lines, fmt.Sprintf(
		"direct-path activation: active=%d failed=%d (direct-path only; relay, scheduler, multi-WAN not implemented)",
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

		status := ""
		if a.IngressActive && a.DeliveryActive {
			status = "ingress+delivery active"
		} else if a.IngressActive {
			status = "ingress active (no delivery)"
		} else if a.DeliveryActive {
			status = "delivery active (no ingress)"
		}

		lines = append(lines, fmt.Sprintf(
			"  association %s: %s",
			a.AssociationID, status,
		))
		lines = append(lines, fmt.Sprintf(
			"    local_ingress=%s -> remote=%s (outbound: app sends here into mesh)",
			a.LocalIngress, a.RemoteEndpoint,
		))
		if a.DeliveryActive {
			lines = append(lines, fmt.Sprintf(
				"    mesh_listen=%s -> local_target=%s (inbound: mesh delivers to service)",
				a.MeshListen, a.LocalTarget,
			))
		}
	}

	lines = append(lines, "direct-path validation note: this proves WireGuard can use Transitloom local ingress ports on a direct path; standard WireGuard remains unchanged")

	return lines
}

// ActivateDirectPaths wires the direct raw UDP carriage primitives into the
// node runtime for each configured association that has a direct_endpoint.
//
// For each eligible association, this function:
//  1. Resolves the local ingress address from the service's ingress config
//  2. Builds a forwarding entry bridging control-plane association state to
//     data-plane forwarding state
//  3. Installs the entry in the forwarding table
//  4. Starts the ingress listener (outbound: local app → mesh → remote peer)
//  5. If mesh_listen_port is configured, starts the delivery listener
//     (inbound: remote peer → mesh → local service target)
//
// The function preserves the architectural distinction between local ingress
// and local target. WireGuard's real ListenPort is the local target.
// Transitloom's local ingress ports are separate peer-facing endpoints.
func ActivateDirectPaths(
	ctx context.Context,
	cfg config.NodeConfig,
	runtime *DirectPathRuntime,
	associationResults []AssociationActivationInput,
) DirectPathResult {
	var result DirectPathResult

	for _, input := range associationResults {
		activation := activateSingleDirectPath(ctx, cfg, runtime, input)
		result.Activations = append(result.Activations, activation)
		if activation.Error != "" {
			result.TotalFailed++
		} else {
			result.TotalActive++
		}
	}

	return result
}

// AssociationActivationInput provides the minimum context needed to activate
// direct-path carriage for one association. It bridges the control-plane
// association result to the data-plane activation step.
type AssociationActivationInput struct {
	AssociationID string
	SourceNode    string
	SourceService service.Identity
	DestNode      string
	DestService   service.Identity

	// DirectEndpoint is the remote peer's mesh-facing UDP endpoint.
	DirectEndpoint string

	// MeshListenPort is the local port for inbound delivery (0 = no delivery).
	MeshListenPort uint16
}

// BuildAssociationActivationInputs constructs activation inputs from the node
// config and coordinator association results. Only associations with a
// direct_endpoint are included — associations without one remain control-plane
// records only.
func BuildAssociationActivationInputs(
	cfg config.NodeConfig,
	assocResults []AssociationResultEntry,
) []AssociationActivationInput {
	// Build a map from association key parts to config for direct_endpoint lookup.
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

	var inputs []AssociationActivationInput
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
		if !exists || strings.TrimSpace(ac.DirectEndpoint) == "" {
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

		inputs = append(inputs, AssociationActivationInput{
			AssociationID: ar.AssociationID,
			SourceNode:    cfg.Identity.Name,
			SourceService: service.Identity{Name: ar.SourceServiceName, Type: sourceType},
			DestNode:      ar.DestinationNode,
			DestService:   service.Identity{Name: ar.DestinationService, Type: config.ServiceTypeRawUDP},
			DirectEndpoint: strings.TrimSpace(ac.DirectEndpoint),
			MeshListenPort: ac.MeshListenPort,
		})
	}

	return inputs
}

// AssociationResultEntry is a minimal summary of an accepted association from
// the coordinator, used to bridge to direct-path activation.
type AssociationResultEntry struct {
	AssociationID      string
	SourceServiceName  string
	DestinationNode    string
	DestinationService string
	Accepted           bool
}

// activateSingleDirectPath handles one association's direct-path activation.
func activateSingleDirectPath(
	ctx context.Context,
	cfg config.NodeConfig,
	runtime *DirectPathRuntime,
	input AssociationActivationInput,
) DirectPathActivation {
	activation := DirectPathActivation{
		AssociationID:  input.AssociationID,
		SourceService:  input.SourceService.Name,
		DestNode:       input.DestNode,
		DestService:    input.DestService.Name,
		RemoteEndpoint: input.DirectEndpoint,
	}

	// Resolve the local ingress address from the source service's config.
	localIngressAddr, err := resolveLocalIngressAddr(cfg, input.SourceService.Name)
	if err != nil {
		activation.Error = fmt.Sprintf("resolve local ingress: %v", err)
		return activation
	}
	activation.LocalIngress = localIngressAddr

	// Resolve the local target from the source service's binding.
	// The local target is the WireGuard ListenPort — the service binding's
	// endpoint where inbound carried traffic is delivered.
	localTargetAddr, err := resolveLocalTargetAddr(cfg, input.SourceService.Name)
	if err != nil {
		activation.Error = fmt.Sprintf("resolve local target: %v", err)
		return activation
	}
	activation.LocalTarget = localTargetAddr

	// Resolve mesh listen address for inbound delivery.
	meshListenAddr := ""
	if input.MeshListenPort > 0 {
		meshListenAddr = fmt.Sprintf("0.0.0.0:%d", input.MeshListenPort)
		activation.MeshListen = meshListenAddr
	}

	// Build the service records needed by the forwarding entry builder.
	sourceRecord, destRecord := buildMinimalServiceRecords(cfg, input)

	// Build the forwarding entry.
	entry, err := dataplane.BuildDirectForwardingEntry(
		service.AssociationRecord{
			AssociationID:      input.AssociationID,
			SourceNode:         input.SourceNode,
			SourceService:      input.SourceService,
			DestinationNode:    input.DestNode,
			DestinationService: input.DestService,
			State:              service.AssociationStatePending,
			BootstrapOnly:      true,
		},
		sourceRecord,
		destRecord,
		input.DirectEndpoint,
		localIngressAddr,
		meshListenAddr,
	)
	if err != nil {
		activation.Error = fmt.Sprintf("build forwarding entry: %v", err)
		return activation
	}

	// Install in the forwarding table.
	if err := runtime.Table.Install(entry); err != nil {
		activation.Error = fmt.Sprintf("install forwarding entry: %v", err)
		return activation
	}

	// Start ingress (outbound: local app → mesh → remote).
	if err := runtime.Carrier.StartIngress(ctx, input.AssociationID); err != nil {
		activation.Error = fmt.Sprintf("start ingress: %v", err)
		return activation
	}
	activation.IngressActive = true

	// Start delivery if mesh listen port is configured (inbound: mesh → local target).
	if input.MeshListenPort > 0 {
		if err := runtime.Carrier.StartDelivery(ctx, input.AssociationID); err != nil {
			activation.Error = fmt.Sprintf("start delivery: %v", err)
			return activation
		}
		activation.DeliveryActive = true
	}

	return activation
}

// resolveLocalIngressAddr resolves the concrete local ingress address for a
// service from its config. The local ingress is where the local application
// sends traffic into the mesh — NOT the service's own local target.
func resolveLocalIngressAddr(cfg config.NodeConfig, serviceName string) (string, error) {
	for _, svc := range cfg.Services {
		if svc.Name != serviceName {
			continue
		}

		loopback := "127.0.0.1"

		// Check service-level ingress config first.
		if svc.Ingress != nil {
			if svc.Ingress.LoopbackAddress != "" {
				loopback = svc.Ingress.LoopbackAddress
			} else if cfg.LocalIngress.LoopbackAddress != "" {
				loopback = cfg.LocalIngress.LoopbackAddress
			}

			switch svc.Ingress.Mode {
			case config.IngressModeStatic:
				if svc.Ingress.StaticPort > 0 {
					return fmt.Sprintf("%s:%d", loopback, svc.Ingress.StaticPort), nil
				}
			case config.IngressModeDeterministicRange:
				if svc.Ingress.RangeStart > 0 {
					// For bootstrap testing, use the range start port.
					return fmt.Sprintf("%s:%d", loopback, svc.Ingress.RangeStart), nil
				}
			}
		}

		// Fall back to node-level ingress policy.
		if cfg.LocalIngress.LoopbackAddress != "" {
			loopback = cfg.LocalIngress.LoopbackAddress
		}

		if cfg.LocalIngress.DefaultMode == config.IngressModeStatic {
			// No static port available at node level, use ephemeral.
			return fmt.Sprintf("%s:0", loopback), nil
		}

		if cfg.LocalIngress.DefaultMode == config.IngressModeDeterministicRange && cfg.LocalIngress.RangeStart > 0 {
			return fmt.Sprintf("%s:%d", loopback, cfg.LocalIngress.RangeStart), nil
		}

		// Default: ephemeral port on loopback.
		return fmt.Sprintf("%s:0", loopback), nil
	}

	return "", fmt.Errorf("service %q not found in node config", serviceName)
}

// resolveLocalTargetAddr resolves the local target address from the service
// binding config. This is the WireGuard ListenPort — the address where
// Transitloom delivers inbound carried traffic to the local service.
func resolveLocalTargetAddr(cfg config.NodeConfig, serviceName string) (string, error) {
	for _, svc := range cfg.Services {
		if svc.Name == serviceName {
			if svc.Binding.Address == "" || svc.Binding.Port == 0 {
				return "", fmt.Errorf("service %q has incomplete binding", serviceName)
			}
			return fmt.Sprintf("%s:%d", svc.Binding.Address, svc.Binding.Port), nil
		}
	}
	return "", fmt.Errorf("service %q not found in node config", serviceName)
}

// buildMinimalServiceRecords creates the source and destination service records
// needed by the forwarding entry builder. For the destination side, we use the
// source service's binding as the local target because on this node, inbound
// traffic should be delivered to the local service's listen port.
func buildMinimalServiceRecords(cfg config.NodeConfig, input AssociationActivationInput) (service.Record, service.Record) {
	var sourceBinding service.Binding
	for _, svc := range cfg.Services {
		if svc.Name == input.SourceService.Name {
			sourceBinding = service.Binding{
				LocalTarget: service.LocalTarget{
					Address: svc.Binding.Address,
					Port:    svc.Binding.Port,
				},
			}
			break
		}
	}

	sourceRecord := service.Record{
		NodeName: input.SourceNode,
		Identity: input.SourceService,
		Binding:  sourceBinding,
	}

	// For direct-path carriage, the destination service record's local target
	// is the same as the source service's binding because delivery happens to
	// the local service on this node. The actual remote target would come from
	// the coordinator in a full system.
	destRecord := service.Record{
		NodeName: input.DestNode,
		Identity: input.DestService,
		Binding:  sourceBinding, // local target for delivery
	}

	return sourceRecord, destRecord
}
