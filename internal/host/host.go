package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Host represents a known host in the system
type Host struct {
	// Basic Info
	Name      string    `json:"name"`       // User-friendly name (e.g., "Alice's Laptop")
	PublicKey string    `json:"public_key"` // Their age public key
	AddedAt   time.Time `json:"added_at"`   // When we first connected
	LastUsed  time.Time `json:"last_used"`  // Last successful transfer
	Trusted   bool      `json:"trusted"`    // Whether we trust this host

	// Additional Info
	Description string   `json:"description,omitempty"` // Optional description
	IPAddress   string   `json:"ip_address,omitempty"`  // Last known IP
	LastPort    int      `json:"last_port,omitempty"`   // Last used port
	Alias       string   `json:"alias,omitempty"`       // Short alias for quick reference
	Tags        []string `json:"tags,omitempty"`        // User-defined tags

	// Certificate Info (new fields, all optional for backward compatibility)
	CertInfo *CertificateInfo `json:"cert_info,omitempty"` // Certificate information
}

// CertificateInfo holds information about a host's certificate
type CertificateInfo struct {
	Fingerprint  string    `json:"fingerprint"`   // SHA-256 fingerprint
	ValidFrom    time.Time `json:"valid_from"`    // Certificate validity start
	ValidTo      time.Time `json:"valid_to"`      // Certificate validity end
	LastVerified time.Time `json:"last_verified"` // When we last verified this cert
}

// UpdateCertificate updates the certificate information for a host
func (h *Host) UpdateCertificate(fingerprint string, validFrom, validTo time.Time) {
	if h.CertInfo == nil {
		h.CertInfo = &CertificateInfo{}
	}
	h.CertInfo.Fingerprint = fingerprint
	h.CertInfo.ValidFrom = validFrom
	h.CertInfo.ValidTo = validTo
	h.CertInfo.LastVerified = time.Now()
}

// VerifyCertificate verifies if a certificate is valid for this host
func (h *Host) VerifyCertificate(fingerprint string, validFrom, validTo time.Time) error {
	// If we have no stored certificate, this is the first time
	if h.CertInfo == nil {
		return nil
	}

	// Verify the fingerprint matches
	if h.CertInfo.Fingerprint != fingerprint {
		return fmt.Errorf("certificate fingerprint mismatch for host %s", h.Name)
	}

	// Verify the certificate hasn't expired
	if time.Now().After(h.CertInfo.ValidTo) {
		return fmt.Errorf("stored certificate for host %s has expired", h.Name)
	}

	// Verify the new certificate is not older than the stored one
	// This prevents certificate rollback attacks
	if validTo.Before(h.CertInfo.ValidTo) {
		return fmt.Errorf("new certificate for host %s expires before stored certificate", h.Name)
	}

	return nil
}

// Manager handles host management operations
type Manager struct {
	configDir string
	hosts     map[string]*Host // Map of host name to host
}

// NewManager creates a new host manager
func NewManager() (*Manager, error) {
	// Get user's home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Use default global directory
	globalDir := filepath.Join(home, ".dsp-global")

	// Create hosts directory if it doesn't exist
	hostsDir := filepath.Join(globalDir, "hosts")
	if err := os.MkdirAll(hostsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create hosts directory: %w", err)
	}

	manager := &Manager{
		configDir: hostsDir,
		hosts:     make(map[string]*Host),
	}

	// Load existing hosts
	if err := manager.loadHosts(); err != nil {
		return nil, fmt.Errorf("failed to load hosts: %w", err)
	}

	return manager, nil
}

// loadHosts loads all hosts from the hosts directory
func (m *Manager) loadHosts() error {
	// Read hosts directory
	entries, err := os.ReadDir(m.configDir)
	if err != nil {
		return fmt.Errorf("failed to read hosts directory: %w", err)
	}

	// Load each host file
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		hostPath := filepath.Join(m.configDir, entry.Name())
		data, err := os.ReadFile(hostPath)
		if err != nil {
			return fmt.Errorf("failed to read host file %s: %w", entry.Name(), err)
		}

		var host Host
		if err := json.Unmarshal(data, &host); err != nil {
			return fmt.Errorf("failed to parse host file %s: %w", entry.Name(), err)
		}

		m.hosts[host.Name] = &host
	}

	return nil
}

// saveHost saves a host to disk
func (m *Manager) saveHost(host *Host) error {
	// Marshal host to JSON
	data, err := json.MarshalIndent(host, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal host: %w", err)
	}

	// Create host file path
	hostPath := filepath.Join(m.configDir, host.Name+".json")

	// Write host file
	if err := os.WriteFile(hostPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write host file: %w", err)
	}

	return nil
}

// AddHost adds a new host
func (m *Manager) AddHost(host *Host) error {
	if _, exists := m.hosts[host.Name]; exists {
		return fmt.Errorf("host with name %s already exists", host.Name)
	}

	host.AddedAt = time.Now()
	host.LastUsed = time.Now()

	if err := m.saveHost(host); err != nil {
		return err
	}

	m.hosts[host.Name] = host
	return nil
}

// UpdateHost updates an existing host
func (m *Manager) UpdateHost(host *Host) error {
	if _, exists := m.hosts[host.Name]; !exists {
		return fmt.Errorf("host %s does not exist", host.Name)
	}

	host.LastUsed = time.Now()

	if err := m.saveHost(host); err != nil {
		return err
	}

	m.hosts[host.Name] = host
	return nil
}

// RemoveHost removes a host
func (m *Manager) RemoveHost(name string) error {
	if _, exists := m.hosts[name]; !exists {
		return fmt.Errorf("host %s does not exist", name)
	}

	hostPath := filepath.Join(m.configDir, name+".json")
	if err := os.Remove(hostPath); err != nil {
		return fmt.Errorf("failed to remove host file: %w", err)
	}

	delete(m.hosts, name)
	return nil
}

// GetHost retrieves a host by name
func (m *Manager) GetHost(name string) (*Host, error) {
	host, exists := m.hosts[name]
	if !exists {
		return nil, fmt.Errorf("host %s does not exist", name)
	}
	return host, nil
}

// GetHostByAlias retrieves a host by alias
func (m *Manager) GetHostByAlias(alias string) (*Host, error) {
	for _, host := range m.hosts {
		if host.Alias == alias {
			return host, nil
		}
	}
	return nil, fmt.Errorf("no host found with alias %s", alias)
}

// GetHostByTag retrieves hosts by tag
func (m *Manager) GetHostByTag(tag string) []*Host {
	var hosts []*Host
	for _, host := range m.hosts {
		for _, t := range host.Tags {
			if t == tag {
				hosts = append(hosts, host)
				break
			}
		}
	}
	return hosts
}

// ListHosts returns all hosts
func (m *Manager) ListHosts() []*Host {
	hosts := make([]*Host, 0, len(m.hosts))
	for _, host := range m.hosts {
		hosts = append(hosts, host)
	}
	return hosts
}

// UpdateLastUsed updates the LastUsed timestamp for a host
func (m *Manager) UpdateLastUsed(name string) error {
	host, err := m.GetHost(name)
	if err != nil {
		return err
	}

	host.LastUsed = time.Now()
	return m.UpdateHost(host)
}
