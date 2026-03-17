package config

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

type validationErrors struct {
	entries []string
}

func (v *validationErrors) add(field, message string) {
	v.entries = append(v.entries, fmt.Sprintf("%s: %s", field, message))
}

func (v *validationErrors) err(role string) error {
	if len(v.entries) == 0 {
		return nil
	}

	var builder strings.Builder
	builder.WriteString(role)
	builder.WriteString(" config validation failed:")
	for _, entry := range v.entries {
		builder.WriteString("\n- ")
		builder.WriteString(entry)
	}

	return fmt.Errorf("%s", builder.String())
}

func validateIdentity(prefix string, identity IdentityMetadata, errs *validationErrors) {
	if strings.TrimSpace(identity.Name) == "" {
		errs.add(prefix+".name", "must be set")
	}
}

func validateStorage(prefix string, storage StorageConfig, errs *validationErrors) {
	if strings.TrimSpace(storage.DataDir) == "" {
		errs.add(prefix+".data_dir", "must be set")
	}
}

func validateNodeIdentity(prefix string, identity NodeIdentityConfig, errs *validationErrors) {
	if strings.TrimSpace(identity.CertificatePath) == "" {
		errs.add(prefix+".certificate_path", "must be set")
	}
	if strings.TrimSpace(identity.PrivateKeyPath) == "" {
		errs.add(prefix+".private_key_path", "must be set")
	}
}

func validateNodeAdmission(prefix string, admission NodeAdmissionConfig, errs *validationErrors) {
	if strings.TrimSpace(admission.CurrentTokenPath) == "" {
		errs.add(prefix+".current_token_path", "must be set")
	}
}

func validateLogging(prefix string, logging LoggingConfig, errs *validationErrors) {
	switch strings.TrimSpace(logging.Level) {
	case "", "debug", "info", "warn", "error":
	default:
		errs.add(prefix+".level", `must be one of "debug", "info", "warn", or "error"`)
	}

	switch strings.TrimSpace(logging.Format) {
	case "", "text", "json":
	default:
		errs.add(prefix+".format", `must be one of "text" or "json"`)
	}
}

func validateEndpointToggle(prefix string, toggle EndpointToggleConfig, errs *validationErrors) {
	if toggle.Enabled && strings.TrimSpace(toggle.Listen) == "" {
		errs.add(prefix+".listen", "must be set when enabled is true")
	}

	if strings.TrimSpace(toggle.Listen) != "" {
		validateHostPort(prefix+".listen", toggle.Listen, true, errs)
	}
}

func validateObservability(prefix string, observability ObservabilityConfig, errs *validationErrors) {
	validateLogging(prefix+".logging", observability.Logging, errs)
	validateEndpointToggle(prefix+".metrics", observability.Metrics, errs)
	validateEndpointToggle(prefix+".status", observability.Status, errs)
}

func validateTransportListener(prefix string, listener TransportListenerConfig, errs *validationErrors) {
	if listener.Enabled && len(listener.ListenEndpoints) == 0 {
		errs.add(prefix+".listen_endpoints", "must contain at least one endpoint when enabled is true")
	}

	for i, endpoint := range listener.ListenEndpoints {
		validateHostPort(fmt.Sprintf("%s.listen_endpoints[%d]", prefix, i), endpoint, true, errs)
	}
}

func validateControlPreferences(prefix string, control ControlPreferencesConfig, errs *validationErrors) {
	allowed := make(map[Transport]struct{}, len(control.AllowedTransports))
	for i, transport := range control.AllowedTransports {
		if !isSupportedTransport(transport) {
			errs.add(fmt.Sprintf("%s.allowed_transports[%d]", prefix, i), `must be "quic" or "tcp"`)
			continue
		}
		if _, exists := allowed[transport]; exists {
			errs.add(fmt.Sprintf("%s.allowed_transports[%d]", prefix, i), "must not repeat a transport")
			continue
		}
		allowed[transport] = struct{}{}
	}

	if control.PreferredTransport == "" {
		return
	}
	if !isSupportedTransport(control.PreferredTransport) {
		errs.add(prefix+".preferred_transport", `must be "quic" or "tcp"`)
		return
	}
	if len(allowed) > 0 {
		if _, exists := allowed[control.PreferredTransport]; !exists {
			errs.add(prefix+".preferred_transport", "must be included in allowed_transports when that list is set")
		}
	}
}

func validateBootstrapCoordinator(prefix string, entry BootstrapCoordinatorConfig, errs *validationErrors) {
	if len(entry.ControlEndpoints) == 0 {
		errs.add(prefix+".control_endpoints", "must contain at least one endpoint")
	}
	for i, endpoint := range entry.ControlEndpoints {
		validateHostPort(fmt.Sprintf("%s.control_endpoints[%d]", prefix, i), endpoint, false, errs)
	}
	validateControlPreferences(prefix, ControlPreferencesConfig{
		AllowedTransports:  entry.AllowedTransports,
		PreferredTransport: entry.PreferredTransport,
	}, errs)
}

func validateBinding(prefix string, binding ServiceBindingConfig, errs *validationErrors) {
	if strings.TrimSpace(binding.Address) == "" {
		errs.add(prefix+".address", "must be set")
	} else {
		validateIPAddress(prefix+".address", binding.Address, false, errs)
	}
	if binding.Port == 0 {
		errs.add(prefix+".port", "must be greater than zero")
	}
}

func validateLocalIngressPolicy(prefix string, policy LocalIngressPolicyConfig, errs *validationErrors) {
	if policy.DefaultMode != "" {
		switch policy.DefaultMode {
		case IngressModeDeterministicRange, IngressModePersistedAuto:
		default:
			errs.add(prefix+".default_mode", `must be "deterministic-range" or "persisted-auto"`)
		}
	}

	validatePortRange(prefix, policy.RangeStart, policy.RangeEnd, true, errs)

	if strings.TrimSpace(policy.LoopbackAddress) != "" {
		validateIPAddress(prefix+".loopback_address", policy.LoopbackAddress, true, errs)
	}
}

func validateService(prefix string, service ServiceConfig, nodeIngress LocalIngressPolicyConfig, errs *validationErrors) {
	if strings.TrimSpace(service.Name) == "" {
		errs.add(prefix+".name", "must be set")
	}

	if service.Type != ServiceTypeRawUDP {
		errs.add(prefix+".type", `must be "raw-udp" in v1`)
	}

	validateBinding(prefix+".binding", service.Binding, errs)

	if service.Ingress == nil {
		return
	}

	mode := service.Ingress.Mode
	if mode == "" {
		mode = nodeIngress.DefaultMode
	}
	if mode == "" {
		errs.add(prefix+".ingress.mode", "must be set on the service or inherited from local_ingress.default_mode")
		return
	}

	validatePortRange(prefix+".ingress", service.Ingress.RangeStart, service.Ingress.RangeEnd, true, errs)

	loopbackAddress := strings.TrimSpace(service.Ingress.LoopbackAddress)
	if loopbackAddress == "" {
		loopbackAddress = strings.TrimSpace(nodeIngress.LoopbackAddress)
	}
	if loopbackAddress != "" {
		validateIPAddress(prefix+".ingress.loopback_address", loopbackAddress, true, errs)
	}

	switch mode {
	case IngressModeStatic:
		if service.Ingress.StaticPort == 0 {
			errs.add(prefix+".ingress.static_port", "must be greater than zero when ingress.mode is static")
		}
		if service.Ingress.RangeStart != 0 || service.Ingress.RangeEnd != 0 {
			errs.add(prefix+".ingress.range_start", "must not be set when ingress.mode is static")
		}
	case IngressModeDeterministicRange:
		rangeStart, rangeEnd := service.Ingress.RangeStart, service.Ingress.RangeEnd
		if rangeStart == 0 && rangeEnd == 0 {
			rangeStart, rangeEnd = nodeIngress.RangeStart, nodeIngress.RangeEnd
		}
		if rangeStart == 0 || rangeEnd == 0 {
			errs.add(prefix+".ingress.range_start", "must be set on the service or local_ingress when ingress.mode is deterministic-range")
		}
		if service.Ingress.StaticPort != 0 {
			errs.add(prefix+".ingress.static_port", "must not be set when ingress.mode is deterministic-range")
		}
	case IngressModePersistedAuto:
		if service.Ingress.StaticPort != 0 {
			errs.add(prefix+".ingress.static_port", "must not be set when ingress.mode is persisted-auto")
		}
	default:
		errs.add(prefix+".ingress.mode", `must be "static", "deterministic-range", or "persisted-auto"`)
	}
}

func validatePortRange(prefix string, start, end uint16, allowEmpty bool, errs *validationErrors) {
	if start == 0 && end == 0 {
		if !allowEmpty {
			errs.add(prefix+".range_start", "must be set")
		}
		return
	}
	if start == 0 || end == 0 {
		errs.add(prefix+".range_start", "range_start and range_end must both be set")
		return
	}
	if start > end {
		errs.add(prefix+".range_start", "must be less than or equal to range_end")
	}
}

func validateHostPort(field, value string, allowEmptyHost bool, errs *validationErrors) {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		errs.add(field, `must be a valid host:port value`)
		return
	}
	if !allowEmptyHost && strings.TrimSpace(host) == "" {
		errs.add(field, "must include a host")
	}
	if strings.TrimSpace(port) == "" {
		errs.add(field, "must include a non-zero port")
		return
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber <= 0 || portNumber > 65535 {
		errs.add(field, "must include a valid port between 1 and 65535")
	}
}

func validateIPAddress(field, value string, mustBeLoopback bool, errs *validationErrors) {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		errs.add(field, "must be a valid IP address")
		return
	}
	if mustBeLoopback && !addr.IsLoopback() {
		errs.add(field, "must be a loopback IP address")
	}
}

func validateDuration(field, value string, errs *validationErrors) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if _, err := time.ParseDuration(value); err != nil {
		errs.add(field, "must be a valid Go duration string")
	}
}

func validateAssociation(prefix string, assoc AssociationConfig, serviceNames map[string]struct{}, errs *validationErrors) {
	if strings.TrimSpace(assoc.SourceService) == "" {
		errs.add(prefix+".source_service", "must be set")
	} else if _, exists := serviceNames[assoc.SourceService]; !exists {
		errs.add(prefix+".source_service", "must reference a configured service name")
	}
	if strings.TrimSpace(assoc.DestinationNode) == "" {
		errs.add(prefix+".destination_node", "must be set")
	}
	if strings.TrimSpace(assoc.DestinationService) == "" {
		errs.add(prefix+".destination_service", "must be set")
	}
	if endpoint := strings.TrimSpace(assoc.DirectEndpoint); endpoint != "" {
		if _, _, err := net.SplitHostPort(endpoint); err != nil {
			errs.add(prefix+".direct_endpoint", "must be a valid host:port address")
		}
	}
	if endpoint := strings.TrimSpace(assoc.RelayEndpoint); endpoint != "" {
		if _, _, err := net.SplitHostPort(endpoint); err != nil {
			errs.add(prefix+".relay_endpoint", "must be a valid host:port address")
		}
	}
}

func isSupportedTransport(transport Transport) bool {
	return transport == TransportQUIC || transport == TransportTCP
}
