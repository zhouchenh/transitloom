package admission_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/admission"
	"github.com/zhouchenh/transitloom/internal/config"
)

func TestInspectTokenCache(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name               string
		token              *admission.CachedTokenRecord
		rawToken           string
		wantPhase          admission.TokenCachePhase
		wantErr            string
		wantReportContains []string
	}{
		{
			name:      "missing token cache is allowed",
			wantPhase: admission.TokenCachePhaseMissing,
			wantReportContains: []string{
				"no locally cached current admission token",
				`from "admission/current-token.json"`,
				"missing",
			},
		},
		{
			name: "usable cached token",
			token: &admission.CachedTokenRecord{
				TokenID:             "tok-1",
				NodeID:              "node-1",
				IssuerCoordinatorID: "coord-a",
				IssuedAt:            now.Add(-30 * time.Minute),
				ExpiresAt:           now.Add(30 * time.Minute),
			},
			wantPhase: admission.TokenCachePhaseUsable,
			wantReportContains: []string{
				"usable cached current admission token",
				"token_id=tok-1",
				"issuer=coord-a",
			},
		},
		{
			name: "expired cached token",
			token: &admission.CachedTokenRecord{
				TokenID:             "tok-2",
				NodeID:              "node-1",
				IssuerCoordinatorID: "coord-a",
				IssuedAt:            now.Add(-2 * time.Hour),
				ExpiresAt:           now.Add(-1 * time.Minute),
			},
			wantPhase: admission.TokenCachePhaseExpired,
			wantReportContains: []string{
				"expired at",
				"token_id=tok-2",
			},
		},
		{
			name:     "malformed token file is rejected",
			rawToken: `{"token_id":123}`,
			wantErr:  "decode cached admission token",
		},
		{
			name: "invalid token metadata is rejected",
			token: &admission.CachedTokenRecord{
				TokenID:             "tok-3",
				NodeID:              "",
				IssuerCoordinatorID: "coord-a",
				IssuedAt:            now,
				ExpiresAt:           now.Add(time.Hour),
			},
			wantErr: "must set node_id",
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
				Admission: config.NodeAdmissionConfig{
					CurrentTokenPath: "admission/current-token.json",
				},
			}

			if tt.token != nil || tt.rawToken != "" {
				tokenPath := filepath.Join(dataDir, "admission", "current-token.json")
				writeTokenFile(t, tokenPath, tt.token, tt.rawToken)
			}

			state, err := admission.InspectTokenCache(cfg, now)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("InspectTokenCache() error = nil, want non-nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("InspectTokenCache() error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("InspectTokenCache() error = %v", err)
			}
			if state.Phase != tt.wantPhase {
				t.Fatalf("InspectTokenCache() phase = %q, want %q", state.Phase, tt.wantPhase)
			}

			wantResolved := filepath.Join(dataDir, "admission", "current-token.json")
			if state.Cache.ResolvedPath != wantResolved {
				t.Fatalf("InspectTokenCache() resolved token path = %q, want %q", state.Cache.ResolvedPath, wantResolved)
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

func writeTokenFile(t *testing.T, path string, token *admission.CachedTokenRecord, raw string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	var contents []byte
	if raw != "" {
		contents = []byte(raw)
	} else {
		data, err := json.Marshal(token)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		contents = data
	}

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}
