package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhouchenh/transitloom/internal/config"
)

func TestLoadRootValid(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadRoot(filepath.Join("testdata", "root-valid.yaml"))
	if err != nil {
		t.Fatalf("LoadRoot() error = %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRootValidateInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		wantText []string
	}{
		{
			name: "missing required fields",
			yaml: `
storage:
  data_dir: /var/lib/transitloom/root
trust:
  root_cert_path: /var/lib/transitloom/root/root.crt
`,
			wantText: []string{
				"identity.name: must be set",
				"trust.root_key_path: must be set",
			},
		},
		{
			name: "admin endpoint required when enabled",
			yaml: `
identity:
  name: root-a
storage:
  data_dir: /var/lib/transitloom/root
trust:
  root_cert_path: /var/lib/transitloom/root/root.crt
  root_key_path: /var/lib/transitloom/root/root.key
admin:
  enabled: true
`,
			wantText: []string{
				"admin.listen: must be set when admin.enabled is true",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeTempConfig(t, tt.yaml)
			cfg, err := config.LoadRoot(path)
			if err != nil {
				t.Fatalf("LoadRoot() error = %v", err)
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

func TestLoadRootRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	path := writeTempConfig(t, `
identity:
  name: root-a
storage:
  data_dir: /var/lib/transitloom/root
trust:
  root_cert_path: /var/lib/transitloom/root/root.crt
  root_key_path: /var/lib/transitloom/root/root.key
unknown_block: true
`)

	_, err := config.LoadRoot(path)
	if err == nil {
		t.Fatal("LoadRoot() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "field unknown_block not found") {
		t.Fatalf("LoadRoot() error = %q, want unknown field failure", err)
	}
}

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}
