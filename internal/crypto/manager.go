package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// NewKeyManager creates a new key manager
func NewKeyManager() (*KeyManager, error) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Set up global directory
	keyDir := filepath.Join(homeDir, ".dsp-global")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}

	// Create key manager
	km := &KeyManager{
		keyDir:      keyDir,
		certPath:    filepath.Join(keyDir, "dsp-local.crt"),
		certKeyPath: filepath.Join(keyDir, "dsp-local.key"),
	}

	// Load existing config or create new one
	if err := km.loadConfig(); err != nil {
		// If config doesn't exist, create empty one
		km.Config = RecipientsConfig{Recipients: []Recipient{}}
		if err := km.saveConfig(); err != nil {
			return nil, fmt.Errorf("failed to create initial config: %w", err)
		}
	}

	return km, nil
}

// InitializeKeys generates new age keys and a local certificate
func (m *KeyManager) InitializeKeys() error {
	// Generate age keys
	if err := m.generateAgeKeys(); err != nil {
		return fmt.Errorf("failed to generate age keys: %w", err)
	}

	// Generate local certificate if it doesn't exist
	if _, err := os.Stat(m.certPath); os.IsNotExist(err) {
		if err := m.generateLocalCertificate(); err != nil {
			return fmt.Errorf("failed to generate local certificate: %w", err)
		}
	}

	return nil
}

// generateAgeKeys generates new age keys for encryption
func (m *KeyManager) generateAgeKeys() error {
	// Create keys directory if it doesn't exist
	keysDir := filepath.Join(m.keyDir, "keys", "private")
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	// Generate new age key pair
	// TODO: Implement actual age key generation
	// For now, just create placeholder files
	privateKeyPath := filepath.Join(keysDir, "age.key")
	publicKeyPath := filepath.Join(keysDir, "age.key.pub")

	if err := os.WriteFile(privateKeyPath, []byte("placeholder-private-key"), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}
	if err := os.WriteFile(publicKeyPath, []byte("placeholder-public-key"), 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

// generateLocalCertificate generates a self-signed certificate for local LAN use
func (m *KeyManager) generateLocalCertificate() error {
	// Get hostname for certificate
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"DSP Local Network"},
			CommonName:   hostname,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0), // 10 year validity
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
		DNSNames: []string{
			"localhost",
			hostname,
			"*.local", // Allow .local domain
		},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Save certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	if err := os.WriteFile(m.certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	// Save private key
	keyPEM, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEMBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyPEM,
	})
	if err := os.WriteFile(m.certKeyPath, keyPEMBlock, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	return nil
}

// GetCertificate returns the local certificate and private key
func (m *KeyManager) GetCertificate() (tls.Certificate, error) {
	return tls.LoadX509KeyPair(m.certPath, m.certKeyPath)
}

// GetCertificateFingerprint returns the SHA-256 fingerprint of the local certificate
func (m *KeyManager) GetCertificateFingerprint() (string, error) {
	certPEM, err := os.ReadFile(m.certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Calculate SHA-256 fingerprint
	fingerprint := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(fingerprint[:]), nil
}

// VerifyCertificate verifies a certificate against the local certificate
func (m *KeyManager) VerifyCertificate(cert *x509.Certificate) error {
	// Read local certificate
	localCertPEM, err := os.ReadFile(m.certPath)
	if err != nil {
		return fmt.Errorf("failed to read local certificate: %w", err)
	}

	block, _ := pem.Decode(localCertPEM)
	if block == nil {
		return fmt.Errorf("failed to decode local certificate PEM")
	}

	localCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse local certificate: %w", err)
	}

	// Verify certificate
	if !bytes.Equal(cert.Raw, localCert.Raw) {
		return fmt.Errorf("certificate does not match local certificate")
	}

	return nil
}

// loadConfig loads the recipients configuration
func (m *KeyManager) loadConfig() error {
	configPath := filepath.Join(m.keyDir, "keys", "recipients.yaml")

	// If config doesn't exist, create empty one
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		m.Config = RecipientsConfig{Recipients: []Recipient{}}
		return m.saveConfig()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &m.Config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	return nil
}

// saveConfig saves the recipients configuration
func (m *KeyManager) saveConfig() error {
	data, err := yaml.Marshal(m.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(m.keyDir, "keys", "recipients.yaml")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// AddRecipient adds a new recipient
func (m *KeyManager) AddRecipient(name, publicKey string) error {
	// Generate a unique key ID
	keyID := fmt.Sprintf("%s-%d", name, time.Now().Unix())

	// Save the public key file
	keyPath := filepath.Join(m.keyDir, "keys", "public", "recipients", keyID+".pub")
	if err := os.WriteFile(keyPath, []byte(publicKey), 0644); err != nil {
		return fmt.Errorf("failed to save public key: %w", err)
	}

	// Add to config
	recipient := Recipient{
		Name:    name,
		KeyID:   keyID,
		Key:     publicKey,
		Added:   time.Now(),
		Trusted: true, // Default to trusted
	}
	m.Config.Recipients = append(m.Config.Recipients, recipient)

	return m.saveConfig()
}

// GetRecipient gets a recipient by name
func (m *KeyManager) GetRecipient(name string) (*Recipient, error) {
	for _, r := range m.Config.Recipients {
		if r.Name == name {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("recipient not found: %s", name)
}

// ListRecipients lists all known recipients
func (m *KeyManager) ListRecipients() []Recipient {
	return m.Config.Recipients
}

// RemoveRecipient removes a recipient
func (m *KeyManager) RemoveRecipient(name string) error {
	// Find and remove from config
	for i, r := range m.Config.Recipients {
		if r.Name == name {
			// Remove the key file
			keyPath := filepath.Join(m.keyDir, "keys", "public", "recipients", r.KeyID+".pub")
			if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove key file: %w", err)
			}

			// Remove from config
			m.Config.Recipients = append(m.Config.Recipients[:i], m.Config.Recipients[i+1:]...)
			return m.saveConfig()
		}
	}
	return fmt.Errorf("recipient not found: %s", name)
}

// GetPrivateKeyPath returns the path to the private key
func (m *KeyManager) GetPrivateKeyPath() string {
	return filepath.Join(m.keyDir, "keys", "private", "age.key")
}

// GetPublicKeyPath returns the path to the public key
func (m *KeyManager) GetPublicKeyPath() string {
	return filepath.Join(m.keyDir, "keys", "private", "age.key.pub")
}
