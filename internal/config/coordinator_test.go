package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhouchenh/transitloom/internal/config"
)

func TestLoadCoordinatorValid(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadCoordinator(filepath.Join("testdata", "coordinator-valid.yaml"))
	if err != nil {
		t.Fatalf("LoadCoordinator() error = %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestCoordinatorValidateInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		wantText []string
	}{
		{
			name: "control transport required",
			yaml: `
identity:
  name: coord-a
storage:
  data_dir: /var/lib/transitloom/coordinator
control:
  quic:
    enabled: false
  tcp:
    enabled: false
trust:
  root_anchor_path: /var/lib/transitloom/coordinator/root.crt
  intermediate_cert_path: /var/lib/transitloom/coordinator/intermediate.crt
  intermediate_key_path: /var/lib/transitloom/coordinator/intermediate.key
`,
			wantText: []string{
				"control: must enable at least one control transport",
			},
		},
		{
			name: "relay endpoint required when relay enabled",
			yaml: `
identity:
  name: coord-a
storage:
  data_dir: /var/lib/transitloom/coordinator
control:
  quic:
    enabled: true
    listen_endpoints:
      - ":8443"
  tcp:
    enabled: false
trust:
  root_anchor_path: /var/lib/transitloom/coordinator/root.crt
  intermediate_cert_path: /var/lib/transitloom/coordinator/intermediate.crt
  intermediate_key_path: /var/lib/transitloom/coordinator/intermediate.key
relay:
  control_enabled: true
`,
			wantText: []string{
				"relay.listen_endpoints: must contain at least one endpoint when relay is enabled",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeTempConfig(t, tt.yaml)
			cfg, err := config.LoadCoordinator(path)
			if err != nil {
				t.Fatalf("LoadCoordinator() error = %v", err)
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
