package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/T-I-M/dsp/config"
	"github.com/T-I-M/dsp/internal/snapshot"
)

// Bundle represents a bundle of changes
type Bundle struct {
	// Metadata about the bundle
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
	Description string    `json:"description"`

	// Source and target snapshots
	SourceSnapshot string `json:"source_snapshot"`
	TargetSnapshot string `json:"target_snapshot"`

	// Repository information
	Repository struct {
		// Basic repository info
		Name    string `json:"name"`
		DSPDir  string `json:"dsp_dir"`
		DataDir string `json:"data_dir"`

		// Configuration
		Config struct {
			HashAlgorithm    string `json:"hash_algorithm"`
			CompressionLevel int    `json:"compression_level"`
		} `json:"config"`

		// Tracking configuration from the source
		TrackingConfig *snapshot.TrackingConfig `json:"tracking_config"`
	} `json:"repository"`

	// Changes in this bundle
	Changes []Change `json:"changes"`
}

// Change represents a single change in the bundle
type Change struct {
	Path          string    `json:"path"`
	Type          string    `json:"type"` // "add", "modify", "delete"
	Hash          string    `json:"hash"`
	Size          int64     `json:"size"`
	ModifiedTime  time.Time `json:"modified_time"`
	IsSymlink     bool      `json:"is_symlink"`
	SymlinkTarget string    `json:"symlink_target,omitempty"`
}

// New creates a new bundle from the given snapshots
func New(sourceSnapshot, targetSnapshot string) (*Bundle, error) {
	// Generate bundle ID (timestamp-based)
	bundleID := time.Now().Format("20060102150405")

	// Load snapshots
	source, err := snapshot.Load(sourceSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to load source snapshot: %w", err)
	}

	target, err := snapshot.Load(targetSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to load target snapshot: %w", err)
	}

	// Get repository information
	repoPath := filepath.Dir(filepath.Dir(sourceSnapshot)) // Go up two levels from snapshot to repo root
	cfg, err := config.NewWithRepo(repoPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load repository config: %w", err)
	}

	// Create bundle
	bundle := &Bundle{
		ID:             bundleID,
		CreatedAt:      time.Now(),
		CreatedBy:      os.Getenv("USERNAME"),
		SourceSnapshot: filepath.Base(sourceSnapshot),
		TargetSnapshot: filepath.Base(targetSnapshot),
	}

	// Set repository information
	bundle.Repository.Name = filepath.Base(repoPath)
	bundle.Repository.DSPDir = cfg.DSPDir
	bundle.Repository.DataDir = cfg.DataDir
	bundle.Repository.Config.HashAlgorithm = cfg.HashAlgorithm
	bundle.Repository.Config.CompressionLevel = cfg.CompressionLevel

	// Load tracking configuration
	trackingConfig, err := snapshot.LoadTrackingConfig(filepath.Join(repoPath, cfg.DSPDir))
	if err != nil {
		return nil, fmt.Errorf("failed to load tracking config: %w", err)
	}
	bundle.Repository.TrackingConfig = trackingConfig

	// Compute changes between snapshots
	if err := bundle.computeChanges(source, target); err != nil {
		return nil, fmt.Errorf("failed to compute changes: %w", err)
	}

	return bundle, nil
}

// computeChanges computes the changes between two snapshots
func (b *Bundle) computeChanges(source, target *snapshot.Snapshot) error {
	// Create maps for quick lookup
	sourceFiles := make(map[string]snapshot.File)
	targetFiles := make(map[string]snapshot.File)

	// Add source files to map
	for _, f := range source.Files {
		sourceFiles[f.Path] = f
	}

	// Add target files to map and compute changes
	for _, f := range target.Files {
		targetFiles[f.Path] = f

		// Check if file exists in source
		sourceFile, exists := sourceFiles[f.Path]
		if !exists {
			// File was added
			b.Changes = append(b.Changes, Change{
				Path:          f.Path,
				Type:          "add",
				Hash:          f.Hash,
				Size:          f.Size,
				ModifiedTime:  f.ModifiedTime,
				IsSymlink:     f.IsSymlink,
				SymlinkTarget: f.SymlinkTarget,
			})
			continue
		}

		// File exists in both, check if modified
		if sourceFile.Hash != f.Hash {
			b.Changes = append(b.Changes, Change{
				Path:          f.Path,
				Type:          "modify",
				Hash:          f.Hash,
				Size:          f.Size,
				ModifiedTime:  f.ModifiedTime,
				IsSymlink:     f.IsSymlink,
				SymlinkTarget: f.SymlinkTarget,
			})
		}
	}

	// Check for deleted files
	for _, f := range source.Files {
		if _, exists := targetFiles[f.Path]; !exists {
			b.Changes = append(b.Changes, Change{
				Path:          f.Path,
				Type:          "delete",
				Hash:          f.Hash,
				Size:          f.Size,
				ModifiedTime:  f.ModifiedTime,
				IsSymlink:     f.IsSymlink,
				SymlinkTarget: f.SymlinkTarget,
			})
		}
	}

	return nil
}

// Save saves the bundle to a file
func (b *Bundle) Save(path string) error {
	// Create the bundle directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create bundle directory: %w", err)
	}

	// Marshal the bundle metadata
	metadata, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bundle metadata: %w", err)
	}

	// Write the metadata file
	if err := os.WriteFile(path, metadata, 0644); err != nil {
		return fmt.Errorf("failed to write bundle file: %w", err)
	}

	return nil
}

// Load loads a bundle from a file
func Load(path string) (*Bundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle file: %w", err)
	}

	var bundle Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to parse bundle: %w", err)
	}

	// Validate bundle
	if bundle.ID == "" {
		return nil, fmt.Errorf("bundle has no ID")
	}
	if bundle.Repository.DSPDir == "" {
		return nil, fmt.Errorf("bundle has no DSP directory")
	}
	if bundle.Repository.DataDir == "" {
		return nil, fmt.Errorf("bundle has no data directory")
	}
	if bundle.SourceSnapshot == "" {
		return nil, fmt.Errorf("bundle has no source snapshot")
	}
	if len(bundle.Changes) == 0 {
		return nil, fmt.Errorf("bundle has no changes")
	}

	return &bundle, nil
}

// LoadFromBytes loads a bundle from raw bytes
func LoadFromBytes(data []byte) (*Bundle, error) {
	var b Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("failed to parse bundle: %w", err)
	}
	return &b, nil
}

// Verify checks the bundle's integrity
func (b *Bundle) Verify() error {
	// Check required fields
	if b.ID == "" {
		return fmt.Errorf("bundle has no ID")
	}
	if b.CreatedAt.IsZero() {
		return fmt.Errorf("bundle has no creation time")
	}
	if b.CreatedBy == "" {
		return fmt.Errorf("bundle has no creator")
	}
	if b.SourceSnapshot == "" {
		return fmt.Errorf("bundle has no source snapshot")
	}
	if b.TargetSnapshot == "" {
		return fmt.Errorf("bundle has no target snapshot")
	}

	// Check repository information
	if b.Repository.Name == "" {
		return fmt.Errorf("bundle has no repository name")
	}
	if b.Repository.DSPDir == "" {
		return fmt.Errorf("bundle has no DSP directory")
	}
	if b.Repository.DataDir == "" {
		return fmt.Errorf("bundle has no data directory")
	}
	if b.Repository.Config.HashAlgorithm == "" {
		return fmt.Errorf("bundle has no hash algorithm")
	}
	if b.Repository.Config.CompressionLevel < 1 || b.Repository.Config.CompressionLevel > 9 {
		return fmt.Errorf("invalid compression level: %d", b.Repository.Config.CompressionLevel)
	}
	if b.Repository.TrackingConfig == nil {
		return fmt.Errorf("bundle has no tracking configuration")
	}

	// Check changes
	if len(b.Changes) == 0 {
		return fmt.Errorf("bundle has no changes")
	}

	// Validate each change
	for i, change := range b.Changes {
		if change.Path == "" {
			return fmt.Errorf("change %d has no path", i)
		}
		if change.Type == "" {
			return fmt.Errorf("change %d has no type", i)
		}
		if change.Type != "add" && change.Type != "modify" && change.Type != "delete" {
			return fmt.Errorf("change %d has invalid type: %s", i, change.Type)
		}
		if change.Hash == "" {
			return fmt.Errorf("change %d has no hash", i)
		}
		if change.Size < 0 {
			return fmt.Errorf("change %d has invalid size: %d", i, change.Size)
		}
		if change.IsSymlink && change.SymlinkTarget == "" {
			return fmt.Errorf("change %d is a symlink but has no target", i)
		}
	}

	return nil
}
