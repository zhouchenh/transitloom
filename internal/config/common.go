package config

type Transport string

const (
	TransportQUIC Transport = "quic"
	TransportTCP  Transport = "tcp"
)

type ServiceType string

const (
	ServiceTypeRawUDP ServiceType = "raw-udp"
)

type IngressMode string

const (
	IngressModeStatic             IngressMode = "static"
	IngressModeDeterministicRange IngressMode = "deterministic-range"
	IngressModePersistedAuto      IngressMode = "persisted-auto"
)

type IdentityMetadata struct {
	Name        string   `yaml:"name"`
	Label       string   `yaml:"label,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

type StorageConfig struct {
	DataDir string `yaml:"data_dir"`
	LogDir  string `yaml:"log_dir,omitempty"`
}

type LoggingConfig struct {
	Level       string `yaml:"level,omitempty"`
	Format      string `yaml:"format,omitempty"`
	Destination string `yaml:"destination,omitempty"`
}

type EndpointToggleConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen,omitempty"`
}

type ObservabilityConfig struct {
	Logging LoggingConfig        `yaml:"logging"`
	Metrics EndpointToggleConfig `yaml:"metrics"`
	Status  EndpointToggleConfig `yaml:"status"`
}

type TransportListenerConfig struct {
	Enabled         bool     `yaml:"enabled"`
	ListenEndpoints []string `yaml:"listen_endpoints,omitempty"`
}

type ControlTransportConfig struct {
	QUIC TransportListenerConfig `yaml:"quic"`
	TCP  TransportListenerConfig `yaml:"tcp"`
}

type ControlPreferencesConfig struct {
	AllowedTransports  []Transport `yaml:"allowed_transports,omitempty"`
	PreferredTransport Transport   `yaml:"preferred_transport,omitempty"`
}

type BootstrapCoordinatorConfig struct {
	Label               string      `yaml:"label,omitempty"`
	CoordinatorIDHint   string      `yaml:"coordinator_id_hint,omitempty"`
	ControlEndpoints    []string    `yaml:"control_endpoints"`
	AllowedTransports   []Transport `yaml:"allowed_transports,omitempty"`
	PreferredTransport  Transport   `yaml:"preferred_transport,omitempty"`
	ExpectedTrustAnchor string      `yaml:"expected_trust_anchor,omitempty"`
	Region              string      `yaml:"region,omitempty"`
	Tags                []string    `yaml:"tags,omitempty"`
}

// ServiceBindingConfig is the concrete local service target. It is distinct
// from ServiceIngressConfig, which describes the application-facing local
// endpoint used to send traffic into the mesh.
type ServiceBindingConfig struct {
	Address string `yaml:"address"`
	Port    uint16 `yaml:"port"`
}

// ServiceIngressConfig is optional per service because not every service needs
// a stable local ingress endpoint in v1.
type ServiceIngressConfig struct {
	Mode            IngressMode `yaml:"mode,omitempty"`
	StaticPort      uint16      `yaml:"static_port,omitempty"`
	RangeStart      uint16      `yaml:"range_start,omitempty"`
	RangeEnd        uint16      `yaml:"range_end,omitempty"`
	LoopbackAddress string      `yaml:"loopback_address,omitempty"`
}

type ServiceConfig struct {
	Name         string                `yaml:"name"`
	Type         ServiceType           `yaml:"type"`
	Labels       []string              `yaml:"labels,omitempty"`
	PolicyLabels []string              `yaml:"policy_labels,omitempty"`
	Discoverable bool                  `yaml:"discoverable"`
	Binding      ServiceBindingConfig  `yaml:"binding"`
	Ingress      *ServiceIngressConfig `yaml:"ingress,omitempty"`
}

type LocalIngressPolicyConfig struct {
	DefaultMode     IngressMode `yaml:"default_mode,omitempty"`
	RangeStart      uint16      `yaml:"range_start,omitempty"`
	RangeEnd        uint16      `yaml:"range_end,omitempty"`
	LoopbackAddress string      `yaml:"loopback_address,omitempty"`
}

type CoordinatorDiscoveryConfig struct {
	Discoverable         bool `yaml:"discoverable"`
	AdvertiseEndpoints   bool `yaml:"advertise_endpoints"`
	ParticipateInCatalog bool `yaml:"participate_in_catalog"`
}

type NodeDiscoveryConfig struct {
	Enabled                      bool `yaml:"enabled"`
	SharePrivateAddresses        bool `yaml:"share_private_addresses"`
	ServiceDiscoverableByDefault bool `yaml:"service_discoverable_by_default"`
}

type CoordinatorRelayConfig struct {
	ControlEnabled  bool     `yaml:"control_enabled"`
	DataEnabled     bool     `yaml:"data_enabled"`
	ListenEndpoints []string `yaml:"listen_endpoints,omitempty"`
	DrainMode       bool     `yaml:"drain_mode"`
}

type NodeRelayConfig struct {
	ControlEnabled  bool `yaml:"control_enabled"`
	DataEnabled     bool `yaml:"data_enabled"`
	Advertise       bool `yaml:"advertise"`
	MaxAssociations int  `yaml:"max_associations,omitempty"`
}

// AssociationConfig declares a desired association between a local service
// and a remote service on another node. This is the node-local config intent;
// the coordinator validates and records the association separately.
// Association config is intentionally narrow: it names the source and
// destination services without implying path selection, relay eligibility,
// or forwarding-state installation.
type AssociationConfig struct {
	SourceService      string `yaml:"source_service"`
	DestinationNode    string `yaml:"destination_node"`
	DestinationService string `yaml:"destination_service"`

	// DirectEndpoint is the peer node's mesh-facing UDP address for direct
	// raw UDP carriage (e.g., "192.0.2.1:51830"). This is a bootstrap-only
	// convenience for early direct-path testing. In the full system, peer
	// endpoint addresses will come from coordinator-distributed path
	// candidates, not from static node config.
	//
	// This field is optional. When empty, the association exists as a
	// control-plane record but does not enable direct data-plane carriage.
	DirectEndpoint string `yaml:"direct_endpoint,omitempty"`

	// MeshListenPort is the local UDP port where this node receives inbound
	// direct-path traffic for this association. Because Transitloom v1 uses
	// zero in-band overhead, the association is identified by which mesh
	// listener port received the packet. Each inbound association needs its
	// own mesh listen port.
	//
	// This is a bootstrap-only convenience for direct-path testing. In the
	// full system, mesh listen ports will be managed by the runtime based
	// on coordinator-distributed path candidates.
	//
	// Optional. When zero, no inbound delivery listener is started for
	// this association.
	MeshListenPort uint16 `yaml:"mesh_listen_port,omitempty"`
}
