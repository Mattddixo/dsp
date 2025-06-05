package exportcmd

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
	"github.com/Mattddixo/dsp/internal/bundle"
	"github.com/Mattddixo/dsp/internal/crypto"
	hostpkg "github.com/Mattddixo/dsp/internal/host"
	"github.com/urfave/cli/v2"
)

// ExportServer handles the HTTP server for bundle distribution
type ExportServer struct {
	server          *http.Server
	listener        net.Listener
	bundlePath      string
	outputPath      string
	auth            *ExportAuth
	downloads       int
	maxDownloads    int
	mu              sync.Mutex
	done            chan struct{}
	encrypted       bool // Only true for password auth
	exportInfo      ExportInfo
	certFingerprint string // Store certificate fingerprint for export info
}

// ExportAuth handles authentication for the export server
type ExportAuth struct {
	Method     string                // "password" or "user"
	Password   string                // For password auth
	Users      []string              // For user auth
	Downloaded map[string]bool       // Track who has downloaded
	Tokens     map[string]*TokenInfo // Map of tokens to their info
	TokenPool  []string              // Available tokens for new connections
	mu         sync.Mutex            // Mutex for token operations
}

// TokenInfo tracks token information
type TokenInfo struct {
	Token      string
	Expiry     time.Time
	Used       bool
	ClientIP   string    // IP address of the client that received this token
	AssignedAt time.Time // When the token was assigned
}

// ExportInfo contains information needed for import
type ExportInfo struct {
	Host            string    `json:"host"`
	Port            int       `json:"port"`
	BundleID        string    `json:"bundle_id"`
	Auth            string    `json:"auth_method"`
	Users           []string  `json:"users,omitempty"`
	Password        string    `json:"password,omitempty"`
	Signature       string    `json:"signature"`
	Expires         string    `json:"expires"`
	Encrypted       bool      `json:"encrypted"`
	OneTimeToken    string    `json:"one_time_token"`
	TokenExpiry     time.Time `json:"token_expiry"`
	CertFingerprint string    `json:"cert_fingerprint"` // Add certificate fingerprint

	// Key exchange information
	KeyExchange struct {
		ExporterPublicKey string `json:"exporter_public_key,omitempty"`
		ImporterPublicKey string `json:"importer_public_key,omitempty"`
		KeyExchangeID     string `json:"key_exchange_id"`
		Timestamp         string `json:"timestamp"`
	} `json:"key_exchange,omitempty"`
}

var Command = &cli.Command{
	Name:  "export",
	Usage: "Export a bundle for distribution",
	Description: `Export a bundle for distribution with optional encryption.
The command starts a server to distribute the bundle and provides import information.
When using password authentication, the bundle will be encrypted using the password.

Examples:
  # Export with password authentication and encryption
  dsp export -p "secret123" -f bundle.zip bundle.json

  # Export with user authentication (no encryption)
  dsp export -u "user1,user2" -f bundle.zip bundle.json

  # Export with download limit
  dsp export -p "secret123" -n 5 -f bundle.zip bundle.json`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			Usage:   "Password for authentication (mutually exclusive with -u)",
		},
		&cli.StringFlag{
			Name:    "user",
			Aliases: []string{"u"},
			Usage:   "Comma-separated list of users for authentication (mutually exclusive with -p)",
		},
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Usage:   "Output file name (default: bundle.zip)",
			Value:   "bundle.zip",
		},
		&cli.IntFlag{
			Name:  "port",
			Usage: "Port to use (default: from config)",
		},
		&cli.IntFlag{
			Name:     "number",
			Aliases:  []string{"n"},
			Usage:    "Number of allowed downloads (required)",
			Required: true,
		},
		&cli.DurationFlag{
			Name:    "timeout",
			Aliases: []string{"t"},
			Usage:   "Server timeout (default: 1h)",
			Value:   time.Hour,
		},
	},
	Action: func(c *cli.Context) error {
		// Validate arguments
		if c.NArg() != 1 {
			return fmt.Errorf("expected one bundle file argument")
		}

		// Validate auth options
		password := c.String("password")
		users := c.String("user")
		if password != "" && users != "" {
			return fmt.Errorf("cannot use both password and user authentication")
		}
		if password == "" && users == "" {
			return fmt.Errorf("must specify either password or user authentication")
		}

		// Load and validate bundle
		bundlePath := c.Args().First()
		b, err := bundle.Load(bundlePath)
		if err != nil {
			return fmt.Errorf("failed to load bundle: %w", err)
		}

		// Get certificate from key manager
		keyManager, err := crypto.NewKeyManager()
		if err != nil {
			return fmt.Errorf("failed to create key manager: %w", err)
		}

		cert, err := keyManager.GetCertificate()
		if err != nil {
			return fmt.Errorf("failed to get certificate: %w", err)
		}

		// Get certificate fingerprint
		fingerprint, err := keyManager.GetCertificateFingerprint()
		if err != nil {
			return fmt.Errorf("failed to get certificate fingerprint: %w", err)
		}

		// Create export server
		server := &ExportServer{
			bundlePath: bundlePath,
			outputPath: c.String("file"),
			auth: &ExportAuth{
				Method:     "password",
				Downloaded: make(map[string]bool),
				Tokens:     make(map[string]*TokenInfo),
			},
			maxDownloads:    c.Int("number"),
			done:            make(chan struct{}),
			encrypted:       password != "", // Enable encryption only for password auth
			certFingerprint: fingerprint,
		}

		// Set up authentication
		if password != "" {
			server.auth.Method = "password"
			server.auth.Password = password
			// Generate tokens for each allowed download
			if err := server.generateTokens(c.Int("number")); err != nil {
				return fmt.Errorf("failed to generate security tokens: %w", err)
			}
		} else {
			server.auth.Method = "user"
			server.auth.Users = splitAndTrim(users, ",")
			server.encrypted = false // No encryption for user auth
		}

		// Start server
		port := c.Int("port")
		if port == 0 {
			// TODO: Get port from config
			port = 8080
		}

		// Create TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		// Create listener with TLS
		listener, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}
		server.listener = listener

		// Set up HTTP server
		mux := http.NewServeMux()
		mux.HandleFunc("/download", server.handleDownload)
		mux.HandleFunc("/status", server.handleStatus)
		mux.HandleFunc("/key-exchange", server.handleKeyExchange)

		server.server = &http.Server{
			Handler: mux,
		}

		// Start server in background
		go func() {
			if err := server.server.Serve(listener); err != nil && err != http.ErrServerClosed {
				fmt.Printf("Server error: %v\n", err)
			}
		}()

		// Sign the export info
		keyManager, err = crypto.NewKeyManager()
		if err != nil {
			return fmt.Errorf("failed to create key manager: %w", err)
		}

		// Get host information
		hostname, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("failed to get hostname: %w", err)
		}

		// Create export info
		info := ExportInfo{
			Host:            hostname,
			Port:            port,
			BundleID:        b.ID,
			Auth:            server.auth.Method,
			Expires:         time.Now().Add(c.Duration("timeout")).Format(time.RFC3339),
			Encrypted:       server.encrypted,
			CertFingerprint: server.certFingerprint, // Include certificate fingerprint
		}

		if server.auth.Method == "password" {
			info.Password = server.auth.Password
			// Include token only for password auth
			if server.auth.Tokens != nil && len(server.auth.Tokens) > 0 {
				info.OneTimeToken = server.auth.Tokens[server.auth.TokenPool[0]].Token
				info.TokenExpiry = server.auth.Tokens[server.auth.TokenPool[0]].Expiry
			}
		} else {
			info.Users = server.auth.Users
		}

		// Sign the export info
		signature, err := keyManager.SignExportInfo(info)
		if err != nil {
			return fmt.Errorf("failed to sign export info: %w", err)
		}
		info.Signature = signature

		// Print export information
		infoJSON, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal export info: %w", err)
		}
		fmt.Printf("Export information:\n%s\n", string(infoJSON))
		fmt.Printf("\nServer running on port %d. Press Ctrl+C to stop.\n", port)

		// Wait for server to finish
		<-server.done
		return nil
	},
}

// handleDownload handles bundle download requests
func (s *ExportServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	// Check authentication first
	if !s.authenticateRequest(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get client IP
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		clientIP = r.RemoteAddr
	}

	// For password auth, verify token
	if s.auth.Method == "password" {
		token := r.Header.Get("X-One-Time-Token")
		if err := s.verifyToken(token, clientIP); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
	}

	// Check download limits
	s.mu.Lock()
	if s.maxDownloads > 0 && s.downloads >= s.maxDownloads {
		s.mu.Unlock()
		http.Error(w, "Download limit reached", http.StatusForbidden)
		s.shutdown()
		return
	}
	s.downloads++
	s.mu.Unlock()

	// For user auth, mark user as downloaded
	if s.auth.Method == "user" {
		user := r.Header.Get("X-User")
		s.mu.Lock()
		s.auth.Downloaded[user] = true
		s.mu.Unlock()
	}

	// Verify bundle exists
	if _, err := os.Stat(s.bundlePath); os.IsNotExist(err) {
		http.Error(w, "Bundle not found", http.StatusNotFound)
		return
	}

	// If using password auth, encrypt the bundle
	if s.auth.Method == "password" && s.encrypted {
		// Read the bundle file
		bundleData, err := os.ReadFile(s.bundlePath)
		if err != nil {
			http.Error(w, "Failed to read bundle", http.StatusInternalServerError)
			return
		}

		// Verify bundle integrity before encryption
		b, err := bundle.LoadFromBytes(bundleData)
		if err != nil {
			http.Error(w, "Invalid bundle format", http.StatusInternalServerError)
			return
		}
		if err := b.Verify(); err != nil {
			http.Error(w, "Bundle verification failed", http.StatusInternalServerError)
			return
		}

		// Create multiple recipients for each token
		var recipients []age.Recipient
		for _, tokenID := range s.auth.TokenPool {
			token := s.auth.Tokens[tokenID]
			if !token.Used {
				// Create a recipient for each valid token
				combinedKey := s.auth.Password + token.Token
				recipient, err := age.NewScryptRecipient(combinedKey)
				if err != nil {
					http.Error(w, "Failed to create recipient", http.StatusInternalServerError)
					return
				}
				recipients = append(recipients, recipient)
			}
		}

		if len(recipients) == 0 {
			http.Error(w, "No valid tokens available", http.StatusInternalServerError)
			return
		}

		// Create an encrypted writer with all recipients
		var buf bytes.Buffer
		encWriter, err := age.Encrypt(&buf, recipients...)
		if err != nil {
			http.Error(w, "Failed to create encrypted writer", http.StatusInternalServerError)
			return
		}

		// Write the data
		if _, err := encWriter.Write(bundleData); err != nil {
			http.Error(w, "Failed to write data", http.StatusInternalServerError)
			return
		}

		// Close the writer to finalize encryption
		if err := encWriter.Close(); err != nil {
			http.Error(w, "Failed to finalize encryption", http.StatusInternalServerError)
			return
		}

		encryptedData := buf.Bytes()

		// Set content type and serve encrypted data
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(encryptedData)))
		w.Write(encryptedData)

		// Mark the used token as used
		token := r.Header.Get("X-One-Time-Token")
		s.auth.mu.Lock()
		if tokenInfo, exists := s.auth.Tokens[token]; exists {
			tokenInfo.Used = true
		}
		s.auth.mu.Unlock()
	} else {
		// For user auth, serve the file as-is
		file, err := os.Open(s.bundlePath)
		if err != nil {
			http.Error(w, "Failed to open bundle", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Get file info for content length
		fileInfo, err := file.Stat()
		if err != nil {
			http.Error(w, "Failed to get file info", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		http.ServeContent(w, r, filepath.Base(s.bundlePath), fileInfo.ModTime(), file)
	}

	// Check if we should shutdown
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.auth.Method == "user" {
		// For user auth, shutdown when all users have downloaded
		allDownloaded := true
		for _, user := range s.auth.Users {
			if !s.auth.Downloaded[user] {
				allDownloaded = false
				break
			}
		}
		if allDownloaded {
			s.shutdown()
		}
	} else if s.maxDownloads > 0 && s.downloads >= s.maxDownloads {
		// For password auth, shutdown when download limit is reached
		s.shutdown()
	}
}

// handleStatus handles status requests
func (s *ExportServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Check password authentication first
	if !s.authenticateRequest(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get client IP
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		clientIP = r.RemoteAddr
	}

	// For password auth, assign a token if client doesn't have one
	var token string
	if s.auth.Method == "password" {
		var err error
		token, err = s.assignToken(clientIP)
		if err != nil {
			http.Error(w, "No tokens available", http.StatusForbidden)
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create status response
	status := struct {
		Downloads    int      `json:"downloads"`
		MaxDownloads int      `json:"max_downloads"`
		AuthMethod   string   `json:"auth_method"`
		Users        []string `json:"users,omitempty"`
		Downloaded   []string `json:"downloaded,omitempty"`
		Token        string   `json:"token,omitempty"`
		TokenExpiry  string   `json:"token_expiry,omitempty"`
	}{
		Downloads:    s.downloads,
		MaxDownloads: s.maxDownloads,
		AuthMethod:   s.auth.Method,
	}

	if s.auth.Method == "user" {
		status.Users = s.auth.Users
		for user, downloaded := range s.auth.Downloaded {
			if downloaded {
				status.Downloaded = append(status.Downloaded, user)
			}
		}
	} else if token != "" {
		status.Token = token
		status.TokenExpiry = s.auth.Tokens[token].Expiry.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// authenticateRequest authenticates the request
func (s *ExportServer) authenticateRequest(r *http.Request) bool {
	if s.auth.Method == "password" {
		// Password authentication
		password := r.Header.Get("X-Password")
		return password == s.auth.Password
	} else {
		// User authentication
		user := r.Header.Get("X-User")
		if user == "" {
			return false
		}

		// Check if user is authorized
		authorized := false
		for _, u := range s.auth.Users {
			if u == user {
				authorized = true
				break
			}
		}
		if !authorized {
			return false
		}

		// Mark user as downloaded
		s.auth.Downloaded[user] = true
		return true
	}
}

// shutdown gracefully shuts down the server
func (s *ExportServer) shutdown() {
	close(s.done)
	s.server.Close()
}

// splitAndTrim splits a string and trims each part
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}

// handleKeyExchange handles the key exchange handshake
func (s *ExportServer) handleKeyExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check password authentication first
	if !s.authenticateRequest(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get host manager
	hostManager, err := hostpkg.NewManager()
	if err != nil {
		http.Error(w, "Failed to get host manager", http.StatusInternalServerError)
		return
	}

	// Get client IP and port
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		clientIP = r.RemoteAddr // If no port, use the whole string
	}

	// Read importer's public key from request
	var keyExchange struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&keyExchange); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the public key format
	if !strings.HasPrefix(keyExchange.PublicKey, "age1") {
		http.Error(w, "Invalid public key format", http.StatusBadRequest)
		return
	}

	// Get exporter's public key
	keyManager, err := crypto.NewKeyManager()
	if err != nil {
		http.Error(w, "Failed to get key manager", http.StatusInternalServerError)
		return
	}

	exporterKey, err := keyManager.GetPublicKey()
	if err != nil {
		http.Error(w, "Failed to get exporter's public key", http.StatusInternalServerError)
		return
	}

	// Check if host already exists
	existingHost, err := hostManager.GetHost(clientIP)
	if err != nil {
		// Host doesn't exist, create new one
		existingHost = &hostpkg.Host{
			Name:      clientIP,
			PublicKey: keyExchange.PublicKey,
			Trusted:   true, // Start trusted by default
			AddedAt:   time.Now(),
			LastUsed:  time.Now(),
			IPAddress: clientIP,
			LastPort:  s.exportInfo.Port,
		}
		if err := hostManager.AddHost(existingHost); err != nil {
			http.Error(w, "Failed to add host", http.StatusInternalServerError)
			return
		}
	} else {
		// Update existing host
		existingHost.PublicKey = keyExchange.PublicKey
		existingHost.LastUsed = time.Now()
		existingHost.IPAddress = clientIP
		existingHost.LastPort = s.exportInfo.Port
		existingHost.Trusted = true // Ensure host is trusted
		if err := hostManager.UpdateHost(existingHost); err != nil {
			http.Error(w, "Failed to update host", http.StatusInternalServerError)
			return
		}
	}

	// Add importer as a recipient
	if err := keyManager.AddRecipient(clientIP, keyExchange.PublicKey); err != nil {
		http.Error(w, "Failed to add importer as recipient", http.StatusInternalServerError)
		return
	}

	// Update export info with both keys
	s.mu.Lock()
	s.exportInfo.KeyExchange.ImporterPublicKey = keyExchange.PublicKey
	s.exportInfo.KeyExchange.ExporterPublicKey = exporterKey
	s.exportInfo.KeyExchange.Timestamp = time.Now().Format(time.RFC3339)
	s.exportInfo.KeyExchange.KeyExchangeID = fmt.Sprintf("keyx-%s", s.exportInfo.BundleID)
	keyExchangeID := s.exportInfo.KeyExchange.KeyExchangeID
	s.mu.Unlock()

	// Return success with exporter's public key
	response := struct {
		Status        string `json:"status"`
		PublicKey     string `json:"public_key"`
		KeyExchangeID string `json:"key_exchange_id"`
	}{
		Status:        "success",
		PublicKey:     exporterKey,
		KeyExchangeID: keyExchangeID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// generateTokens generates a pool of one-time tokens
func (s *ExportServer) generateTokens(count int) error {
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()

	// Initialize token maps if needed
	if s.auth.Tokens == nil {
		s.auth.Tokens = make(map[string]*TokenInfo)
	}
	if s.auth.TokenPool == nil {
		s.auth.TokenPool = make([]string, 0, count)
	}

	// Generate requested number of tokens
	for i := 0; i < count; i++ {
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}

		tokenStr := base64.URLEncoding.EncodeToString(token)
		s.auth.TokenPool = append(s.auth.TokenPool, tokenStr)
	}

	return nil
}

// assignToken assigns a token to a client
func (s *ExportServer) assignToken(clientIP string) (string, error) {
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()

	// Check if client already has a token
	for token, info := range s.auth.Tokens {
		if info.ClientIP == clientIP && !info.Used && time.Now().Before(info.Expiry) {
			return token, nil
		}
	}

	// Get next available token
	if len(s.auth.TokenPool) == 0 {
		return "", fmt.Errorf("no tokens available")
	}

	// Pop token from pool
	token := s.auth.TokenPool[0]
	s.auth.TokenPool = s.auth.TokenPool[1:]

	// Create token info
	s.auth.Tokens[token] = &TokenInfo{
		Token:      token,
		Expiry:     time.Now().Add(5 * time.Minute),
		Used:       false,
		ClientIP:   clientIP,
		AssignedAt: time.Now(),
	}

	return token, nil
}

// verifyToken verifies a token is valid and marks it as used
func (s *ExportServer) verifyToken(token, clientIP string) error {
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()

	info, exists := s.auth.Tokens[token]
	if !exists {
		return fmt.Errorf("invalid token")
	}

	if info.Used {
		return fmt.Errorf("token already used")
	}

	if time.Now().After(info.Expiry) {
		return fmt.Errorf("token expired")
	}

	if info.ClientIP != clientIP {
		return fmt.Errorf("token assigned to different client")
	}

	info.Used = true
	return nil
}
