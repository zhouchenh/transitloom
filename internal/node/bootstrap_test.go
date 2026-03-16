package node_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/admission"
	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/node"
)

func TestInspectBootstrap(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name               string
		writeCertificate   bool
		writePrivateKey    bool
		token              *admission.CachedTokenRecord
		wantPhase          node.BootstrapPhase
		wantErr            string
		wantReportContains []string
	}{
		{
			name:      "identity bootstrap required without local state",
			wantPhase: node.BootstrapPhaseIdentityBootstrapRequired,
			wantReportContains: []string{
				"node bootstrap readiness: identity bootstrap required",
				"node identity bootstrap state:",
			},
		},
		{
			name:            "awaiting certificate when only key exists",
			writePrivateKey: true,
			wantPhase:       node.BootstrapPhaseAwaitingCertificate,
			wantReportContains: []string{
				"needs certificate issuance",
				"awaiting certificate issuance",
			},
		},
		{
			name:             "identity ready but token missing",
			writeCertificate: true,
			writePrivateKey:  true,
			wantPhase:        node.BootstrapPhaseAdmissionMissing,
			wantReportContains: []string{
				"no cached current admission token is available",
				"node admission bootstrap state:",
			},
		},
		{
			name:             "identity ready but token expired",
			writeCertificate: true,
			writePrivateKey:  true,
			token: &admission.CachedTokenRecord{
				TokenID:             "tok-1",
				NodeID:              "node-1",
				IssuerCoordinatorID: "coord-a",
				IssuedAt:            now.Add(-2 * time.Hour),
				ExpiresAt:           now.Add(-time.Minute),
			},
			wantPhase: node.BootstrapPhaseAdmissionExpired,
			wantReportContains: []string{
				"cached current admission token is expired",
				"token_id=tok-1",
			},
		},
		{
			name:             "identity and token ready",
			writeCertificate: true,
			writePrivateKey:  true,
			token: &admission.CachedTokenRecord{
				TokenID:             "tok-2",
				NodeID:              "node-1",
				IssuerCoordinatorID: "coord-a",
				IssuedAt:            now.Add(-10 * time.Minute),
				ExpiresAt:           now.Add(time.Hour),
			},
			wantPhase: node.BootstrapPhaseReady,
			wantReportContains: []string{
				"cached current admission token is present",
				"token_id=tok-2",
			},
		},
		{
			name:    "token without ready identity is rejected",
			token:   &admission.CachedTokenRecord{TokenID: "tok-3", NodeID: "node-1", IssuerCoordinatorID: "coord-a", IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)},
			wantErr: "admission.current_token_path",
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
				Admission: config.NodeAdmissionConfig{
					CurrentTokenPath: "admission/current-token.json",
				},
			}

			if tt.writeCertificate {
				writeBootstrapFile(t, filepath.Join(dataDir, "identity", "current.crt"), []byte("placeholder"))
			}
			if tt.writePrivateKey {
				writeBootstrapFile(t, filepath.Join(dataDir, "identity", "current.key"), []byte("placeholder"))
			}
			if tt.token != nil {
				data, err := json.Marshal(tt.token)
				if err != nil {
					t.Fatalf("json.Marshal() error = %v", err)
				}
				writeBootstrapFile(t, filepath.Join(dataDir, "admission", "current-token.json"), data)
			}

			state, err := node.InspectBootstrap(cfg, now)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("InspectBootstrap() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("InspectBootstrap() error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("InspectBootstrap() error = %v", err)
			}
			if state.Phase != tt.wantPhase {
				t.Fatalf("InspectBootstrap() phase = %q, want %q", state.Phase, tt.wantPhase)
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

func writeBootstrapFile(t *testing.T, path string, contents []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}
