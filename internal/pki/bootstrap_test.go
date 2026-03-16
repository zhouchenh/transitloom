package pki_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/pki"
)

func TestInspectRootBootstrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		generateKey        bool
		writeCert          bool
		writeKey           bool
		wantPhase          pki.RootBootstrapPhase
		wantErr            string
		wantReportContains []string
	}{
		{
			name:        "ready with existing trust anchor",
			generateKey: false,
			writeCert:   true,
			writeKey:    true,
			wantPhase:   pki.RootBootstrapPhaseReady,
			wantReportContains: []string{
				"root bootstrap state: ready",
				"root trust anchor certificate:",
				"present",
			},
		},
		{
			name:        "initialization required when generation requested",
			generateKey: true,
			wantPhase:   pki.RootBootstrapPhaseInitializationReady,
			wantReportContains: []string{
				`from "trust/root.crt"`,
				"initialization required",
				"missing",
			},
		},
		{
			name:        "missing material rejected when generation disabled",
			generateKey: false,
			wantErr:     "trust.generate_key is false",
		},
		{
			name:        "partial material rejected",
			generateKey: true,
			writeCert:   true,
			wantErr:     "trust.root_cert_path",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dataDir := filepath.Join(t.TempDir(), "root-state")
			cfg := config.RootConfig{
				Identity: config.IdentityMetadata{Name: "root-a"},
				Storage:  config.StorageConfig{DataDir: dataDir},
				Trust: config.RootTrustConfig{
					RootCertPath: "trust/root.crt",
					RootKeyPath:  "trust/root.key",
					GenerateKey:  tt.generateKey,
				},
			}

			if tt.writeCert {
				writeMaterialFile(t, filepath.Join(dataDir, "trust", "root.crt"))
			}
			if tt.writeKey {
				writeMaterialFile(t, filepath.Join(dataDir, "trust", "root.key"))
			}

			state, err := pki.InspectRootBootstrap(cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("InspectRootBootstrap() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("InspectRootBootstrap() error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("InspectRootBootstrap() error = %v", err)
			}
			if state.Phase != tt.wantPhase {
				t.Fatalf("InspectRootBootstrap() phase = %q, want %q", state.Phase, tt.wantPhase)
			}

			wantResolved := filepath.Join(dataDir, "trust", "root.crt")
			if state.RootCert.ResolvedPath != wantResolved {
				t.Fatalf("InspectRootBootstrap() resolved cert path = %q, want %q", state.RootCert.ResolvedPath, wantResolved)
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

func TestInspectCoordinatorBootstrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		writeRootAnchor    bool
		writeIntermediate  bool
		writeIntermediateK bool
		wantPhase          pki.CoordinatorBootstrapPhase
		wantErr            string
		wantReportContains []string
	}{
		{
			name:               "ready with intermediate",
			writeRootAnchor:    true,
			writeIntermediate:  true,
			writeIntermediateK: true,
			wantPhase:          pki.CoordinatorBootstrapPhaseReady,
			wantReportContains: []string{"coordinator trust bootstrap state: ready", "coordinator root trust anchor:", "present"},
		},
		{
			name:            "awaiting intermediate material",
			writeRootAnchor: true,
			wantPhase:       pki.CoordinatorBootstrapPhaseAwaitingIntermediate,
			wantReportContains: []string{
				"awaiting intermediate material from the root authority",
				`from "intermediates/current.crt"`,
				"missing",
			},
		},
		{
			name:    "missing root anchor rejected",
			wantErr: "trust.root_anchor_path",
		},
		{
			name:              "partial intermediate rejected",
			writeRootAnchor:   true,
			writeIntermediate: true,
			wantErr:           "trust.intermediate_cert_path",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dataDir := filepath.Join(t.TempDir(), "coordinator-state")
			cfg := config.CoordinatorConfig{
				Identity: config.IdentityMetadata{Name: "coord-a"},
				Storage:  config.StorageConfig{DataDir: dataDir},
				Trust: config.CoordinatorTrustConfig{
					RootAnchorPath:       "anchors/root.crt",
					IntermediateCertPath: "intermediates/current.crt",
					IntermediateKeyPath:  "intermediates/current.key",
				},
			}

			if tt.writeRootAnchor {
				writeMaterialFile(t, filepath.Join(dataDir, "anchors", "root.crt"))
			}
			if tt.writeIntermediate {
				writeMaterialFile(t, filepath.Join(dataDir, "intermediates", "current.crt"))
			}
			if tt.writeIntermediateK {
				writeMaterialFile(t, filepath.Join(dataDir, "intermediates", "current.key"))
			}

			state, err := pki.InspectCoordinatorBootstrap(cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("InspectCoordinatorBootstrap() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("InspectCoordinatorBootstrap() error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("InspectCoordinatorBootstrap() error = %v", err)
			}
			if state.Phase != tt.wantPhase {
				t.Fatalf("InspectCoordinatorBootstrap() phase = %q, want %q", state.Phase, tt.wantPhase)
			}

			wantResolved := filepath.Join(dataDir, "anchors", "root.crt")
			if state.RootAnchor.ResolvedPath != wantResolved {
				t.Fatalf("InspectCoordinatorBootstrap() resolved root anchor path = %q, want %q", state.RootAnchor.ResolvedPath, wantResolved)
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

func writeMaterialFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}
