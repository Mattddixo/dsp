package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"filippo.io/age"
)

// GenerateKeyPair generates a new age key pair
func (m *KeyManager) GenerateKeyPair() error {
	privateKeyPath := m.GetPrivateKeyPath()
	publicKeyPath := m.GetPublicKeyPath()

	// Check if keys already exist
	if _, err := os.Stat(privateKeyPath); err == nil {
		return fmt.Errorf("private key already exists at %s", privateKeyPath)
	}

	// Generate new key pair
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write private key
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privateKeyFile.Close()

	// Write the private key in age format
	if _, err := fmt.Fprintf(privateKeyFile, "# created: %s\n# public key: %s\n%s\n",
		time.Now().Format(time.RFC3339),
		identity.Recipient().String(),
		identity.String()); err != nil {
		os.Remove(privateKeyPath) // Clean up on error
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key
	publicKeyFile, err := os.OpenFile(publicKeyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		os.Remove(privateKeyPath) // Clean up on error
		return fmt.Errorf("failed to create public key file: %w", err)
	}
	defer publicKeyFile.Close()

	if _, err := fmt.Fprintf(publicKeyFile, "# public key: %s\n%s\n", identity.Recipient().String(), identity.Recipient().String()); err != nil {
		os.Remove(privateKeyPath) // Clean up on error
		os.Remove(publicKeyPath)  // Clean up on error
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

// GetPublicKey returns the public key
func (m *KeyManager) GetPublicKey() (string, error) {
	publicKeyPath := m.GetPublicKeyPath()
	data, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key: %w", err)
	}
	return string(data), nil
}

// EncryptWithPublicKey encrypts data for a recipient
func (m *KeyManager) EncryptWithPublicKey(recipientName string, data []byte) ([]byte, error) {
	recipient, err := m.GetRecipient(recipientName)
	if err != nil {
		return nil, err
	}

	// Parse the recipient's public key
	r, err := age.ParseX25519Recipient(recipient.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse recipient key: %w", err)
	}

	// Create an encrypted writer
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, r)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypted writer: %w", err)
	}

	// Write the data
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	// Close the writer to finalize encryption
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize encryption: %w", err)
	}

	return buf.Bytes(), nil
}

// DecryptWithPrivateKey decrypts data using the private key
func (m *KeyManager) DecryptWithPrivateKey(data []byte) ([]byte, error) {
	privateKeyPath := m.GetPrivateKeyPath()
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("private key not found at %s", privateKeyPath)
	}

	// Read and parse the private key
	identityFile, err := os.Open(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open private key: %w", err)
	}
	defer identityFile.Close()

	identity, err := age.ParseIdentities(identityFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create a reader for the encrypted data
	r, err := age.Decrypt(bytes.NewReader(data), identity...)
	if err != nil {
		return nil, fmt.Errorf("failed to create decrypted reader: %w", err)
	}

	// Read the decrypted data
	decrypted, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}

	return decrypted, nil
}

// EncryptWithPassphrase encrypts data with a passphrase
func (m *KeyManager) EncryptWithPassphrase(passphrase string, data []byte) ([]byte, error) {
	// Create a passphrase recipient
	r, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create passphrase recipient: %w", err)
	}

	// Create an encrypted writer
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, r)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypted writer: %w", err)
	}

	// Write the data
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	// Close the writer to finalize encryption
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize encryption: %w", err)
	}

	return buf.Bytes(), nil
}

// DecryptWithPassphrase decrypts data with a passphrase
func (m *KeyManager) DecryptWithPassphrase(passphrase string, data []byte) ([]byte, error) {
	// Create a passphrase identity
	i, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create passphrase identity: %w", err)
	}

	// Create a reader for the encrypted data
	r, err := age.Decrypt(bytes.NewReader(data), i)
	if err != nil {
		return nil, fmt.Errorf("failed to create decrypted reader: %w", err)
	}

	// Read the decrypted data
	decrypted, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}

	return decrypted, nil
}

// EncryptWithMultipleRecipients encrypts data for multiple recipients
func (m *KeyManager) EncryptWithMultipleRecipients(recipientNames []string, data []byte) ([]byte, error) {
	if len(recipientNames) == 0 {
		return nil, fmt.Errorf("no recipients specified")
	}

	// Parse all recipient keys
	var recipients []age.Recipient
	for _, name := range recipientNames {
		recipient, err := m.GetRecipient(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get recipient %s: %w", name, err)
		}

		// Parse the recipient's public key
		r, err := age.ParseX25519Recipient(recipient.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse recipient key for %s: %w", name, err)
		}
		recipients = append(recipients, r)
	}

	// Create an encrypted writer with all recipients
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipients...)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypted writer: %w", err)
	}

	// Write the data
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	// Close the writer to finalize encryption
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize encryption: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateSigningKeyPair generates a new ed25519 key pair for signing
func (m *KeyManager) GenerateSigningKeyPair() error {
	// Generate new key pair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(m.GetSigningKeyPath()), 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Convert private key to PKCS8 format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Create PEM block for private key
	privateKeyPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	// Write private key
	privateKeyPath := m.GetSigningKeyPath()
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privateKeyFile.Close()

	// Write the private key in PEM format
	if _, err := fmt.Fprintf(privateKeyFile, "# created: %s\n%s\n",
		time.Now().Format(time.RFC3339),
		pem.EncodeToMemory(privateKeyPEM)); err != nil {
		os.Remove(privateKeyPath) // Clean up on error
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Convert public key to PKIX format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		os.Remove(privateKeyPath) // Clean up on error
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	// Create PEM block for public key
	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	// Write public key
	publicKeyPath := m.GetSigningPublicKeyPath()
	publicKeyFile, err := os.OpenFile(publicKeyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		os.Remove(privateKeyPath) // Clean up on error
		return fmt.Errorf("failed to create public key file: %w", err)
	}
	defer publicKeyFile.Close()

	if _, err := fmt.Fprintf(publicKeyFile, "# created: %s\n%s\n",
		time.Now().Format(time.RFC3339),
		pem.EncodeToMemory(publicKeyPEM)); err != nil {
		os.Remove(privateKeyPath) // Clean up on error
		os.Remove(publicKeyPath)  // Clean up on error
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

// SignExportInfo signs the export information using the private key
func (m *KeyManager) SignExportInfo(info interface{}) (string, error) {
	// Read signing private key
	privateKeyPath := m.GetSigningKeyPath()
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read signing key: %w", err)
	}

	// Parse PEM block
	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	// Parse private key
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse signing key: %w", err)
	}

	// Convert to ed25519.PrivateKey
	ed25519Key, ok := privateKey.(ed25519.PrivateKey)
	if !ok {
		return "", fmt.Errorf("signing key is not an ed25519 key")
	}

	// Marshal the info to JSON
	infoJSON, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("failed to marshal info: %w", err)
	}

	// Create a signature using ed25519
	signature := ed25519.Sign(ed25519Key, infoJSON)

	// Return the signature as a base64 string
	return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifyExportInfo verifies the signature of export information
func (m *KeyManager) VerifyExportInfo(info interface{}, signature string) error {
	// Get the signing public key
	publicKeyPath := m.GetSigningPublicKeyPath()
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read signing public key: %w", err)
	}

	// Parse PEM block
	block, _ := pem.Decode(publicKeyData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	// Parse public key
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse signing public key: %w", err)
	}

	// Convert to ed25519.PublicKey
	ed25519Key, ok := pub.(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf("signing public key is not an ed25519 key")
	}

	// Marshal the info to JSON
	infoJSON, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal info: %w", err)
	}

	// Decode the signature
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	// Verify the signature using ed25519
	if !ed25519.Verify(ed25519Key, infoJSON, sig) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// EncryptWithPassphrase encrypts data using a passphrase
func EncryptWithPassphrase(data []byte, passphrase string) ([]byte, error) {
	// Create a new age recipient from the passphrase
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipient: %w", err)
	}

	// Create a buffer to hold the encrypted data
	var buf bytes.Buffer

	// Create an age writer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypt writer: %w", err)
	}

	// Write the data
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	// Close the writer to finalize encryption
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize encryption: %w", err)
	}

	return buf.Bytes(), nil
}

// DecryptWithPassphrase decrypts data using a passphrase
func DecryptWithPassphrase(data []byte, passphrase string) ([]byte, error) {
	// Create a new age identity from the passphrase
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	// Create a reader from the encrypted data
	r := bytes.NewReader(data)

	// Create an age reader
	decrypted, err := age.Decrypt(r, identity)
	if err != nil {
		return nil, fmt.Errorf("failed to create decrypt reader: %w", err)
	}

	// Read the decrypted data
	decryptedData, err := io.ReadAll(decrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}

	return decryptedData, nil
}
