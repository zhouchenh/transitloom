package service

import (
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
)

// Identity is the logical service identity within a node scope. It stays
// separate from the service binding and from any local ingress intent.
type Identity struct {
	Name string             `json:"name"`
	Type config.ServiceType `json:"type"`
}

type Metadata struct {
	Labels       []string `json:"labels,omitempty"`
	PolicyLabels []string `json:"policy_labels,omitempty"`
	Discoverable bool     `json:"discoverable"`
}

type LocalTarget struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
}

// Binding maps the logical service to the local runtime target that receives
// inbound carried traffic.
type Binding struct {
	LocalTarget LocalTarget `json:"local_target"`
}

// LocalIngressIntent captures requested application-facing ingress behavior
// without claiming that a concrete ingress binding has already been allocated.
type LocalIngressIntent struct {
	Mode            config.IngressMode `json:"mode"`
	LoopbackAddress string             `json:"loopback_address,omitempty"`
	StaticPort      uint16             `json:"static_port,omitempty"`
	RangeStart      uint16             `json:"range_start,omitempty"`
	RangeEnd        uint16             `json:"range_end,omitempty"`
}

// Registration is the node-reported service declaration used by the current
// bootstrap-only service-registration path.
type Registration struct {
	Identity              Identity            `json:"identity"`
	Metadata              Metadata            `json:"metadata"`
	Binding               Binding             `json:"binding"`
	RequestedLocalIngress *LocalIngressIntent `json:"requested_local_ingress,omitempty"`
}

// Record is the coordinator-side placeholder registry state for a registered
// service. RequestedLocalIngress remains separate because registration does not
// allocate or authorize application-facing ingress bindings.
type Record struct {
	NodeName              string
	Identity              Identity
	Metadata              Metadata
	Binding               Binding
	RequestedLocalIngress *LocalIngressIntent
	RegisteredAt          time.Time
	BootstrapOnly         bool
}

func BuildRegistrations(cfg config.NodeConfig) ([]Registration, error) {
	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("node config has no services to register")
	}

	registrations := make([]Registration, 0, len(cfg.Services))
	for i, serviceCfg := range cfg.Services {
		registration := Registration{
			Identity: Identity{
				Name: strings.TrimSpace(serviceCfg.Name),
				Type: serviceCfg.Type,
			},
			Metadata: Metadata{
				Labels:       append([]string(nil), serviceCfg.Labels...),
				PolicyLabels: append([]string(nil), serviceCfg.PolicyLabels...),
				Discoverable: serviceCfg.Discoverable,
			},
			Binding: Binding{
				LocalTarget: LocalTarget{
					Address: strings.TrimSpace(serviceCfg.Binding.Address),
					Port:    serviceCfg.Binding.Port,
				},
			},
		}

		if serviceCfg.Ingress != nil {
			registration.RequestedLocalIngress = resolveLocalIngressIntent(*serviceCfg.Ingress, cfg.LocalIngress)
		}

		if err := registration.Validate(); err != nil {
			return nil, fmt.Errorf("services[%d]: %w", i, err)
		}

		registrations = append(registrations, registration)
	}

	return registrations, nil
}

func RecordKey(nodeName string, identity Identity) string {
	return fmt.Sprintf("%s/%s:%s", strings.TrimSpace(nodeName), strings.TrimSpace(identity.Name), identity.Type)
}

func CloneLocalIngressIntent(intent *LocalIngressIntent) *LocalIngressIntent {
	if intent == nil {
		return nil
	}

	clone := *intent
	return &clone
}

func (r Registration) Validate() error {
	if err := r.Identity.Validate(); err != nil {
		return fmt.Errorf("identity: %w", err)
	}
	if err := r.Binding.Validate(); err != nil {
		return fmt.Errorf("binding: %w", err)
	}
	if r.RequestedLocalIngress != nil {
		if err := r.RequestedLocalIngress.Validate(); err != nil {
			return fmt.Errorf("requested_local_ingress: %w", err)
		}
	}
	return nil
}

func (r Registration) ToRecord(nodeName string, registeredAt time.Time) Record {
	return Record{
		NodeName:              strings.TrimSpace(nodeName),
		Identity:              r.Identity,
		Metadata:              cloneMetadata(r.Metadata),
		Binding:               r.Binding,
		RequestedLocalIngress: CloneLocalIngressIntent(r.RequestedLocalIngress),
		RegisteredAt:          registeredAt,
		BootstrapOnly:         true,
	}
}

func (r Record) Key() string {
	return RecordKey(r.NodeName, r.Identity)
}

func (r Record) Clone() Record {
	return Record{
		NodeName:              r.NodeName,
		Identity:              r.Identity,
		Metadata:              cloneMetadata(r.Metadata),
		Binding:               r.Binding,
		RequestedLocalIngress: CloneLocalIngressIntent(r.RequestedLocalIngress),
		RegisteredAt:          r.RegisteredAt,
		BootstrapOnly:         r.BootstrapOnly,
	}
}

func (i Identity) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return fmt.Errorf("name must be set")
	}
	if i.Type != config.ServiceTypeRawUDP {
		return fmt.Errorf("type must be %q in v1", config.ServiceTypeRawUDP)
	}
	return nil
}

func (b Binding) Validate() error {
	return b.LocalTarget.Validate()
}

func (t LocalTarget) Validate() error {
	if strings.TrimSpace(t.Address) == "" {
		return fmt.Errorf("local_target.address must be set")
	}
	if _, err := netip.ParseAddr(t.Address); err != nil {
		return fmt.Errorf("local_target.address must be a valid IP address")
	}
	if t.Port == 0 {
		return fmt.Errorf("local_target.port must be greater than zero")
	}
	return nil
}

func (i LocalIngressIntent) Validate() error {
	switch i.Mode {
	case config.IngressModeStatic:
		if i.StaticPort == 0 {
			return fmt.Errorf("static_port must be greater than zero when mode is %q", config.IngressModeStatic)
		}
		if i.RangeStart != 0 || i.RangeEnd != 0 {
			return fmt.Errorf("range_start and range_end must not be set when mode is %q", config.IngressModeStatic)
		}
	case config.IngressModeDeterministicRange:
		if i.RangeStart == 0 || i.RangeEnd == 0 {
			return fmt.Errorf("range_start and range_end must both be set when mode is %q", config.IngressModeDeterministicRange)
		}
		if i.RangeStart > i.RangeEnd {
			return fmt.Errorf("range_start must be less than or equal to range_end")
		}
		if i.StaticPort != 0 {
			return fmt.Errorf("static_port must not be set when mode is %q", config.IngressModeDeterministicRange)
		}
	case config.IngressModePersistedAuto:
		if i.StaticPort != 0 {
			return fmt.Errorf("static_port must not be set when mode is %q", config.IngressModePersistedAuto)
		}
		if (i.RangeStart == 0) != (i.RangeEnd == 0) {
			return fmt.Errorf("range_start and range_end must both be set together")
		}
		if i.RangeStart > i.RangeEnd {
			return fmt.Errorf("range_start must be less than or equal to range_end")
		}
	default:
		return fmt.Errorf("mode must be %q, %q, or %q", config.IngressModeStatic, config.IngressModeDeterministicRange, config.IngressModePersistedAuto)
	}

	if strings.TrimSpace(i.LoopbackAddress) != "" {
		addr, err := netip.ParseAddr(i.LoopbackAddress)
		if err != nil {
			return fmt.Errorf("loopback_address must be a valid IP address")
		}
		if !addr.IsLoopback() {
			return fmt.Errorf("loopback_address must be a loopback IP address")
		}
	}

	return nil
}

func resolveLocalIngressIntent(serviceIngress config.ServiceIngressConfig, nodeIngress config.LocalIngressPolicyConfig) *LocalIngressIntent {
	intent := &LocalIngressIntent{
		Mode:            serviceIngress.Mode,
		LoopbackAddress: strings.TrimSpace(serviceIngress.LoopbackAddress),
		StaticPort:      serviceIngress.StaticPort,
		RangeStart:      serviceIngress.RangeStart,
		RangeEnd:        serviceIngress.RangeEnd,
	}

	if intent.Mode == "" {
		intent.Mode = nodeIngress.DefaultMode
	}
	if intent.LoopbackAddress == "" {
		intent.LoopbackAddress = strings.TrimSpace(nodeIngress.LoopbackAddress)
	}
	if intent.RangeStart == 0 && intent.RangeEnd == 0 {
		intent.RangeStart = nodeIngress.RangeStart
		intent.RangeEnd = nodeIngress.RangeEnd
	}

	return intent
}

func cloneMetadata(metadata Metadata) Metadata {
	return Metadata{
		Labels:       append([]string(nil), metadata.Labels...),
		PolicyLabels: append([]string(nil), metadata.PolicyLabels...),
		Discoverable: metadata.Discoverable,
	}
}
