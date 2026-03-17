package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// IssuanceConfig carries the configurable parameters for certificate generation.
//
// It is intentionally narrow: only the fields needed for v1 Transitloom PKI
// bootstrap are present. More sophisticated fields (extended policy, OCSP,
// CRL distribution points) can be added when the full issuance path is
// implemented.
type IssuanceConfig struct {
	// CommonName is the certificate subject common name.
	CommonName string

	// ValidFor is the validity window from the time of generation.
	// If zero, a role-appropriate default is used.
	ValidFor time.Duration

	// DNSNames is the list of DNS subject alternative names.
	// For coordinator TLS certificates, this should include the coordinator's
	// hostname(s) so TLS clients can verify the server name during mTLS
	// handshakes. Empty for node identity certificates.
	DNSNames []string
}

// Default validity windows. These are deliberately long in v1 because hard
// revoke is enforced via admission tokens, not certificate expiry.
//
// From spec/v1-pki-admission.md section 10.3 and 21.1:
//   - longer-lived node certificates are acceptable because current
//     participation permission is enforced separately through admission tokens
//   - admission tokens are short-lived; certificate lifetimes can be longer
const (
	defaultRootCAValidFor       = 10 * 365 * 24 * time.Hour // 10 years
	defaultIntermediateValidFor = 5 * 365 * 24 * time.Hour  // 5 years
	defaultNodeCertValidFor     = 2 * 365 * 24 * time.Hour  // 2 years
)

// RootCAMaterial holds the PEM-encoded root CA certificate and ECDSA private
// key. Both fields are explicit PEM so callers can write them to files without
// further transformation.
type RootCAMaterial struct {
	CertPEM []byte
	KeyPEM  []byte
}

// IntermediateMaterial holds the PEM-encoded coordinator intermediate CA
// certificate and ECDSA private key.
type IntermediateMaterial struct {
	CertPEM []byte
	KeyPEM  []byte
}

// NodeCertMaterial holds the PEM-encoded node identity certificate and ECDSA
// private key.
type NodeCertMaterial struct {
	CertPEM []byte
	KeyPEM  []byte
}

// GenerateRootCA generates a self-signed Transitloom root CA certificate and
// ECDSA P-256 key pair. The root CA is constrained as follows:
//   - IsCA=true with MaxPathLen=1 (allows exactly one level of intermediates)
//   - KeyUsage: CertSign + CRLSign only
//   - No ExtKeyUsage: the root CA should not be used as a transport certificate
//
// From spec/v1-pki-admission.md section 5.1 and 12:
//   - the root CA is the trust anchor for the Transitloom deployment
//   - it should be kept offline after initial setup and coordinator intermediate
//     issuance; it is not a normal node-facing coordinator target
func GenerateRootCA(cfg IssuanceConfig) (RootCAMaterial, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return RootCAMaterial{}, fmt.Errorf("generate root CA key: %w", err)
	}

	validFor := cfg.ValidFor
	if validFor <= 0 {
		validFor = defaultRootCAValidFor
	}

	now := time.Now().UTC()
	serial, err := newSerialNumber()
	if err != nil {
		return RootCAMaterial{}, fmt.Errorf("generate root CA serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: []string{"Transitloom"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(validFor),
		IsCA:                  true,
		MaxPathLen:            1,
		MaxPathLenZero:        false,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return RootCAMaterial{}, fmt.Errorf("create root CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM, err := encodePrivateKeyPEM(key)
	if err != nil {
		return RootCAMaterial{}, fmt.Errorf("encode root CA key: %w", err)
	}

	return RootCAMaterial{CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

// GenerateCoordinatorIntermediate generates a coordinator intermediate CA
// certificate and ECDSA P-256 key pair, signed by the given root CA.
//
// The intermediate is constrained as follows:
//   - IsCA=true with MaxPathLen=0 (prevents further intermediate issuance)
//   - KeyUsage: CertSign + CRLSign
//   - No ExtKeyUsage restriction: CA certs should not restrict the extended key
//     usages of end-entity certs they issue. Restricting to ServerAuth only would
//     prevent Go's x509 chain verification from accepting node certs (ClientAuth)
//     signed by this intermediate.
//   - DNSNames from cfg (for TLS server name verification by connecting nodes)
//
// From spec/v1-pki-admission.md section 5.2:
//   - coordinator intermediates are the normal issuing authorities for node
//     certificates; they must not issue outside deployment permissions
//   - routine node certificate renewal should not require the root to be online
func GenerateCoordinatorIntermediate(cfg IssuanceConfig, rootCert *x509.Certificate, rootKey *ecdsa.PrivateKey) (IntermediateMaterial, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return IntermediateMaterial{}, fmt.Errorf("generate coordinator intermediate key: %w", err)
	}

	validFor := cfg.ValidFor
	if validFor <= 0 {
		validFor = defaultIntermediateValidFor
	}

	now := time.Now().UTC()
	serial, err := newSerialNumber()
	if err != nil {
		return IntermediateMaterial{}, fmt.Errorf("generate coordinator intermediate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: []string{"Transitloom"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(validFor),
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		DNSNames:              cfg.DNSNames,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, rootCert, &key.PublicKey, rootKey)
	if err != nil {
		return IntermediateMaterial{}, fmt.Errorf("create coordinator intermediate certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM, err := encodePrivateKeyPEM(key)
	if err != nil {
		return IntermediateMaterial{}, fmt.Errorf("encode coordinator intermediate key: %w", err)
	}

	return IntermediateMaterial{CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

// GenerateNodeCertificate generates a node identity certificate and ECDSA
// P-256 key pair, signed by the given coordinator intermediate.
//
// The node certificate is a leaf (IsCA=false) with:
//   - ExtKeyUsage: ClientAuth (required for mTLS node-to-coordinator sessions)
//
// A valid node certificate is necessary but NOT sufficient for normal
// participation. A valid admission token is also required.
// From spec/v1-pki-admission.md section 5.3 and 6.3.
func GenerateNodeCertificate(cfg IssuanceConfig, intermediateCert *x509.Certificate, intermediateKey *ecdsa.PrivateKey) (NodeCertMaterial, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return NodeCertMaterial{}, fmt.Errorf("generate node certificate key: %w", err)
	}

	validFor := cfg.ValidFor
	if validFor <= 0 {
		validFor = defaultNodeCertValidFor
	}

	now := time.Now().UTC()
	serial, err := newSerialNumber()
	if err != nil {
		return NodeCertMaterial{}, fmt.Errorf("generate node certificate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: []string{"Transitloom"},
		},
		NotBefore:   now,
		NotAfter:    now.Add(validFor),
		IsCA:        false,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, intermediateCert, &key.PublicKey, intermediateKey)
	if err != nil {
		return NodeCertMaterial{}, fmt.Errorf("create node certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM, err := encodePrivateKeyPEM(key)
	if err != nil {
		return NodeCertMaterial{}, fmt.Errorf("encode node certificate key: %w", err)
	}

	return NodeCertMaterial{CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

// ParseCertificatePEM parses a single PEM-encoded X.509 certificate.
// It returns an error if the PEM is malformed or contains more than one cert block.
func ParseCertificatePEM(certPEM []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in certificate data")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("unexpected PEM block type %q, want CERTIFICATE", block.Type)
	}
	// Require exactly one certificate block. Concatenated cert PEM (for chain
	// building) should use ParseTLSCertificatePEM with a full chain.
	if nextBlock, _ := pem.Decode(rest); nextBlock != nil {
		return nil, fmt.Errorf("expected exactly one PEM certificate block; use ParseTLSCertificatePEM for certificate chains")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	return cert, nil
}

// ParseECPrivateKeyPEM parses a PEM-encoded EC private key.
// It accepts both PKCS#8 (PRIVATE KEY) and SEC1 (EC PRIVATE KEY) PEM formats.
func ParseECPrivateKeyPEM(keyPEM []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in private key data")
	}
	switch block.Type {
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse EC private key (SEC1): %w", err)
		}
		return key, nil
	case "PRIVATE KEY":
		// PKCS#8 wraps the algorithm identifier alongside the key material,
		// making it unambiguous when loading. This is the format produced by
		// encodePrivateKeyPEM (via x509.MarshalPKCS8PrivateKey).
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key (PKCS8): %w", err)
		}
		ecKey, ok := keyInterface.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 private key is not an EC key")
		}
		return ecKey, nil
	default:
		return nil, fmt.Errorf("unexpected PEM block type %q for private key", block.Type)
	}
}

// ParseTLSCertificatePEM parses a PEM-encoded certificate (and optional
// chain certificates) plus a private key into a tls.Certificate suitable
// for use in tls.Config.
//
// certChainPEM may contain multiple PEM-encoded certificate blocks. If it
// contains both a leaf certificate and an intermediate certificate (chain),
// both will be included in the tls.Certificate.Certificate slice, which is
// required for full chain presentation during mTLS handshakes.
func ParseTLSCertificatePEM(certChainPEM, keyPEM []byte) (tls.Certificate, error) {
	tlsCert, err := tls.X509KeyPair(certChainPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse TLS certificate pair: %w", err)
	}
	return tlsCert, nil
}

// NewCertPool creates an x509.CertPool containing the given PEM-encoded CA
// certificate. It is used to build the trust anchor for TLS config (RootCAs
// for clients, ClientCAs for servers in mTLS).
func NewCertPool(caPEM []byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("no valid certificates found in PEM data")
	}
	return pool, nil
}

// newSerialNumber generates a 128-bit random certificate serial number.
// 128 bits of randomness exceeds X.509 recommendations for private PKIs
// with bounded certificate counts.
func newSerialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate serial number: %w", err)
	}
	return serial, nil
}

// encodePrivateKeyPEM encodes an ECDSA private key in PKCS#8 PEM format.
// PKCS#8 includes the algorithm identifier, making the key type unambiguous
// when loading later without parsing the key curve manually.
func encodePrivateKeyPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal EC private key to PKCS8: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), nil
}
