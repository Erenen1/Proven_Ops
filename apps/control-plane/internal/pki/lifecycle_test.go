package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestCertificateLifecycle_IssueValidateRenew(t *testing.T) {
	// 1. Generate Root CA
	ca, err := GenerateCA("OpsPilot Test Root CA")
	if err != nil {
		t.Fatalf("GenerateCA failed: %v", err)
	}

	// 2. Load CA from PEM
	loadedCA, err := LoadCAFromPEM(ca.CertPEM, ca.KeyPEM)
	if err != nil {
		t.Fatalf("LoadCAFromPEM failed: %v", err)
	}
	if loadedCA.Cert.Subject.CommonName != "OpsPilot Test Root CA" {
		t.Errorf("expected CA CN %s, got %s", "OpsPilot Test Root CA", loadedCA.Cert.Subject.CommonName)
	}

	// 3. Issue Client Certificate for Agent
	agentID := "agent-worker-01"
	hostname := "worker-01.opspilot.internal"
	validDuration := 30 * 24 * time.Hour

	keyPair, err := IssueAgentCertificate(loadedCA, agentID, hostname, validDuration)
	if err != nil {
		t.Fatalf("IssueAgentCertificate failed: %v", err)
	}

	if keyPair.Cert.Subject.CommonName != agentID {
		t.Errorf("expected agent CommonName %s, got %s", agentID, keyPair.Cert.Subject.CommonName)
	}
	if len(keyPair.Cert.DNSNames) == 0 || keyPair.Cert.DNSNames[0] != hostname {
		t.Errorf("expected DNSName %s, got %v", hostname, keyPair.Cert.DNSNames)
	}

	// 4. Validate Certificate
	validatedCert, err := ValidateCertificate(keyPair.CertPEM, ca.CertPEM)
	if err != nil {
		t.Fatalf("ValidateCertificate failed: %v", err)
	}
	if validatedCert.Subject.CommonName != agentID {
		t.Errorf("validated cert common name mismatch: %s", validatedCert.Subject.CommonName)
	}

	// 5. Test Forged Certificate Rejection
	roguePriv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rogueTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(9999),
		Subject:      pkix.Name{CommonName: agentID},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
	}
	rogueDer, _ := x509.CreateCertificate(rand.Reader, rogueTmpl, rogueTmpl, &roguePriv.PublicKey, roguePriv)
	roguePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rogueDer})

	_, rogueErr := ValidateCertificate(roguePEM, ca.CertPEM)
	if rogueErr == nil {
		t.Errorf("expected rogue certificate validation to fail, but it passed")
	}

	// 6. Renew Certificate
	renewedPair, err := RenewAgentCertificate(loadedCA, keyPair.CertPEM, 60*24*time.Hour)
	if err != nil {
		t.Fatalf("RenewAgentCertificate failed: %v", err)
	}

	if renewedPair.Cert.Subject.CommonName != agentID {
		t.Errorf("renewed cert common name mismatch: %s", renewedPair.Cert.Subject.CommonName)
	}
	if len(renewedPair.Cert.DNSNames) == 0 || renewedPair.Cert.DNSNames[0] != hostname {
		t.Errorf("renewed cert hostname mismatch: %v", renewedPair.Cert.DNSNames)
	}

	// Ensure renewed cert validates against CA
	if _, err := ValidateCertificate(renewedPair.CertPEM, ca.CertPEM); err != nil {
		t.Fatalf("renewed cert failed validation: %v", err)
	}
}
