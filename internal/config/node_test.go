package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhouchenh/transitloom/internal/config"
)

func TestLoadNodeValid(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadNode(filepath.Join("testdata", "node-valid.yaml"))
	if err != nil {
		t.Fatalf("LoadNode() error = %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestNodeValidateInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		wantText []string
	}{
		{
			name: "bootstrap coordinator required",
			yaml: `
identity:
  name: node-a
storage:
  data_dir: /var/lib/transitloom/node
node_identity:
  certificate_path: identity/current.crt
  private_key_path: identity/current.key
admission:
  current_token_path: admission/current-token.json
services:
  - name: wg0
    type: raw-udp
    binding:
      address: 127.0.0.1
      port: 51820
`,
			wantText: []string{
				"bootstrap_coordinators: must contain at least one bootstrap coordinator",
			},
		},
		{
			name: "service ingress requires usable range or static port",
			yaml: `
identity:
  name: node-a
storage:
  data_dir: /var/lib/transitloom/node
node_identity:
  certificate_path: identity/current.crt
  private_key_path: identity/current.key
admission:
  current_token_path: admission/current-token.json
bootstrap_coordinators:
  - label: coord-a
    control_endpoints:
      - coord-a.example.net:8443
    allowed_transports:
      - quic
services:
  - name: wg0
    type: raw-udp
    binding:
      address: 127.0.0.1
      port: 51820
    ingress:
      mode: deterministic-range
  - name: wg0
    type: raw-udp
    binding:
      address: 127.0.0.1
      port: 51821
`,
			wantText: []string{
				"services[0].ingress.range_start: must be set on the service or local_ingress when ingress.mode is deterministic-range",
				"services[1].name: must be unique within the node config",
			},
		},
		{
			name: "identity and admission paths required",
			yaml: `
identity:
  name: node-a
storage:
  data_dir: /var/lib/transitloom/node
bootstrap_coordinators:
  - label: coord-a
    control_endpoints:
      - coord-a.example.net:8443
services:
  - name: wg0
    type: raw-udp
    binding:
      address: 127.0.0.1
      port: 51820
`,
			wantText: []string{
				"node_identity.certificate_path: must be set",
				"node_identity.private_key_path: must be set",
				"admission.current_token_path: must be set",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeTempConfig(t, tt.yaml)
			cfg, err := config.LoadNode(path)
			if err != nil {
				t.Fatalf("LoadNode() error = %v", err)
			}

			err = cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			for _, want := range tt.wantText {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("Validate() error = %q, want substring %q", err, want)
				}
			}
		})
	}
}
