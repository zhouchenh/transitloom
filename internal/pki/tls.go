package pki

import (
	"crypto/tls"
	"crypto/x509"
)

// BuildCoordinatorTLSConfig builds a *tls.Config for the coordinator TCP+TLS
// control listener. This is the intended fallback control transport per
// spec/v1-control-plane.md section 6.2.
//
// The config enforces:
//   - TLS 1.3 minimum (TLS 1.2 and earlier are explicitly rejected)
//   - Mutual TLS: the coordinator requires and verifies client certificates
//   - Client certificate trust: verified against the provided root CA pool
//
// serverCert should be the coordinator intermediate certificate (or a
// dedicated TLS leaf certificate signed by the intermediate). Its DNSNames
// should include the coordinator's hostname for client-side server name
// verification.
//
// rootCertPool should be built from the Transitloom root CA certificate
// using NewCertPool. Both coordinator and node certificates chain to the
// same root, so a single pool is used for both client and server verification.
//
// QUIC+TLS 1.3 mTLS (the primary transport, spec section 6.1) will use the
// same certificate material with a different transport wrapper when QUIC is
// implemented.
func BuildCoordinatorTLSConfig(serverCert tls.Certificate, rootCertPool *x509.CertPool) *tls.Config {
	return &tls.Config{
		// TLS 1.3 minimum is non-negotiable for the intended control transport.
		// This is explicit rather than left to cipher suite negotiation.
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{serverCert},
		// mTLS: the coordinator must verify that the connecting node presents
		// a certificate chain valid under the Transitloom root CA. This is
		// how node identity is verified at the transport layer.
		// A valid node certificate is necessary but not sufficient for
		// participation; admission-token enforcement is an application-layer
		// concern above this transport.
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  rootCertPool,
	}
}

// BuildNodeTLSConfig builds a *tls.Config for a node TCP+TLS 1.3 mTLS
// control session client. This is the node side of the mutual TLS handshake.
//
// The config enforces:
//   - TLS 1.3 minimum
//   - Node certificate presented to the coordinator for mTLS authentication
//   - Coordinator certificate verified against the provided root CA pool
//   - ServerName used for coordinator TLS certificate verification
//
// clientCert should be built from the concatenated node certificate and
// coordinator intermediate certificate PEM using ParseTLSCertificatePEM,
// so the full chain is presented during the mTLS handshake.
//
// serverName should match a DNS SAN in the coordinator's TLS certificate.
// When empty, TLS server name verification falls back to IP SANs, which is
// insufficient for hostname-based deployments.
func BuildNodeTLSConfig(clientCert tls.Certificate, rootCertPool *x509.CertPool, serverName string) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{clientCert},
		// Verify the coordinator's certificate against the Transitloom root CA.
		// This prevents connecting to a coordinator without a valid certificate
		// chain to the trusted root, even if the TLS handshake would otherwise
		// succeed (e.g., self-signed impostor coordinators).
		RootCAs:    rootCertPool,
		ServerName: serverName,
	}
}
