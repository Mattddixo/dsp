package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigKeyType is the type for the config context key
type ConfigKeyType struct{}

// ConfigKey is the context key for storing the config
var ConfigKey = ConfigKeyType{}

// Config holds all configuration values for DSP
type Config struct {
	// DSPDir is the directory where DSP stores its metadata
	DSPDir string `yaml:"dsp_dir"`

	// DataDir is the directory where DSP stores its metadata
	DataDir string `yaml:"data_dir"`

	// HashAlgorithm is the algorithm used for file hashing
	HashAlgorithm string `yaml:"hash_algorithm"`

	// CompressionLevel is the compression level for bundles (1-9)
	CompressionLevel int `yaml:"compression_level"`
}

// normalizePath converts a path to the OS-specific format and cleans it
func normalizePath(path string) string {
	// Convert to OS-specific path separators
	path = filepath.FromSlash(path)
	// Clean the path (remove . and .., resolve symlinks)
	return filepath.Clean(path)
}

// New creates a new Config with values from config file, environment variables, or defaults
func New() (*Config, error) {
	return NewWithRepo("", "")
}

// NewWithRepo creates a new Config for a specific repository
func NewWithRepo(repoPath, dspDir string) (*Config, error) {
	// Create config with defaults from embedded YAML
	var cfg Config
	if err := yaml.Unmarshal([]byte(DefaultConfigYAML), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse default config: %w", err)
	}

	// If repository path is provided, load its config
	if repoPath != "" {
		// If DSP directory is not provided, use the default
		if dspDir == "" {
			dspDir = cfg.DSPDir
		}

		// Load repository config
		configPath := filepath.Join(repoPath, dspDir, "config.yaml")
		if data, err := os.ReadFile(configPath); err == nil {
			// Found repository config, parse it
			var repoCfg Config
			if err := yaml.Unmarshal(data, &repoCfg); err == nil {
				// Override defaults with repository config
				cfg = repoCfg
			}
		}
	}

	// Override with environment variables if they exist
	if envDataDir := os.Getenv("DSP_DATA_DIR"); envDataDir != "" {
		cfg.DataDir = normalizePath(envDataDir)
	}
	if envHashAlgo := os.Getenv("DSP_HASH_ALGORITHM"); envHashAlgo != "" {
		cfg.HashAlgorithm = envHashAlgo
	}
	if envCompLevel := os.Getenv("DSP_COMPRESSION_LEVEL"); envCompLevel != "" {
		if level, err := strconv.Atoi(envCompLevel); err == nil {
			cfg.CompressionLevel = level
		}
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Save saves the current configuration to a YAML file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validate checks if the configuration is valid
func (c *Config) validate() error {
	// Validate hash algorithm
	valid := false
	for _, algo := range ValidHashAlgorithms {
		if c.HashAlgorithm == algo {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid hash algorithm: %s, must be one of: %s",
			c.HashAlgorithm, strings.Join(ValidHashAlgorithms, ", "))
	}

	// Validate compression level
	if c.CompressionLevel < MinCompressionLevel || c.CompressionLevel > MaxCompressionLevel {
		return fmt.Errorf("invalid compression level: %d, must be between %d and %d",
			c.CompressionLevel, MinCompressionLevel, MaxCompressionLevel)
	}

	return nil
}

// GetDataDirPath returns the absolute path to the data directory
func (c *Config) GetDataDirPath() (string, error) {
	// If DataDir is absolute, return it as is
	if filepath.IsAbs(c.DataDir) {
		return c.DataDir, nil
	}

	// Otherwise, make it relative to the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	return filepath.Join(wd, c.DataDir), nil
}

// EnsureDataDir creates the data directory if it doesn't exist
func (c *Config) EnsureDataDir() error {
	path, err := c.GetDataDirPath()
	if err != nil {
		return fmt.Errorf("failed to get data directory path: %w", err)
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	return nil
}

// String returns a string representation of the configuration
func (c *Config) String() string {
	var sb strings.Builder
	sb.WriteString("Configuration:\n")
	sb.WriteString(fmt.Sprintf("  Data Directory: %s\n", c.DataDir))
	sb.WriteString(fmt.Sprintf("  Hash Algorithm: %s\n", c.HashAlgorithm))
	sb.WriteString(fmt.Sprintf("  Compression Level: %d\n", c.CompressionLevel))
	return sb.String()
}

// WithContext adds the config to the context
func (c *Config) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ConfigKey, c)
}

// GetConfigFromContext retrieves the config from a context
func GetConfigFromContext(ctx context.Context) (*Config, error) {
	if cfg, ok := ctx.Value(ConfigKey).(*Config); ok {
		return cfg, nil
	}
	return nil, fmt.Errorf("no config found in context")
}
