package crypto

import "time"

// Recipient represents a person who can receive encrypted bundles
type Recipient struct {
	Name    string    `yaml:"name"`
	KeyID   string    `yaml:"key_id"`
	Key     string    `yaml:"key"` // The actual public key
	Added   time.Time `yaml:"added"`
	Notes   string    `yaml:"notes,omitempty"`
	Trusted bool      `yaml:"trusted"`
}

// RecipientsConfig holds the configuration for known recipients
type RecipientsConfig struct {
	Recipients []Recipient `yaml:"recipients"`
}

// KeyManager manages cryptographic keys and certificates
type KeyManager struct {
	keyDir      string
	privateKey  string
	publicKey   string
	certPath    string           // Path to the local certificate
	certKeyPath string           // Path to the certificate private key
	Config      RecipientsConfig // Configuration for recipients
}

// EncryptionMethod specifies how a bundle is encrypted
type EncryptionMethod string

const (
	// AgePublicKey uses age public key encryption
	AgePublicKey EncryptionMethod = "age-public"
	// AgePassphrase uses age passphrase encryption
	AgePassphrase EncryptionMethod = "age-pass"
	// None means no encryption
	None EncryptionMethod = "none"
)

// BundleEncryptionInfo holds information about how a bundle is encrypted
type BundleEncryptionInfo struct {
	Method     EncryptionMethod `json:"method"`
	Recipients []string         `json:"recipients,omitempty"` // For public key encryption
	Passphrase string           `json:"passphrase,omitempty"` // For passphrase encryption
	CreatedAt  time.Time        `json:"created_at"`
}
