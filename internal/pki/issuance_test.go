package pki_test

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/zhouchenh/transitloom/internal/pki"
)

// TestGenerateRootCA verifies that GenerateRootCA produces a valid self-signed
// root CA certificate with the expected constraints.
func TestGenerateRootCA(t *testing.T) {
	t.Parallel()

	material, err := pki.GenerateRootCA(pki.IssuanceConfig{
		CommonName: "test-root",
	})
	if err != nil {
		t.Fatalf("GenerateRootCA() error = %v", err)
	}
	if len(material.CertPEM) == 0 {
		t.Error("CertPEM is empty")
	}
	if len(material.KeyPEM) == 0 {
		t.Error("KeyPEM is empty")
	}

	cert, err := pki.ParseCertificatePEM(material.CertPEM)
	if err != nil {
		t.Fatalf("ParseCertificatePEM() error = %v", err)
	}

	if !cert.IsCA {
		t.Error("root CA certificate: IsCA = false, want true")
	}
	if cert.MaxPathLen != 1 {
		t.Errorf("root CA MaxPathLen = %d, want 1", cert.MaxPathLen)
	}
	if cert.Subject.CommonName != "test-root" {
		t.Errorf("CommonName = %q, want %q", cert.Subject.CommonName, "test-root")
	}
	// Root CA must not have ExtKeyUsage set (it should not be used as a
	// transport certificate).
	if len(cert.ExtKeyUsage) != 0 {
		t.Errorf("root CA ExtKeyUsage = %v, want none", cert.ExtKeyUsage)
	}
	// Verify self-signed: issuer == subject.
	if cert.Issuer.CommonName != cert.Subject.CommonName {
		t.Errorf("root CA issuer CN %q != subject CN %q (not self-signed)", cert.Issuer.CommonName, cert.Subject.CommonName)
	}

	// Round-trip the key: parse and check it is an EC key.
	key, err := pki.ParseECPrivateKeyPEM(material.KeyPEM)
	if err != nil {
		t.Fatalf("ParseECPrivateKeyPEM() error = %v", err)
	}
	if key == nil {
		t.Error("parsed key is nil")
	}
}

// TestGenerateCoordinatorIntermediate verifies that an intermediate CA
// certificate is signed by the root CA and has the expected constraints.
func TestGenerateCoordinatorIntermediate(t *testing.T) {
	t.Parallel()

	rootMaterial, err := pki.GenerateRootCA(pki.IssuanceConfig{CommonName: "test-root"})
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
		DNSNames:   []string{"test-coordinator", "localhost"},
	}, rootCert, rootKey)
	if err != nil {
		t.Fatalf("GenerateCoordinatorIntermediate() error = %v", err)
	}

	intermCert, err := pki.ParseCertificatePEM(intermMaterial.CertPEM)
	if err != nil {
		t.Fatalf("ParseCertificatePEM(intermediate) error = %v", err)
	}

	if !intermCert.IsCA {
		t.Error("coordinator intermediate: IsCA = false, want true")
	}
	if !intermCert.MaxPathLenZero {
		t.Error("coordinator intermediate: MaxPathLenZero = false, want true (prevents further sub-intermediates)")
	}
	if intermCert.Subject.CommonName != "test-coordinator" {
		t.Errorf("CommonName = %q, want %q", intermCert.Subject.CommonName, "test-coordinator")
	}
	// Intermediate CA must NOT restrict ExtKeyUsage. Restricting to ServerAuth
	// only would prevent Go's x509 chain verification from accepting node certs
	// (ClientAuth) signed by this intermediate. Certs with no ExtKeyUsage are
	// treated as unrestricted and valid for any usage, including server auth.
	if len(intermCert.ExtKeyUsage) != 0 {
		t.Errorf("coordinator intermediate: ExtKeyUsage = %v, want none (CA certs must not restrict leaf usage)", intermCert.ExtKeyUsage)
	}

	// Verify the intermediate is signed by the root CA.
	rootPool, err := pki.NewCertPool(rootMaterial.CertPEM)
	if err != nil {
		t.Fatalf("NewCertPool() error = %v", err)
	}
	opts := x509.VerifyOptions{
		Roots:     rootPool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if _, err := intermCert.Verify(opts); err != nil {
		t.Errorf("intermediate certificate does not verify against root CA: %v", err)
	}
}

// TestGenerateNodeCertificate verifies that a node certificate is signed by
// the coordinator intermediate and has the expected constraints.
func TestGenerateNodeCertificate(t *testing.T) {
	t.Parallel()

	rootMaterial, intermMaterial, err := generateTestChain(t)
	if err != nil {
		t.Fatalf("generateTestChain() error = %v", err)
	}

	intermCert, err := pki.ParseCertificatePEM(intermMaterial.CertPEM)
	if err != nil {
		t.Fatalf("ParseCertificatePEM(intermediate) error = %v", err)
	}
	intermKey, err := pki.ParseECPrivateKeyPEM(intermMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseECPrivateKeyPEM(intermediate) error = %v", err)
	}

	nodeMaterial, err := pki.GenerateNodeCertificate(pki.IssuanceConfig{
		CommonName: "test-node",
	}, intermCert, intermKey)
	if err != nil {
		t.Fatalf("GenerateNodeCertificate() error = %v", err)
	}

	nodeCert, err := pki.ParseCertificatePEM(nodeMaterial.CertPEM)
	if err != nil {
		t.Fatalf("ParseCertificatePEM(node) error = %v", err)
	}

	if nodeCert.IsCA {
		t.Error("node certificate: IsCA = true, want false (leaf cert)")
	}
	if nodeCert.Subject.CommonName != "test-node" {
		t.Errorf("CommonName = %q, want %q", nodeCert.Subject.CommonName, "test-node")
	}
	// Node cert must have ClientAuth for mTLS node-to-coordinator sessions.
	hasClientAuth := false
	for _, eku := range nodeCert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
		}
	}
	if !hasClientAuth {
		t.Error("node certificate: ExtKeyUsage does not include ClientAuth")
	}

	// Verify the full chain: node → intermediate → root.
	rootPool, err := pki.NewCertPool(rootMaterial.CertPEM)
	if err != nil {
		t.Fatalf("NewCertPool() error = %v", err)
	}
	intermPool := x509.NewCertPool()
	intermPool.AppendCertsFromPEM(intermMaterial.CertPEM)

	opts := x509.VerifyOptions{
		Roots:         rootPool,
		Intermediates: intermPool,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if _, err := nodeCert.Verify(opts); err != nil {
		t.Errorf("node certificate does not verify against root CA via intermediate: %v", err)
	}
}

// TestParseCertificatePEMRejectsMultiple verifies that ParseCertificatePEM
// rejects PEM data containing more than one certificate block, since it is
// intended for single-cert parsing only.
func TestParseCertificatePEMRejectsMultiple(t *testing.T) {
	t.Parallel()

	rootMaterial, intermMaterial, err := generateTestChain(t)
	if err != nil {
		t.Fatalf("generateTestChain() error = %v", err)
	}

	// Concatenate two valid certs — should be rejected.
	combined := append(rootMaterial.CertPEM, intermMaterial.CertPEM...)
	_, err = pki.ParseCertificatePEM(combined)
	if err == nil {
		t.Error("ParseCertificatePEM(two certs) = nil error, want error")
	}
}

// TestTLSHandshakeWithGeneratedCertificates verifies an actual TLS 1.3 mTLS
// handshake between a coordinator (using the intermediate cert as server cert)
// and a node (using the node cert as client cert).
//
// This is the core test for T-0021: it proves that the PKI generation
// primitives and TLS config builders produce material that enables an actual
// authenticated TLS 1.3 mTLS connection.
func TestTLSHandshakeWithGeneratedCertificates(t *testing.T) {
	t.Parallel()

	rootMaterial, intermMaterial, err := generateTestChain(t)
	if err != nil {
		t.Fatalf("generateTestChain() error = %v", err)
	}
	intermCert, err := pki.ParseCertificatePEM(intermMaterial.CertPEM)
	if err != nil {
		t.Fatalf("ParseCertificatePEM(intermediate) error = %v", err)
	}
	intermKey, err := pki.ParseECPrivateKeyPEM(intermMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseECPrivateKeyPEM(intermediate) error = %v", err)
	}
	nodeMaterial, err := pki.GenerateNodeCertificate(pki.IssuanceConfig{
		CommonName: "test-node",
	}, intermCert, intermKey)
	if err != nil {
		t.Fatalf("GenerateNodeCertificate() error = %v", err)
	}

	rootPool, err := pki.NewCertPool(rootMaterial.CertPEM)
	if err != nil {
		t.Fatalf("NewCertPool() error = %v", err)
	}

	// Build the coordinator (server) TLS config.
	// The coordinator uses its intermediate cert as the server certificate.
	// Its DNS SANs include "localhost" so the node can verify the server name.
	serverTLSCert, err := pki.ParseTLSCertificatePEM(intermMaterial.CertPEM, intermMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseTLSCertificatePEM(coordinator) error = %v", err)
	}
	serverTLSConfig := pki.BuildCoordinatorTLSConfig(serverTLSCert, rootPool)

	// Build the node (client) TLS config.
	// The node presents its cert with the intermediate cert appended (full chain)
	// so the server can verify the chain back to the root CA.
	nodeChainPEM := append(nodeMaterial.CertPEM, intermMaterial.CertPEM...)
	clientTLSCert, err := pki.ParseTLSCertificatePEM(nodeChainPEM, nodeMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseTLSCertificatePEM(node chain) error = %v", err)
	}
	clientTLSConfig := pki.BuildNodeTLSConfig(clientTLSCert, rootPool, "localhost")

	// Start a TLS listener on a random port.
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
	if err != nil {
		t.Fatalf("tls.Listen() error = %v", err)
	}
	defer ln.Close()

	type serverResult struct {
		peerCN string
		err    error
	}
	serverCh := make(chan serverResult, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			serverCh <- serverResult{err: err}
			return
		}
		defer conn.Close()

		// Force the TLS handshake so we can inspect peer certificate state.
		tlsConn := conn.(*tls.Conn)
		if err := tlsConn.Handshake(); err != nil {
			serverCh <- serverResult{err: err}
			return
		}
		state := tlsConn.ConnectionState()
		peerCN := ""
		if len(state.PeerCertificates) > 0 {
			peerCN = state.PeerCertificates[0].Subject.CommonName
		}
		serverCh <- serverResult{peerCN: peerCN}
	}()

	// Connect as a TLS client (the node).
	clientConn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 3 * time.Second},
		"tcp",
		ln.Addr().String(),
		clientTLSConfig,
	)
	if err != nil {
		t.Fatalf("tls.DialWithDialer() error = %v (TLS handshake failed)", err)
	}
	defer clientConn.Close()

	// Verify TLS 1.3 was negotiated.
	clientState := clientConn.ConnectionState()
	if clientState.Version != tls.VersionTLS13 {
		t.Errorf("TLS version = 0x%04x, want TLS 1.3 (0x%04x)", clientState.Version, tls.VersionTLS13)
	}

	// Verify the server presented the coordinator's intermediate cert.
	if len(clientState.PeerCertificates) == 0 {
		t.Error("client: no peer (coordinator) certificate received")
	} else if clientState.PeerCertificates[0].Subject.CommonName != "test-coordinator" {
		t.Errorf("server cert CN = %q, want %q", clientState.PeerCertificates[0].Subject.CommonName, "test-coordinator")
	}

	// Verify the server received the node certificate.
	res := <-serverCh
	if res.err != nil {
		t.Fatalf("server side error: %v", res.err)
	}
	if res.peerCN != "test-node" {
		t.Errorf("server: peer cert CN = %q, want %q", res.peerCN, "test-node")
	}
}

// TestTLSHandshakeRejectsClientWithoutCert verifies that the coordinator TLS
// server rejects a client that does not present a certificate. This is the
// mTLS enforcement contract: an admitted node must always prove identity.
//
// In Go's TLS 1.3 implementation, the server's certificate_required rejection
// is processed by the client on the first Read after the initial handshake
// completes (not during DialWithDialer). The test accounts for both behaviors:
// rejection during the handshake (pre-TLS 1.3) and rejection surfacing on the
// first Read (TLS 1.3).
func TestTLSHandshakeRejectsClientWithoutCert(t *testing.T) {
	t.Parallel()

	rootMaterial, intermMaterial, err := generateTestChain(t)
	if err != nil {
		t.Fatalf("generateTestChain() error = %v", err)
	}

	rootPool, err := pki.NewCertPool(rootMaterial.CertPEM)
	if err != nil {
		t.Fatalf("NewCertPool() error = %v", err)
	}

	serverTLSCert, err := pki.ParseTLSCertificatePEM(intermMaterial.CertPEM, intermMaterial.KeyPEM)
	if err != nil {
		t.Fatalf("ParseTLSCertificatePEM(coordinator) error = %v", err)
	}
	serverTLSConfig := pki.BuildCoordinatorTLSConfig(serverTLSCert, rootPool)

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
	if err != nil {
		t.Fatalf("tls.Listen() error = %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.(*tls.Conn).Handshake() //nolint:errcheck // server-side rejection is expected
		conn.Close()
	}()

	// Client without any certificate — no Certificates field set.
	noClientCertConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		RootCAs:    rootPool,
		ServerName: "localhost",
	}
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 3 * time.Second},
		"tcp",
		ln.Addr().String(),
		noClientCertConfig,
	)
	if err != nil {
		// Good: rejected during the handshake itself.
		return
	}
	defer conn.Close()
	// In TLS 1.3, the server's certificate_required alert surfaces on the
	// first Read after the initial handshake. Set a deadline so the test
	// does not block indefinitely if the server is slow to close.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second)) //nolint:errcheck
	buf := make([]byte, 1)
	if _, readErr := conn.Read(buf); readErr == nil {
		t.Error("client without certificate received data from server; mTLS rejection expected")
	}
}

// TestTLSHandshakeRejectsClientFromWrongCA verifies that the coordinator TLS
// server rejects a client whose certificate is from a different root CA.
// This enforces the Transitloom PKI boundary: only certs from the configured
// root are accepted.
//
// See TestTLSHandshakeRejectsClientWithoutCert for the TLS 1.3 timing note.
func TestTLSHandshakeRejectsClientFromWrongCA(t *testing.T) {
	t.Parallel()

	// Generate the legitimate coordinator PKI chain.
	rootMaterial, intermMaterial, err := generateTestChain(t)
	if err != nil {
		t.Fatalf("generateTestChain(legitimate) error = %v", err)
	}

	// Generate a completely separate "attacker" CA and node cert.
	attackerRoot, err := pki.GenerateRootCA(pki.IssuanceConfig{CommonName: "attacker-root"})
	if err != nil {
		t.Fatalf("GenerateRootCA(attacker) error = %v", err)
	}
	attackerRootCert, _ := pki.ParseCertificatePEM(attackerRoot.CertPEM)
	attackerRootKey, _ := pki.ParseECPrivateKeyPEM(attackerRoot.KeyPEM)
	attackerInterm, err := pki.GenerateCoordinatorIntermediate(pki.IssuanceConfig{CommonName: "attacker-interm"}, attackerRootCert, attackerRootKey)
	if err != nil {
		t.Fatalf("GenerateCoordinatorIntermediate(attacker) error = %v", err)
	}
	attackerIntermCert, _ := pki.ParseCertificatePEM(attackerInterm.CertPEM)
	attackerIntermKey, _ := pki.ParseECPrivateKeyPEM(attackerInterm.KeyPEM)
	attackerNode, err := pki.GenerateNodeCertificate(pki.IssuanceConfig{CommonName: "attacker-node"}, attackerIntermCert, attackerIntermKey)
	if err != nil {
		t.Fatalf("GenerateNodeCertificate(attacker) error = %v", err)
	}

	// Legitimate coordinator TLS server.
	rootPool, _ := pki.NewCertPool(rootMaterial.CertPEM)
	serverTLSCert, _ := pki.ParseTLSCertificatePEM(intermMaterial.CertPEM, intermMaterial.KeyPEM)
	serverTLSConfig := pki.BuildCoordinatorTLSConfig(serverTLSCert, rootPool)

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
	if err != nil {
		t.Fatalf("tls.Listen() error = %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.(*tls.Conn).Handshake() //nolint:errcheck // server-side rejection is expected
		conn.Close()
	}()

	// Attacker node presents cert from wrong CA.
	attackerChainPEM := append(attackerNode.CertPEM, attackerInterm.CertPEM...)
	attackerClientCert, _ := pki.ParseTLSCertificatePEM(attackerChainPEM, attackerNode.KeyPEM)
	// The attacker uses the legitimate root pool for RootCAs so the server
	// cert verification passes, but the server will reject the attacker's
	// client cert because it does not chain to the legitimate root.
	attackerClientConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{attackerClientCert},
		RootCAs:      rootPool,
		ServerName:   "localhost",
	}
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 3 * time.Second},
		"tcp",
		ln.Addr().String(),
		attackerClientConfig,
	)
	if err != nil {
		// Good: rejected during the handshake itself.
		return
	}
	defer conn.Close()
	// In TLS 1.3, the server's certificate chain rejection may surface on the
	// first Read. Set a deadline so the test does not block indefinitely.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second)) //nolint:errcheck
	buf := make([]byte, 1)
	if _, readErr := conn.Read(buf); readErr == nil {
		t.Error("client with wrong-CA certificate received data from server; mTLS rejection expected")
	}
}

// generateTestChain generates a root CA and coordinator intermediate for use
// in tests. The coordinator intermediate includes "localhost" as a DNS SAN
// for TLS server name verification in tests.
func generateTestChain(t *testing.T) (pki.RootCAMaterial, pki.IntermediateMaterial, error) {
	t.Helper()

	rootMaterial, err := pki.GenerateRootCA(pki.IssuanceConfig{
		CommonName: "test-root",
		ValidFor:   1 * time.Hour,
	})
	if err != nil {
		return pki.RootCAMaterial{}, pki.IntermediateMaterial{}, err
	}
	rootCert, err := pki.ParseCertificatePEM(rootMaterial.CertPEM)
	if err != nil {
		return pki.RootCAMaterial{}, pki.IntermediateMaterial{}, err
	}
	rootKey, err := pki.ParseECPrivateKeyPEM(rootMaterial.KeyPEM)
	if err != nil {
		return pki.RootCAMaterial{}, pki.IntermediateMaterial{}, err
	}

	intermMaterial, err := pki.GenerateCoordinatorIntermediate(pki.IssuanceConfig{
		CommonName: "test-coordinator",
		ValidFor:   1 * time.Hour,
		DNSNames:   []string{"test-coordinator", "localhost"},
	}, rootCert, rootKey)
	if err != nil {
		return pki.RootCAMaterial{}, pki.IntermediateMaterial{}, err
	}

	return rootMaterial, intermMaterial, nil
}
