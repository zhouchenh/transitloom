package coordinator_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
	"github.com/zhouchenh/transitloom/internal/controlplane"
	"github.com/zhouchenh/transitloom/internal/coordinator"
	"github.com/zhouchenh/transitloom/internal/pki"
)

// testPKIChain holds the test PKI material for use in secure listener tests.
type testPKIChain struct {
	RootMaterial  pki.RootCAMaterial
	IntermMaterial pki.IntermediateMaterial
	RootPool      interface{ AppendCertsFromPEM([]byte) bool }
}

// buildTestPKIChain generates a minimal PKI chain for testing.
func buildTestPKIChain(t *testing.T) (pki.RootCAMaterial, pki.IntermediateMaterial) {
	t.Helper()

	rootMaterial, err := pki.GenerateRootCA(pki.IssuanceConfig{
		CommonName: "test-root",
		ValidFor:   1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("GenerateRootCA() error = %v", err)
	}
	rootCert, err := pki.ParseCertificatePEM(rootMaterial.CertPEM)
	if err != nil {
		t.Fatalf("ParseCertificatePEM(root) error = %v", err)
	}
	rootKey, err := pki.ParseECPrivateKeyPEM(rootMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseECPrivateKeyPEM(root) error = %v", err)
	}

	intermMaterial, err := pki.GenerateCoordinatorIntermediate(pki.IssuanceConfig{
		CommonName: "test-coordinator",
		ValidFor:   1 * time.Hour,
		DNSNames:   []string{"test-coordinator", "localhost"},
	}, rootCert, rootKey)
	if err != nil {
		t.Fatalf("GenerateCoordinatorIntermediate() error = %v", err)
	}

	return rootMaterial, intermMaterial
}

// buildTestNodeCert generates a node certificate under the given intermediate.
func buildTestNodeCert(t *testing.T, intermMaterial pki.IntermediateMaterial, nodeName string) pki.NodeCertMaterial {
	t.Helper()

	intermCert, err := pki.ParseCertificatePEM(intermMaterial.CertPEM)
	if err != nil {
		t.Fatalf("ParseCertificatePEM(intermediate) error = %v", err)
	}
	intermKey, err := pki.ParseECPrivateKeyPEM(intermMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseECPrivateKeyPEM(intermediate) error = %v", err)
	}

	nodeMaterial, err := pki.GenerateNodeCertificate(pki.IssuanceConfig{
		CommonName: nodeName,
		ValidFor:   1 * time.Hour,
	}, intermCert, intermKey)
	if err != nil {
		t.Fatalf("GenerateNodeCertificate() error = %v", err)
	}
	return nodeMaterial
}

// startSecureListener creates and starts a SecureControlListener for tests.
// It returns the listener and a cleanup function.
func startSecureListener(t *testing.T, coordName string, tlsConfig *tls.Config) (*coordinator.SecureControlListener, func()) {
	t.Helper()

	cfg := config.CoordinatorConfig{
		Identity: config.IdentityMetadata{Name: coordName},
		Control: config.ControlTransportConfig{
			TCP: config.TransportListenerConfig{
				Enabled:         true,
				ListenEndpoints: []string{"127.0.0.1:0"},
			},
		},
	}
	bootstrapState := pki.CoordinatorBootstrapState{
		CoordinatorName: coordName,
		Phase:           pki.CoordinatorBootstrapPhaseReady,
	}

	ln, err := coordinator.NewSecureControlListener(cfg, bootstrapState, tlsConfig)
	if err != nil {
		t.Fatalf("NewSecureControlListener() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- ln.Run(ctx) }()

	cleanup := func() {
		cancel()
		if err := <-runErr; err != nil {
			t.Errorf("SecureControlListener.Run() cleanup error = %v", err)
		}
	}

	// Brief pause for the listener to be ready.
	time.Sleep(10 * time.Millisecond)

	return ln, cleanup
}

// TestSecureControlListenerTransportStatus verifies that the transport status
// reports SecureControlModeTLSMTCPFallback and Authenticated=true, making it
// explicitly distinct from the bootstrap-only HTTP mode.
func TestSecureControlListenerTransportStatus(t *testing.T) {
	t.Parallel()

	rootMaterial, intermMaterial := buildTestPKIChain(t)
	rootPool, err := pki.NewCertPool(rootMaterial.CertPEM)
	if err != nil {
		t.Fatalf("NewCertPool() error = %v", err)
	}

	serverCert, err := pki.ParseTLSCertificatePEM(intermMaterial.CertPEM, intermMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseTLSCertificatePEM() error = %v", err)
	}
	serverTLSConfig := pki.BuildCoordinatorTLSConfig(serverCert, rootPool)

	ln, cleanup := startSecureListener(t, "coord-a", serverTLSConfig)
	defer cleanup()

	status := ln.TransportStatus()
	if status.Mode != controlplane.SecureControlModeTLSMTCPFallback {
		t.Errorf("TransportStatus.Mode = %q, want %q", status.Mode, controlplane.SecureControlModeTLSMTCPFallback)
	}
	if !status.Authenticated {
		t.Error("TransportStatus.Authenticated = false, want true")
	}
	// Report lines must include the mode.
	reportLines := status.ReportLines()
	joined := strings.Join(reportLines, "\n")
	if !strings.Contains(joined, string(controlplane.SecureControlModeTLSMTCPFallback)) {
		t.Errorf("transport status report lines do not mention mode %q:\n%s", controlplane.SecureControlModeTLSMTCPFallback, joined)
	}
	// Report lines must explicitly say "mutually-authenticated".
	if !strings.Contains(joined, "mutually-authenticated") {
		t.Errorf("transport status report lines do not say 'mutually-authenticated':\n%s", joined)
	}
}

// TestSecureControlListenerReportLines verifies that the report lines from
// SecureControlListener honestly describe the TLS mode and include the note
// that application-layer admission enforcement is not yet implemented.
func TestSecureControlListenerReportLines(t *testing.T) {
	t.Parallel()

	rootMaterial, intermMaterial := buildTestPKIChain(t)
	rootPool, err := pki.NewCertPool(rootMaterial.CertPEM)
	if err != nil {
		t.Fatalf("NewCertPool() error = %v", err)
	}
	serverCert, _ := pki.ParseTLSCertificatePEM(intermMaterial.CertPEM, intermMaterial.KeyPEM)
	serverTLSConfig := pki.BuildCoordinatorTLSConfig(serverCert, rootPool)

	ln, cleanup := startSecureListener(t, "coord-report", serverTLSConfig)
	defer cleanup()

	lines := ln.ReportLines()
	joined := strings.Join(lines, "\n")

	// Must mention TLS 1.3 mTLS.
	if !strings.Contains(joined, "TLS 1.3 mTLS") {
		t.Errorf("report lines do not mention TLS 1.3 mTLS:\n%s", joined)
	}
	// Must honestly note that admission-token enforcement is not yet done.
	if !strings.Contains(joined, "admission-token enforcement") {
		t.Errorf("report lines do not mention admission-token enforcement status:\n%s", joined)
	}
}

// TestSecureControlListenerBootstrapExchangeOverTLS verifies that a bootstrap
// session request can be exchanged successfully over the TLS-authenticated
// control transport. This is the end-to-end test for T-0021: actual TLS mTLS
// with a real bootstrap session exchange.
func TestSecureControlListenerBootstrapExchangeOverTLS(t *testing.T) {
	t.Parallel()

	rootMaterial, intermMaterial := buildTestPKIChain(t)
	rootPool, err := pki.NewCertPool(rootMaterial.CertPEM)
	if err != nil {
		t.Fatalf("NewCertPool() error = %v", err)
	}

	serverCert, err := pki.ParseTLSCertificatePEM(intermMaterial.CertPEM, intermMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseTLSCertificatePEM(coordinator) error = %v", err)
	}
	serverTLSConfig := pki.BuildCoordinatorTLSConfig(serverCert, rootPool)

	ln, cleanup := startSecureListener(t, "coord-tls", serverTLSConfig)
	defer cleanup()

	// Generate node cert and build client TLS config.
	nodeMaterial := buildTestNodeCert(t, intermMaterial, "node-tls")
	nodeChainPEM := append(nodeMaterial.CertPEM, intermMaterial.CertPEM...)
	clientTLSCert, err := pki.ParseTLSCertificatePEM(nodeChainPEM, nodeMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseTLSCertificatePEM(node chain) error = %v", err)
	}
	clientTLSConfig := pki.BuildNodeTLSConfig(clientTLSCert, rootPool, "localhost")

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: clientTLSConfig,
		},
	}

	// Build and send a bootstrap session request over TLS.
	request := controlplane.BootstrapSessionRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        "node-tls",
		Readiness: controlplane.BootstrapReadinessSummary{
			OverallPhase:   controlplane.ReadinessPhaseReady,
			IdentityPhase:  "ready",
			AdmissionPhase: "ready",
			CachedToken: &controlplane.BootstrapTokenSummary{
				TokenID:             "tok-test-001",
				NodeID:              "node-tls",
				IssuerCoordinatorID: "coord-tls",
				IssuedAt:            time.Now().Add(-1 * time.Hour),
				ExpiresAt:           time.Now().Add(23 * time.Hour),
			},
		},
	}

	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(request) error = %v", err)
	}

	endpoint := ln.BoundEndpoints()[0]
	url := fmt.Sprintf("https://%s%s", endpoint, controlplane.BootstrapSessionPath)

	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s error = %v (TLS handshake may have failed)", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s status = %d, want 200", url, resp.StatusCode)
	}

	var response controlplane.BootstrapSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("decode response error = %v", err)
	}

	if !response.Accepted() {
		t.Errorf("response.Accepted() = false, want true; outcome=%s reason=%s", response.Outcome, response.Reason)
	}
	if !response.BootstrapOnly {
		t.Error("response.BootstrapOnly = false, want true (bootstrap-only semantics remain even over TLS)")
	}
}

// TestSecureControlListenerRejectsUnauthenticatedClient verifies that a client
// without a TLS certificate is rejected. This is the mTLS enforcement contract:
// the TLS layer rejects connections before any application-layer data is read.
func TestSecureControlListenerRejectsUnauthenticatedClient(t *testing.T) {
	t.Parallel()

	rootMaterial, intermMaterial := buildTestPKIChain(t)
	rootPool, err := pki.NewCertPool(rootMaterial.CertPEM)
	if err != nil {
		t.Fatalf("NewCertPool() error = %v", err)
	}
	serverCert, _ := pki.ParseTLSCertificatePEM(intermMaterial.CertPEM, intermMaterial.KeyPEM)
	serverTLSConfig := pki.BuildCoordinatorTLSConfig(serverCert, rootPool)

	ln, cleanup := startSecureListener(t, "coord-mtls", serverTLSConfig)
	defer cleanup()

	// Client without a certificate — no Certificates field set.
	noCertClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS13,
				RootCAs:    rootPool,
				ServerName: "localhost",
			},
		},
	}

	endpoint := ln.BoundEndpoints()[0]
	_, err = noCertClient.Post(
		fmt.Sprintf("https://%s%s", endpoint, controlplane.BootstrapSessionPath),
		"application/json",
		bytes.NewReader([]byte("{}")),
	)
	if err == nil {
		t.Error("POST without client cert succeeded, want error (mTLS requires client certificate)")
	}
}

// TestNewSecureControlListenerRejectsNilTLSConfig verifies that
// NewSecureControlListener returns an error when passed a nil TLS config,
// since that would create a plaintext listener indistinguishable from the
// bootstrap-only HTTP listener.
func TestNewSecureControlListenerRejectsNilTLSConfig(t *testing.T) {
	t.Parallel()

	cfg := config.CoordinatorConfig{
		Identity: config.IdentityMetadata{Name: "coord-a"},
		Control: config.ControlTransportConfig{
			TCP: config.TransportListenerConfig{
				Enabled:         true,
				ListenEndpoints: []string{"127.0.0.1:0"},
			},
		},
	}
	_, err := coordinator.NewSecureControlListener(cfg, pki.CoordinatorBootstrapState{
		CoordinatorName: "coord-a",
		Phase:           pki.CoordinatorBootstrapPhaseReady,
	}, nil)
	if err == nil {
		t.Error("NewSecureControlListener(nil tlsConfig) = nil, want error")
	}
}

// TestSecureControlListenerBootstrapOnlyFlagRetained verifies that even over
// TLS 1.3 mTLS, the bootstrap-only flag in responses remains true. This is
// intentional: the transport is now authenticated, but the application-layer
// session semantics are still bootstrap-only until full admission-token
// enforcement is implemented.
func TestSecureControlListenerBootstrapOnlyFlagRetained(t *testing.T) {
	t.Parallel()

	rootMaterial, intermMaterial := buildTestPKIChain(t)
	rootPool, _ := pki.NewCertPool(rootMaterial.CertPEM)
	serverCert, _ := pki.ParseTLSCertificatePEM(intermMaterial.CertPEM, intermMaterial.KeyPEM)
	serverTLSConfig := pki.BuildCoordinatorTLSConfig(serverCert, rootPool)

	ln, cleanup := startSecureListener(t, "coord-btflag", serverTLSConfig)
	defer cleanup()

	nodeMaterial := buildTestNodeCert(t, intermMaterial, "node-btflag")
	nodeChainPEM := append(nodeMaterial.CertPEM, intermMaterial.CertPEM...)
	clientCert, _ := pki.ParseTLSCertificatePEM(nodeChainPEM, nodeMaterial.KeyPEM)
	clientTLSConfig := pki.BuildNodeTLSConfig(clientCert, rootPool, "localhost")

	httpClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{TLSClientConfig: clientTLSConfig},
	}

	request := controlplane.BootstrapSessionRequest{
		ProtocolVersion: controlplane.BootstrapProtocolVersion,
		NodeName:        "node-btflag",
		Readiness: controlplane.BootstrapReadinessSummary{
			OverallPhase:   controlplane.ReadinessPhaseReady,
			IdentityPhase:  "ready",
			AdmissionPhase: "ready",
			CachedToken: &controlplane.BootstrapTokenSummary{
				TokenID:             "tok-btflag",
				NodeID:              "node-btflag",
				IssuerCoordinatorID: "coord-btflag",
				IssuedAt:            time.Now().Add(-1 * time.Hour),
				ExpiresAt:           time.Now().Add(23 * time.Hour),
			},
		},
	}
	payload, _ := json.Marshal(request)
	endpoint := ln.BoundEndpoints()[0]

	resp, err := httpClient.Post(
		fmt.Sprintf("https://%s%s", endpoint, controlplane.BootstrapSessionPath),
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	defer resp.Body.Close()

	var response controlplane.BootstrapSessionResponse
	json.NewDecoder(resp.Body).Decode(&response) //nolint:errcheck

	// BootstrapOnly must remain true even over TLS: the application-layer
	// semantics are still bootstrap-only until admission enforcement is done.
	if !response.BootstrapOnly {
		t.Error("response.BootstrapOnly = false, want true; " +
			"TLS transport authentication is separate from application-layer admission state")
	}
}
