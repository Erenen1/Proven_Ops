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
	"net"
	"os"
	"path/filepath"
	"time"
)

// CertificateAuthority holds CA cert and key
type CertificateAuthority struct {
	Cert    *x509.Certificate
	CertPEM []byte
	Key     *ecdsa.PrivateKey
	KeyPEM  []byte
}

// KeyPair holds a generated cert and private key in PEM format
type KeyPair struct {
	Cert    *x509.Certificate
	CertPEM []byte
	Key     *ecdsa.PrivateKey
	KeyPEM  []byte
}

// GenerateCA creates a self-signed root CA for OpsPilot
func GenerateCA(commonName string) (*CertificateAuthority, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"OpsPilot Infrastructure Security"},
			CommonName:   commonName,
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CA key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	parsedCert, _ := x509.ParseCertificate(derBytes)

	return &CertificateAuthority{
		Cert:    parsedCert,
		CertPEM: certPEM,
		Key:     priv,
		KeyPEM:  keyPEM,
	}, nil
}

// GenerateServerCert creates a server certificate signed by the provided CA
func GenerateServerCert(ca *CertificateAuthority, hosts []string) (*KeyPair, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"OpsPilot Control Plane"},
			CommonName:   "opspilot-control-plane",
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}
	// Always include localhost and 127.0.0.1
	template.DNSNames = append(template.DNSNames, "localhost")
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"), net.IPv6loopback)

	derBytes, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, &priv.PublicKey, ca.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create server certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, _ := x509.MarshalECPrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	parsedCert, _ := x509.ParseCertificate(derBytes)

	return &KeyPair{
		Cert:    parsedCert,
		CertPEM: certPEM,
		Key:     priv,
		KeyPEM:  keyPEM,
	}, nil
}

// GenerateClientCert creates a client certificate for an Agent signed by the provided CA
func GenerateClientCert(ca *CertificateAuthority, agentID, hostname string) (*KeyPair, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"OpsPilot Fleet Agent"},
			CommonName:   agentID,
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	if hostname != "" {
		template.DNSNames = append(template.DNSNames, hostname)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, &priv.PublicKey, ca.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create client certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, _ := x509.MarshalECPrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	parsedCert, _ := x509.ParseCertificate(derBytes)

	return &KeyPair{
		Cert:    parsedCert,
		CertPEM: certPEM,
		Key:     priv,
		KeyPEM:  keyPEM,
	}, nil
}

// SavePEM saves byte slices to disk with secure permissions
func SavePEM(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// ServerTLSConfig creates a TLS config enforcing mutual TLS
func ServerTLSConfig(caPEM, serverCertPEM, serverKeyPEM []byte) (*tls.Config, error) {
	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load server key pair: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate PEM")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// ClientTLSConfig creates a TLS config for the Agent to authenticate with the server
func ClientTLSConfig(caPEM, clientCertPEM, clientKeyPEM []byte, serverName string) (*tls.Config, error) {
	clientCert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load client key pair: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate PEM")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// LoadCAFromPEM loads a CertificateAuthority from cert and key PEM bytes
func LoadCAFromPEM(certPEM, keyPEM []byte) (*CertificateAuthority, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("failed to decode CA certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode CA private key PEM")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		k, err2 := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse EC private key: %w", err)
		}
		var ok bool
		key, ok = k.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("CA private key is not ECDSA")
		}
	}

	return &CertificateAuthority{
		Cert:    cert,
		CertPEM: certPEM,
		Key:     key,
		KeyPEM:  keyPEM,
	}, nil
}

// IssueAgentCertificate creates a signed client certificate for an agent with specified validity duration
func IssueAgentCertificate(ca *CertificateAuthority, agentID, hostname string, duration time.Duration) (*KeyPair, error) {
	if duration <= 0 {
		duration = 90 * 24 * time.Hour
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"OpsPilot Fleet Agent"},
			CommonName:   agentID,
		},
		NotBefore:   time.Now().Add(-5 * time.Minute),
		NotAfter:    time.Now().Add(duration),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	if hostname != "" {
		template.DNSNames = append(template.DNSNames, hostname)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, &priv.PublicKey, ca.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create client certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, _ := x509.MarshalECPrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	parsedCert, _ := x509.ParseCertificate(derBytes)

	return &KeyPair{
		Cert:    parsedCert,
		CertPEM: certPEM,
		Key:     priv,
		KeyPEM:  keyPEM,
	}, nil
}

// ValidateCertificate verifies that the certificate was signed by the CA and is currently valid
func ValidateCertificate(certPEM, caPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate PEM")
	}

	opts := x509.VerifyOptions{
		Roots:     caPool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageAny},
	}
	if _, err := cert.Verify(opts); err != nil {
		return nil, fmt.Errorf("certificate verification failed: %w", err)
	}

	return cert, nil
}

// RenewAgentCertificate validates the existing certificate and issues a new certificate for the same agent
func RenewAgentCertificate(ca *CertificateAuthority, existingCertPEM []byte, duration time.Duration) (*KeyPair, error) {
	cert, err := ValidateCertificate(existingCertPEM, ca.CertPEM)
	if err != nil {
		return nil, fmt.Errorf("cannot renew invalid certificate: %w", err)
	}

	hostname := ""
	if len(cert.DNSNames) > 0 {
		hostname = cert.DNSNames[0]
	}

	return IssueAgentCertificate(ca, cert.Subject.CommonName, hostname, duration)
}

