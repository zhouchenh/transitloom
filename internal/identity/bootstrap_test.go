package identity_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/identity"
)

func TestInspectNodeIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		writeCertificate   bool
		writePrivateKey    bool
		wantPhase          identity.NodeIdentityPhase
		wantErr            string
		wantReportContains []string
	}{
		{
			name:             "ready with certificate and key",
			writeCertificate: true,
			writePrivateKey:  true,
			wantPhase:        identity.NodeIdentityPhaseReady,
			wantReportContains: []string{
				"node identity bootstrap state: ready",
				"node identity certificate:",
				"present",
			},
		},
		{
			name:      "bootstrap required when no material exists",
			wantPhase: identity.NodeIdentityPhaseBootstrapRequired,
			wantReportContains: []string{
				"bootstrap required",
				`from "identity/current.crt"`,
				"missing",
			},
		},
		{
			name:            "awaiting certificate when key exists",
			writePrivateKey: true,
			wantPhase:       identity.NodeIdentityPhaseAwaitingCertificate,
			wantReportContains: []string{
				"awaiting certificate issuance",
				"node identity private key:",
				"present",
			},
		},
		{
			name:             "certificate without key is invalid",
			writeCertificate: true,
			wantErr:          "node_identity.certificate_path",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dataDir := filepath.Join(t.TempDir(), "node-state")
			cfg := config.NodeConfig{
				Identity: config.IdentityMetadata{Name: "node-a"},
				Storage:  config.StorageConfig{DataDir: dataDir},
				NodeIdentity: config.NodeIdentityConfig{
					CertificatePath: "identity/current.crt",
					PrivateKeyPath:  "identity/current.key",
				},
			}

			if tt.writeCertificate {
				writeIdentityFile(t, filepath.Join(dataDir, "identity", "current.crt"))
			}
			if tt.writePrivateKey {
				writeIdentityFile(t, filepath.Join(dataDir, "identity", "current.key"))
			}

			state, err := identity.InspectNodeIdentity(cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("InspectNodeIdentity() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("InspectNodeIdentity() error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("InspectNodeIdentity() error = %v", err)
			}
			if state.Phase != tt.wantPhase {
				t.Fatalf("InspectNodeIdentity() phase = %q, want %q", state.Phase, tt.wantPhase)
			}

			wantResolved := filepath.Join(dataDir, "identity", "current.crt")
			if state.Certificate.ResolvedPath != wantResolved {
				t.Fatalf("InspectNodeIdentity() resolved certificate path = %q, want %q", state.Certificate.ResolvedPath, wantResolved)
			}

			report := strings.Join(state.ReportLines(), "\n")
			for _, want := range tt.wantReportContains {
				if !strings.Contains(report, want) {
					t.Fatalf("ReportLines() = %q, want substring %q", report, want)
				}
			}
		})
	}
}

func writeIdentityFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}
