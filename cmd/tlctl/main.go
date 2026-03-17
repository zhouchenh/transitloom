// tlctl is the operator CLI for Transitloom runtime inspection.
//
// tlctl provides read-oriented commands so operators can inspect important
// runtime state without reading internal code or scraping ad hoc logs.
//
// Design principles:
//   - Inspect real system state; do not redefine or aggregate it
//   - Preserve architectural distinctions (control-plane, data-plane,
//     bootstrap, scheduler, service, and association state are separate)
//   - Never label configured state as verified state
//   - Never label bootstrap/cached state as stronger truth than it is
//   - Keep output concise by default; one concern per command
//
// Commands are intentionally read-only. Mutation (service creation, association
// management, config changes) is out of scope for this tool.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/node"
	"github.com/zhouchenh/transitloom/internal/pki"
)

func main() {
	if len(os.Args) < 3 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	role := os.Args[1]
	action := os.Args[2]
	args := os.Args[3:]

	var exitCode int
	switch role {
	case "node":
		exitCode = runNodeCommand(action, args)
	case "coordinator":
		exitCode = runCoordinatorCommand(action, args)
	default:
		fmt.Fprintf(os.Stderr, "tlctl: unknown subcommand %q\n\n", role)
		printUsage(os.Stderr)
		exitCode = 2
	}
	os.Exit(exitCode)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: tlctl <role> <action> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "node:")
	fmt.Fprintln(w, "  tlctl node bootstrap --config <path>")
	fmt.Fprintln(w, "      node local identity and admission readiness (reads local files, no running process needed)")
	fmt.Fprintln(w, "  tlctl node config --config <path>")
	fmt.Fprintln(w, "      configured services, associations, and external endpoints (config-only, not runtime state)")
	fmt.Fprintln(w, "  tlctl node status --config <path>")
	fmt.Fprintln(w, "      runtime carriage and scheduler state (requires observability.status enabled)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "coordinator:")
	fmt.Fprintln(w, "  tlctl coordinator bootstrap --config <path>")
	fmt.Fprintln(w, "      coordinator trust material readiness (reads local files, no running process needed)")
	fmt.Fprintln(w, "  tlctl coordinator config --config <path>")
	fmt.Fprintln(w, "      coordinator configured state (config-only, not runtime state)")
	fmt.Fprintln(w, "  tlctl coordinator status --config <path>")
	fmt.Fprintln(w, "      runtime service registry and association state (requires observability.status enabled)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "notes:")
	fmt.Fprintln(w, "  bootstrap and config commands read from disk; no running process is required.")
	fmt.Fprintln(w, "  status commands query the observability.status endpoint in the running process.")
	fmt.Fprintln(w, "  bootstrap readiness (ready) means local material is coherent, NOT coordinator-authorized.")
	fmt.Fprintln(w, "  configured state does not imply registered, active, or coordinator-verified state.")
}

// runNodeCommand dispatches node actions.
func runNodeCommand(action string, args []string) int {
	switch action {
	case "bootstrap":
		return runNodeBootstrap(args)
	case "config":
		return runNodeConfig(args)
	case "status":
		return runNodeStatus(args)
	default:
		fmt.Fprintf(os.Stderr, "tlctl node: unknown action %q\n", action)
		return 2
	}
}

// runCoordinatorCommand dispatches coordinator actions.
func runCoordinatorCommand(action string, args []string) int {
	switch action {
	case "bootstrap":
		return runCoordinatorBootstrap(args)
	case "config":
		return runCoordinatorConfig(args)
	case "status":
		return runCoordinatorStatus(args)
	default:
		fmt.Fprintf(os.Stderr, "tlctl coordinator: unknown action %q\n", action)
		return 2
	}
}

// runNodeBootstrap inspects local node bootstrap readiness from disk.
//
// This reads local certificate, key, and admission token files to determine
// the node's local readiness phase. It does NOT contact a coordinator or
// verify current coordinator authorization.
//
// "ready" means local identity material and cached token appear locally
// coherent. It does NOT mean the coordinator has accepted or authorized
// this node for participation.
func runNodeBootstrap(args []string) int {
	fs := flag.NewFlagSet("tlctl node bootstrap", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "path to node YAML config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "tlctl node bootstrap: --config is required")
		return 2
	}

	cfg, err := config.LoadNode(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlctl node bootstrap: load config: %v\n", err)
		return 1
	}

	state, err := node.InspectBootstrap(cfg, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlctl node bootstrap: inspect: %v\n", err)
		return 1
	}

	for _, line := range state.ReportLines() {
		fmt.Println(line)
	}
	return 0
}

// runNodeConfig shows configured node state from the config file.
//
// This reads the node config file and presents configured services,
// associations, external endpoints, and bootstrap coordinators.
//
// Important: the output represents statically configured intent, not verified
// runtime state. "Configured" does not mean registered with the coordinator,
// association active, or endpoint verified.
func runNodeConfig(args []string) int {
	fs := flag.NewFlagSet("tlctl node config", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "path to node YAML config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "tlctl node config: --config is required")
		return 2
	}

	cfg, err := config.LoadNode(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlctl node config: load config: %v\n", err)
		return 1
	}

	for _, line := range nodeConfigSummaryLines(cfg) {
		fmt.Println(line)
	}
	return 0
}

// runNodeStatus queries the node's runtime status endpoint.
//
// This reads the node config to find the observability.status listen address,
// then queries that endpoint for current runtime state (scheduler decisions,
// active carriers, traffic counters). The status endpoint must be enabled
// (observability.status.enabled: true) in the node config for this to succeed.
func runNodeStatus(args []string) int {
	fs := flag.NewFlagSet("tlctl node status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "path to node YAML config file")
	statusURL := fs.String("url", "", "override status URL (e.g. http://127.0.0.1:9200/status)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *configPath == "" && *statusURL == "" {
		fmt.Fprintln(os.Stderr, "tlctl node status: --config or --url is required")
		return 2
	}

	url := *statusURL
	if url == "" {
		cfg, err := config.LoadNode(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tlctl node status: load config: %v\n", err)
			return 1
		}
		if !cfg.Observability.Status.Enabled {
			fmt.Fprintln(os.Stderr, "tlctl node status: observability.status is not enabled in node config")
			fmt.Fprintln(os.Stderr, "  to enable: set observability.status.enabled: true and observability.status.listen: 127.0.0.1:<port>")
			return 1
		}
		if cfg.Observability.Status.Listen == "" {
			fmt.Fprintln(os.Stderr, "tlctl node status: observability.status.listen is not set in node config")
			return 1
		}
		url = "http://" + cfg.Observability.Status.Listen + "/status"
	}

	return queryStatusEndpoint(url)
}

// runCoordinatorBootstrap inspects coordinator trust material readiness from disk.
//
// This reads coordinator trust material files (root anchor, intermediate cert
// and key) to determine whether the coordinator has the trust material required
// to issue node certificates. It does NOT verify certificate validity or
// coordinator network authorization.
func runCoordinatorBootstrap(args []string) int {
	fs := flag.NewFlagSet("tlctl coordinator bootstrap", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "path to coordinator YAML config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "tlctl coordinator bootstrap: --config is required")
		return 2
	}

	cfg, err := config.LoadCoordinator(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlctl coordinator bootstrap: load config: %v\n", err)
		return 1
	}

	state, err := pki.InspectCoordinatorBootstrap(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlctl coordinator bootstrap: inspect: %v\n", err)
		return 1
	}

	for _, line := range state.ReportLines() {
		fmt.Println(line)
	}
	return 0
}

// runCoordinatorConfig shows configured coordinator state from the config file.
//
// This reads the coordinator config and presents identity, control transport
// configuration, trust material paths, and relay configuration.
// Output represents statically configured intent, not verified runtime state.
func runCoordinatorConfig(args []string) int {
	fs := flag.NewFlagSet("tlctl coordinator config", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "path to coordinator YAML config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "tlctl coordinator config: --config is required")
		return 2
	}

	cfg, err := config.LoadCoordinator(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlctl coordinator config: load config: %v\n", err)
		return 1
	}

	for _, line := range coordinatorConfigSummaryLines(cfg) {
		fmt.Println(line)
	}
	return 0
}

// runCoordinatorStatus queries coordinator runtime state via the status endpoint.
//
// This reads the coordinator config to find the observability.status listen
// address, then queries the endpoint for current runtime state (registered
// services, associations). The status endpoint must be enabled in the config.
func runCoordinatorStatus(args []string) int {
	fs := flag.NewFlagSet("tlctl coordinator status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "path to coordinator YAML config file")
	statusURL := fs.String("url", "", "override status URL (e.g. http://127.0.0.1:9201/status)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *configPath == "" && *statusURL == "" {
		fmt.Fprintln(os.Stderr, "tlctl coordinator status: --config or --url is required")
		return 2
	}

	url := *statusURL
	if url == "" {
		cfg, err := config.LoadCoordinator(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tlctl coordinator status: load config: %v\n", err)
			return 1
		}
		if !cfg.Observability.Status.Enabled {
			fmt.Fprintln(os.Stderr, "tlctl coordinator status: observability.status is not enabled in coordinator config")
			fmt.Fprintln(os.Stderr, "  to enable: set observability.status.enabled: true and observability.status.listen: 127.0.0.1:<port>")
			return 1
		}
		if cfg.Observability.Status.Listen == "" {
			fmt.Fprintln(os.Stderr, "tlctl coordinator status: observability.status.listen is not set in coordinator config")
			return 1
		}
		url = "http://" + cfg.Observability.Status.Listen + "/status"
	}

	return queryStatusEndpoint(url)
}

// queryStatusEndpoint fetches and prints status from an HTTP endpoint.
//
// Returns 0 on success, 1 on error. Prints an actionable hint if the
// endpoint is unreachable (e.g., process not running).
func queryStatusEndpoint(url string) int {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlctl: status endpoint %s: %v\n", url, err)
		fmt.Fprintln(os.Stderr, "  (is the process running? is observability.status.enabled: true?)")
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "tlctl: status endpoint returned HTTP %s\n", resp.Status)
		return 1
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlctl: read status response: %v\n", err)
		return 1
	}

	out := strings.TrimRight(string(body), "\n")
	if out != "" {
		fmt.Println(out)
	}
	return 0
}

// nodeConfigSummaryLines produces a concise summary of configured node state.
//
// This presents what is statically declared in the YAML config:
//   - identity (name)
//   - bootstrap coordinators (where to connect for control sessions)
//   - services (what this node exposes; configured, not coordinator-registered)
//   - associations (configured connectivity intent; not active or forwarding)
//   - external endpoint (configured reachability; not verified)
//
// Each section is labeled to prevent misreading configured state as runtime
// or coordinator-verified truth.
func nodeConfigSummaryLines(cfg config.NodeConfig) []string {
	lines := []string{
		fmt.Sprintf("node: name=%q (configured-state-only; not current-runtime-state)", cfg.Identity.Name),
	}

	// Bootstrap coordinators: where this node connects for control sessions.
	// "Configured" does not mean "currently connected" or "authenticated."
	lines = append(lines, fmt.Sprintf("bootstrap-coordinators: count=%d (configured; not currently-connected)", len(cfg.BootstrapCoordinators)))
	for _, bc := range cfg.BootstrapCoordinators {
		label := bc.Label
		if label == "" {
			label = "(no label)"
		}
		lines = append(lines, fmt.Sprintf("  coordinator: label=%s endpoints=%v", label, bc.ControlEndpoints))
	}

	// Services: what this node declares to the mesh.
	// "Configured" does not mean "coordinator-registered" or "discoverable."
	lines = append(lines, fmt.Sprintf("services: count=%d (configured; not coordinator-registered)", len(cfg.Services)))
	for _, svc := range cfg.Services {
		binding := fmt.Sprintf("%s:%d", svc.Binding.Address, svc.Binding.Port)
		lines = append(lines, fmt.Sprintf("  service: name=%s type=%s binding=%s", svc.Name, svc.Type, binding))
	}

	// Associations: configured service-to-service connectivity intent.
	// "Configured" does not mean "active", "coordinator-accepted", or "data flowing."
	lines = append(lines, fmt.Sprintf("associations: count=%d (configured; not active)", len(cfg.Associations)))
	for _, assoc := range cfg.Associations {
		direct := assoc.DirectEndpoint
		if direct == "" {
			direct = "(none)"
		}
		relay := assoc.RelayEndpoint
		if relay == "" {
			relay = "(none)"
		}
		lines = append(lines, fmt.Sprintf("  association: %s -> %s/%s  direct=%s relay=%s",
			assoc.SourceService, assoc.DestinationNode, assoc.DestinationService, direct, relay))

		var profile *config.ProfileConfig
		if assoc.Profile != "" {
			lines = append(lines, fmt.Sprintf("    profile-reference: %s", assoc.Profile))
			for i := range cfg.Profiles {
				if cfg.Profiles[i].Name == assoc.Profile {
					profile = &cfg.Profiles[i]
					break
				}
			}
		}

		if assoc.PolicyOverrides != nil {
			lines = append(lines, "    inline-overrides: present")
		}

		eff := config.ResolvePolicy(profile, assoc.PolicyOverrides)
		lines = append(lines, fmt.Sprintf("    effective-policy: probing=%dms/%dms fallback=%dms/%dms multi-wan-hysteresis=%dms explainability=%s",
			eff.ProbingIntervalMs, eff.ProbingTimeoutMs,
			eff.FallbackDirectToRelayTimeoutMs, eff.FallbackRelayToDirectRecoveryMs,
			eff.MultiWANHysteresisDelayMs,
			eff.ObservabilityExplainabilityLevel))
	}

	// Profiles: defined configuration bundles
	if len(cfg.Profiles) > 0 {
		lines = append(lines, fmt.Sprintf("profiles: count=%d", len(cfg.Profiles)))
		for _, p := range cfg.Profiles {
			lines = append(lines, fmt.Sprintf("  profile: name=%q", p.Name))
		}
	}

	// External endpoint: explicitly configured external reachability.
	// "Configured" is the highest-precedence source but starts as unverified.
	// DNAT port mappings are preserved separately from the external port to
	// prevent silent breakage in DNAT deployments.
	ext := cfg.ExternalEndpoint
	if ext.PublicHost != "" {
		lines = append(lines, fmt.Sprintf("external-endpoint: public-host=%s [source=configured unverified]", ext.PublicHost))
		if len(ext.ForwardedPorts) > 0 {
			for _, fp := range ext.ForwardedPorts {
				if fp.LocalPort != 0 && fp.LocalPort != fp.ExternalPort {
					desc := ""
					if fp.Description != "" {
						desc = " desc=" + fp.Description
					}
					lines = append(lines, fmt.Sprintf("  forwarded-port: external=%d -> local=%d [DNAT]%s",
						fp.ExternalPort, fp.LocalPort, desc))
				} else {
					desc := ""
					if fp.Description != "" {
						desc = " desc=" + fp.Description
					}
					lines = append(lines, fmt.Sprintf("  forwarded-port: port=%d [no-DNAT]%s",
						fp.ExternalPort, desc))
				}
			}
		} else {
			lines = append(lines, "  forwarded-ports: (none configured)")
		}
	} else {
		lines = append(lines, "external-endpoint: (not configured)")
	}

	return lines
}

// coordinatorConfigSummaryLines produces a concise summary of configured coordinator state.
//
// This presents identity, control transport settings, trust material paths,
// and relay configuration. Output is config-only: it does not reflect runtime
// state, registered nodes, accepted sessions, or association records.
func coordinatorConfigSummaryLines(cfg config.CoordinatorConfig) []string {
	lines := []string{
		fmt.Sprintf("coordinator: name=%q (configured-state-only; not current-runtime-state)", cfg.Identity.Name),
	}

	// Control transport: configured listener settings.
	// QUIC and TCP are kept distinct; they are separate transport layers.
	quicEnabled := cfg.Control.QUIC.Enabled
	tcpEnabled := cfg.Control.TCP.Enabled
	lines = append(lines, fmt.Sprintf("control-transport: quic=%v tcp=%v", quicEnabled, tcpEnabled))
	if quicEnabled && len(cfg.Control.QUIC.ListenEndpoints) > 0 {
		lines = append(lines, fmt.Sprintf("  quic-listen: %v", cfg.Control.QUIC.ListenEndpoints))
	}
	if tcpEnabled && len(cfg.Control.TCP.ListenEndpoints) > 0 {
		lines = append(lines, fmt.Sprintf("  tcp-listen: %v", cfg.Control.TCP.ListenEndpoints))
	}

	// Trust material paths only (not content).
	// Use 'tlctl coordinator bootstrap' to inspect actual readiness.
	lines = append(lines, "trust-material-paths: (paths only; use 'tlctl coordinator bootstrap' to inspect readiness)")
	lines = append(lines, fmt.Sprintf("  root-anchor: %s", cfg.Trust.RootAnchorPath))
	lines = append(lines, fmt.Sprintf("  intermediate-cert: %s", cfg.Trust.IntermediateCertPath))
	lines = append(lines, fmt.Sprintf("  intermediate-key: %s", cfg.Trust.IntermediateKeyPath))

	// Relay configuration.
	if cfg.Relay.ControlEnabled || cfg.Relay.DataEnabled {
		lines = append(lines, fmt.Sprintf("relay: control=%v data=%v listen=%v drain=%v",
			cfg.Relay.ControlEnabled, cfg.Relay.DataEnabled,
			cfg.Relay.ListenEndpoints, cfg.Relay.DrainMode))
	} else {
		lines = append(lines, "relay: (not enabled)")
	}

	return lines
}
