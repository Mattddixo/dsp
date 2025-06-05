package importcmd

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Mattddixo/dsp/config"
	"github.com/Mattddixo/dsp/internal/bundle"
	"github.com/Mattddixo/dsp/internal/crypto"
	hostpkg "github.com/Mattddixo/dsp/internal/host"
	"github.com/Mattddixo/dsp/internal/repo"
	"github.com/Mattddixo/dsp/internal/snapshot"
	"github.com/urfave/cli/v2"
)

// ExportInfo contains information needed for import
type ExportInfo struct {
	Host            string   `json:"host"`
	Port            int      `json:"port"`
	BundleID        string   `json:"bundle_id"`
	Auth            string   `json:"auth_method"`
	Users           []string `json:"users,omitempty"`
	Password        string   `json:"password,omitempty"`
	Signature       string   `json:"signature"`
	Expires         string   `json:"expires"`
	Encrypted       bool     `json:"encrypted"`
	Token           string   `json:"token,omitempty"`        // New field for assigned token
	TokenExpiry     string   `json:"token_expiry,omitempty"` // New field for token expiry
	CertFingerprint string   `json:"cert_fingerprint"`
}

var Command = &cli.Command{
	Name:  "import",
	Usage: "Import a bundle from a remote server",
	Description: `Import a bundle from a remote server and apply its changes.
This command downloads a bundle from a server and creates a new repository
with the bundle's contents. The DSP directory name will be maintained from
the source repository.

Examples:
  # Import with password authentication
  dsp import -h localhost -p "secret123" --repo my-repo --root /path/to/repo

  # Import with default repository setting
  dsp import -h localhost -p "secret123" --repo my-repo --root /path/to/repo --default`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "host",
			Aliases:  []string{"H"},
			Usage:    "Host address of the export server",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "password",
			Aliases:  []string{"p"},
			Usage:    "Password for authentication",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "repo",
			Aliases:  []string{"r"},
			Usage:    "Name for the new repository",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "root",
			Aliases:  []string{"R"},
			Usage:    "Root path for the new repository",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "default",
			Aliases: []string{"D"},
			Usage:   "Set as default repository",
		},
	},
	Action: func(c *cli.Context) error {
		// Get command arguments
		host := c.String("host")
		password := c.String("password")
		repoName := c.String("repo")
		repoRoot := c.String("root")
		setDefault := c.Bool("default")

		// Convert repository root to absolute path
		absRepoRoot, err := filepath.Abs(repoRoot)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Check if repository already exists
		if repo.IsRepository(absRepoRoot) {
			return fmt.Errorf("repository already exists at %s", absRepoRoot)
		}

		// Download bundle from server first to get DSP directory name
		fmt.Printf("Downloading bundle from %s...\n", host)
		tempDir, err := os.MkdirTemp("", "dsp-import-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		bundlePath, err := downloadBundle(host, password, tempDir)
		if err != nil {
			return fmt.Errorf("failed to download bundle: %w", err)
		}

		// Load bundle to get DSP directory name
		b, err := bundle.Load(bundlePath)
		if err != nil {
			return fmt.Errorf("failed to load bundle: %w", err)
		}

		// Create repository manager
		manager, err := repo.NewManager()
		if err != nil {
			return fmt.Errorf("failed to create repository manager: %w", err)
		}

		// Create new repository using DSP directory name from bundle
		if err := manager.InitializeRepository(absRepoRoot, repoName, setDefault, b.Repository.DSPDir); err != nil {
			return fmt.Errorf("failed to initialize repository: %w", err)
		}

		// Get repository context
		currentRepo, err := manager.GetRepository(absRepoRoot)
		if err != nil {
			return fmt.Errorf("failed to get repository: %w", err)
		}

		// Get DSP directory path
		dspDirPath := currentRepo.GetDSPDir()

		// Move bundle to final location
		finalBundlePath := filepath.Join(dspDirPath, "bundles", filepath.Base(bundlePath))
		if err := os.MkdirAll(filepath.Dir(finalBundlePath), 0755); err != nil {
			return fmt.Errorf("failed to create bundles directory: %w", err)
		}
		if err := os.Rename(bundlePath, finalBundlePath); err != nil {
			return fmt.Errorf("failed to move bundle to final location: %w", err)
		}

		// Update repository configuration
		if err := updateRepositoryConfig(dspDirPath, b); err != nil {
			return fmt.Errorf("failed to update repository config: %w", err)
		}

		// Convert and apply tracked paths
		if err := applyTrackedPaths(dspDirPath, b, absRepoRoot); err != nil {
			return fmt.Errorf("failed to apply tracked paths: %w", err)
		}

		fmt.Printf("\nImport completed successfully!\n")
		fmt.Printf("Repository: %s\n", repoName)
		fmt.Printf("Location: %s\n", absRepoRoot)
		fmt.Printf("DSP Directory: %s\n", b.Repository.DSPDir)
		fmt.Printf("Bundle ID: %s\n", b.ID)
		fmt.Printf("Changes applied: %d\n", len(b.Changes))

		return nil
	},
}

// downloadBundle downloads the bundle from the server
func downloadBundle(host, password, dspDir string) (string, error) {
	// Create bundles directory
	bundlesDir := filepath.Join(dspDir, "bundles")
	if err := os.MkdirAll(bundlesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create bundles directory: %w", err)
	}

	// Get export info from server
	exportInfo, err := getExportInfo(host, password)
	if err != nil {
		return "", fmt.Errorf("failed to get export info: %w", err)
	}

	// Verify export info
	if err := verifyExportInfo(exportInfo, password); err != nil {
		return "", fmt.Errorf("invalid export info: %w", err)
	}

	// For password auth, verify token
	if exportInfo.Auth == "password" {
		if exportInfo.Token == "" {
			return "", fmt.Errorf("missing security token")
		}
		expiry, err := time.Parse(time.RFC3339, exportInfo.TokenExpiry)
		if err != nil {
			return "", fmt.Errorf("invalid token expiry format: %w", err)
		}
		if time.Now().After(expiry) {
			return "", fmt.Errorf("security token has expired")
		}
	}

	// Perform key exchange if this is a password-based transfer
	if exportInfo.Auth == "password" {
		if err := performKeyExchange(host, password, exportInfo); err != nil {
			fmt.Printf("Warning: Key exchange failed: %v\n", err)
			fmt.Println("Continuing with password-based transfer only...")
		}
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Required for self-signed certificates
	}

	// Create HTTPS client
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 30 * time.Minute,
	}

	// Get host manager for certificate management
	hostManager, err := hostpkg.NewManager()
	if err != nil {
		return "", fmt.Errorf("failed to create host manager: %w", err)
	}

	// Get or create host entry
	hostEntry, err := hostManager.GetHost(exportInfo.Host)
	if err != nil {
		// Create new host entry
		hostEntry = &hostpkg.Host{
			Name:     exportInfo.Host,
			Trusted:  true, // Start trusted by default
			AddedAt:  time.Now(),
			LastUsed: time.Now(),
		}
	}

	// Create temporary file for download
	tempFile, err := os.CreateTemp(bundlesDir, "bundle-*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		// Only remove temp file if we return an error
		if err != nil {
			os.Remove(tempPath)
		}
	}()

	// Create URL with HTTPS
	url := fmt.Sprintf("https://%s:%d/download", exportInfo.Host, exportInfo.Port)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	req.Header.Set("X-Password", password)
	if exportInfo.Auth == "password" {
		req.Header.Set("X-One-Time-Token", exportInfo.Token)
	} else {
		// For user auth, use the password as the user identifier
		// since we're using public key authentication
		req.Header.Set("X-User", password)
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download bundle: %w", err)
	}
	defer resp.Body.Close()

	// Verify server certificate
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		fingerprint := sha256.Sum256(cert.Raw)
		fingerprintStr := hex.EncodeToString(fingerprint[:])

		// Verify against stored certificate if we have one
		if err := hostEntry.VerifyCertificate(fingerprintStr, cert.NotBefore, cert.NotAfter); err != nil {
			// If this is a new certificate, verify against export info
			if hostEntry.CertInfo == nil {
				if fingerprintStr != exportInfo.CertFingerprint {
					return "", fmt.Errorf("certificate fingerprint mismatch with export info")
				}
				// Store the new certificate info
				hostEntry.UpdateCertificate(fingerprintStr, cert.NotBefore, cert.NotAfter)
				if err := hostManager.UpdateHost(hostEntry); err != nil {
					return "", fmt.Errorf("failed to update host certificate info: %w", err)
				}
			} else {
				// Certificate mismatch with stored certificate
				return "", fmt.Errorf("certificate verification failed: %w", err)
			}
		}
	} else {
		return "", fmt.Errorf("no certificate received from server during download")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned error: %s", resp.Status)
	}

	// Download with progress tracking
	contentLength := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		nr, err := resp.Body.Read(buf)
		if nr > 0 {
			nw, err := tempFile.Write(buf[:nr])
			if err != nil {
				return "", fmt.Errorf("failed to write bundle data: %w", err)
			}
			if nr != nw {
				return "", fmt.Errorf("short write: %d != %d", nr, nw)
			}
			downloaded += int64(nw)
			if contentLength > 0 {
				// Print progress
				progress := float64(downloaded) / float64(contentLength) * 100
				fmt.Printf("\rDownloading: %.1f%%", progress)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read bundle data: %w", err)
		}
	}
	fmt.Println() // New line after progress

	// Close the temp file before reading it
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Read the downloaded data
	bundleData, err := os.ReadFile(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to read downloaded bundle: %w", err)
	}

	// If the bundle is encrypted (password auth), decrypt it
	if exportInfo.Encrypted {
		// Use combined key (password + token) for decryption
		combinedKey := password + exportInfo.Token
		decryptedData, err := crypto.DecryptWithPassphrase(bundleData, combinedKey)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt bundle: %w", err)
		}
		bundleData = decryptedData
	}

	// Verify bundle integrity
	b, err := bundle.LoadFromBytes(bundleData)
	if err != nil {
		return "", fmt.Errorf("invalid bundle format: %w", err)
	}
	if err := b.Verify(); err != nil {
		return "", fmt.Errorf("bundle verification failed: %w", err)
	}

	// Save bundle to final location
	bundlePath := filepath.Join(bundlesDir, fmt.Sprintf("%s.json", exportInfo.BundleID))
	if err := os.WriteFile(bundlePath, bundleData, 0644); err != nil {
		return "", fmt.Errorf("failed to save bundle: %w", err)
	}

	// Remove temporary file
	if err := os.Remove(tempPath); err != nil {
		fmt.Printf("Warning: failed to remove temporary file %s: %v\n", tempPath, err)
	}

	return bundlePath, nil
}

// performKeyExchange performs the key exchange handshake
func performKeyExchange(host string, password string, exportInfo *ExportInfo) error {
	// Get our public key
	keyManager, err := crypto.NewKeyManager()
	if err != nil {
		return fmt.Errorf("failed to create key manager: %w", err)
	}

	publicKey, err := keyManager.GetPublicKey()
	if err != nil {
		return fmt.Errorf("failed to get public key: %w", err)
	}

	// Get host manager
	hostManager, err := hostpkg.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create host manager: %w", err)
	}

	// Parse hostname from the host string
	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		hostname = host // If no port, use the whole string as hostname
	}

	// Check if host already exists
	existingHost, err := hostManager.GetHost(hostname)
	if err != nil {
		// Host doesn't exist, create new one
		existingHost = &hostpkg.Host{
			Name:      hostname,
			PublicKey: "",   // Will be set after exchange
			Trusted:   true, // Start trusted by default
			AddedAt:   time.Now(),
			LastUsed:  time.Now(),
		}
	}

	// Prepare key exchange request
	keyExchangeReq := struct {
		PublicKey string `json:"public_key"`
	}{
		PublicKey: publicKey,
	}

	// Send key exchange request
	url := fmt.Sprintf("http://%s:%d/key-exchange", exportInfo.Host, exportInfo.Port)
	reqBody, err := json.Marshal(keyExchangeReq)
	if err != nil {
		return fmt.Errorf("failed to marshal key exchange request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add password header
	req.Header.Set("X-Password", password)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send key exchange request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("key exchange failed: %s", resp.Status)
	}

	// Parse response
	var keyExchangeResp struct {
		Status        string `json:"status"`
		PublicKey     string `json:"public_key"`
		KeyExchangeID string `json:"key_exchange_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&keyExchangeResp); err != nil {
		return fmt.Errorf("failed to parse key exchange response: %w", err)
	}

	// Update host information
	existingHost.PublicKey = keyExchangeResp.PublicKey
	existingHost.LastUsed = time.Now()
	existingHost.IPAddress = exportInfo.Host
	existingHost.LastPort = exportInfo.Port
	existingHost.Trusted = true // Ensure host is trusted

	// Save host information
	if existingHost.AddedAt.IsZero() {
		// New host
		if err := hostManager.AddHost(existingHost); err != nil {
			return fmt.Errorf("failed to add host: %w", err)
		}
		fmt.Printf("Added new host '%s'\n", hostname)
	} else {
		// Update existing host
		if err := hostManager.UpdateHost(existingHost); err != nil {
			return fmt.Errorf("failed to update host: %w", err)
		}
		fmt.Printf("Updated host '%s'\n", hostname)
	}

	// Add exporter as a recipient in key manager
	if err := keyManager.AddRecipient(hostname, keyExchangeResp.PublicKey); err != nil {
		return fmt.Errorf("failed to add exporter as recipient: %w", err)
	}

	fmt.Printf("Successfully exchanged keys with %s\n", hostname)
	fmt.Printf("Key Exchange ID: %s\n", keyExchangeResp.KeyExchangeID)
	fmt.Printf("Added %s as a recipient. Future transfers can use --user authentication.\n", hostname)

	return nil
}

// getExportInfo gets the export information from the server
func getExportInfo(host, password string) (*ExportInfo, error) {
	// Parse host to get hostname and port
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		hostname = host
		port = "8080"
	}

	// Create custom TLS config that verifies the certificate
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // We'll verify the fingerprint manually
	}

	// Create HTTPS client
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	// Create URL with HTTPS
	url := fmt.Sprintf("https://%s:%s/status", hostname, port)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add password header
	req.Header.Set("X-Password", password)

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to export server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned error: %s", resp.Status)
	}

	// Verify server certificate
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		fingerprint := sha256.Sum256(cert.Raw)
		fingerprintStr := hex.EncodeToString(fingerprint[:])

		// Parse response to get expected fingerprint
		var info ExportInfo
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			return nil, fmt.Errorf("failed to parse export info: %w", err)
		}

		// Verify fingerprint
		if info.CertFingerprint != fingerprintStr {
			return nil, fmt.Errorf("certificate fingerprint mismatch")
		}

		// For password auth, verify we got a token
		if info.Auth == "password" {
			if info.Token == "" {
				return nil, fmt.Errorf("server did not provide a token")
			}
			if info.TokenExpiry == "" {
				return nil, fmt.Errorf("server did not provide token expiry")
			}
			// Verify token hasn't expired
			expiry, err := time.Parse(time.RFC3339, info.TokenExpiry)
			if err != nil {
				return nil, fmt.Errorf("invalid token expiry format: %w", err)
			}
			if time.Now().After(expiry) {
				return nil, fmt.Errorf("token has expired")
			}
		}

		return &info, nil
	}

	return nil, fmt.Errorf("no certificate received from server")
}

// verifyExportInfo verifies the export information
func verifyExportInfo(info *ExportInfo, password string) error {
	// Check expiration
	expires, err := time.Parse(time.RFC3339, info.Expires)
	if err != nil {
		return fmt.Errorf("invalid expiration time: %w", err)
	}
	if time.Now().After(expires) {
		return fmt.Errorf("export has expired")
	}

	// Verify authentication method
	if info.Auth != "password" {
		return fmt.Errorf("unsupported authentication method: %s", info.Auth)
	}

	// Verify password
	if info.Password != password {
		return fmt.Errorf("invalid password")
	}

	// Verify token exists and hasn't expired
	if info.Token == "" {
		return fmt.Errorf("missing security token")
	}
	if info.TokenExpiry == "" {
		return fmt.Errorf("missing token expiry")
	}
	expiry, err := time.Parse(time.RFC3339, info.TokenExpiry)
	if err != nil {
		return fmt.Errorf("invalid token expiry format: %w", err)
	}
	if time.Now().After(expiry) {
		return fmt.Errorf("security token has expired")
	}

	return nil
}

// updateRepositoryConfig updates the repository configuration
func updateRepositoryConfig(dspDir string, b *bundle.Bundle) error {
	// Create new config
	cfg := &config.Config{
		DSPDir:           filepath.Base(dspDir),
		DataDir:          b.Repository.DataDir,
		HashAlgorithm:    b.Repository.Config.HashAlgorithm,
		CompressionLevel: b.Repository.Config.CompressionLevel,
	}

	// Save config
	configPath := filepath.Join(dspDir, "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// applyTrackedPaths converts and applies tracked paths from the bundle
func applyTrackedPaths(dspDir string, b *bundle.Bundle, newRepoRoot string) error {
	// Load tracking config
	trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
	if err != nil {
		return fmt.Errorf("failed to load tracking config: %w", err)
	}

	// Add each tracked path directly from the bundle's tracking config
	for _, path := range b.Repository.TrackingConfig.Paths {
		// Create tracked path
		trackedPath := snapshot.TrackedPath{
			Path:     path.Path,
			IsDir:    path.IsDir,
			Excludes: path.Excludes,
		}

		// Add to tracking config
		if err := snapshot.AddTrackedPathWithExcludes(trackingConfig, trackedPath); err != nil {
			return fmt.Errorf("failed to add tracked path: %w", err)
		}
	}

	// Save tracking config
	if err := snapshot.SaveTrackingConfig(dspDir, trackingConfig); err != nil {
		return fmt.Errorf("failed to save tracking config: %w", err)
	}

	return nil
}
