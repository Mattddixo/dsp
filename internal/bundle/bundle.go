package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Mattddixo/dsp/config"
	"github.com/Mattddixo/dsp/internal/snapshot"
	"github.com/Mattddixo/dsp/pkg/utils"
)

// Bundle represents a bundle of changes
type Bundle struct {
	// Metadata about the bundle
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
	Description string    `json:"description"`
	IsInitial   bool      `json:"is_initial"` // New field for initial bundles

	// Source and target snapshots
	SourceSnapshot string `json:"source_snapshot,omitempty"` // Optional for initial bundles
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

	// File contents for new and modified files
	FileContents map[string][]byte `json:"-"` // Not serialized to JSON
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
	ContentHash   string    `json:"content_hash,omitempty"` // Hash of the file content in the bundle
}

// New creates a new bundle from the given snapshots
func New(sourceSnapshot, targetSnapshot string) (*Bundle, error) {
	// Generate bundle ID (timestamp-based)
	bundleID := time.Now().Format("20060102150405")

	// Check if this is an initial bundle
	isInitial := sourceSnapshot == ""

	// Load target snapshot
	target, err := snapshot.Load(targetSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to load target snapshot: %w", err)
	}

	// Get repository information
	repoPath := filepath.Dir(filepath.Dir(targetSnapshot)) // Go up two levels from snapshot to repo root
	cfg, err := config.NewWithRepo(repoPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load repository config: %w", err)
	}

	// Create bundle
	bundle := &Bundle{
		ID:             bundleID,
		CreatedAt:      time.Now(),
		CreatedBy:      os.Getenv("USERNAME"),
		IsInitial:      isInitial,
		TargetSnapshot: filepath.Base(targetSnapshot),
		FileContents:   make(map[string][]byte),
	}

	// Set source snapshot if not initial
	if !isInitial {
		bundle.SourceSnapshot = filepath.Base(sourceSnapshot)
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

	// For initial bundle, treat all files as additions
	if isInitial {
		for _, f := range target.Files {
			// Read and compress file content
			content, err := readAndCompressFile(f.Path, cfg.CompressionLevel)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", f.Path, err)
			}

			// Add to bundle
			bundle.Changes = append(bundle.Changes, Change{
				Path:          f.Path,
				Type:          "add",
				Hash:          f.Hash,
				Size:          f.Size,
				ModifiedTime:  f.ModifiedTime,
				IsSymlink:     f.IsSymlink,
				SymlinkTarget: f.SymlinkTarget,
				ContentHash:   utils.HashBytes(content),
			})
			bundle.FileContents[f.Path] = content
		}
		return bundle, nil
	}

	// Load source snapshot for comparison
	source, err := snapshot.Load(sourceSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to load source snapshot: %w", err)
	}

	// Compute changes between snapshots
	if err := bundle.computeChanges(source, target, cfg.CompressionLevel); err != nil {
		return nil, fmt.Errorf("failed to compute changes: %w", err)
	}

	return bundle, nil
}

// readAndCompressFile reads and compresses a file
func readAndCompressFile(path string, compressionLevel int) ([]byte, error) {
	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Compress content
	compressed, err := utils.Compress(content, compressionLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to compress file: %w", err)
	}

	return compressed, nil
}

// computeChanges computes the changes between two snapshots
func (b *Bundle) computeChanges(source, target *snapshot.Snapshot, compressionLevel int) error {
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
			// File was added, read and compress content
			content, err := readAndCompressFile(f.Path, compressionLevel)
			if err != nil {
				return fmt.Errorf("failed to read new file %s: %w", f.Path, err)
			}

			b.Changes = append(b.Changes, Change{
				Path:          f.Path,
				Type:          "add",
				Hash:          f.Hash,
				Size:          f.Size,
				ModifiedTime:  f.ModifiedTime,
				IsSymlink:     f.IsSymlink,
				SymlinkTarget: f.SymlinkTarget,
				ContentHash:   utils.HashBytes(content),
			})
			b.FileContents[f.Path] = content
			continue
		}

		// File exists in both, check if modified
		if sourceFile.Hash != f.Hash {
			// File was modified, read and compress new content
			content, err := readAndCompressFile(f.Path, compressionLevel)
			if err != nil {
				return fmt.Errorf("failed to read modified file %s: %w", f.Path, err)
			}

			b.Changes = append(b.Changes, Change{
				Path:          f.Path,
				Type:          "modify",
				Hash:          f.Hash,
				Size:          f.Size,
				ModifiedTime:  f.ModifiedTime,
				IsSymlink:     f.IsSymlink,
				SymlinkTarget: f.SymlinkTarget,
				ContentHash:   utils.HashBytes(content),
			})
			b.FileContents[f.Path] = content
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

	// Ensure path has .zip extension
	if filepath.Ext(path) != ".zip" {
		path = path[:len(path)-len(filepath.Ext(path))] + ".zip"
	}

	// Create a temporary directory for file contents
	tempDir, err := os.MkdirTemp("", "dsp-bundle-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create contents directory
	contentsDir := filepath.Join(tempDir, "contents")
	if err := os.MkdirAll(contentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create contents directory: %w", err)
	}

	// Save file contents
	for _, content := range b.FileContents {
		contentPath := filepath.Join(contentsDir, utils.HashBytes(content))
		if err := os.WriteFile(contentPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write file content: %w", err)
		}
	}

	// Create a zip archive containing metadata and file contents
	if err := utils.CreateZipArchive(path, map[string]string{
		"metadata.json": "",                // Empty initially
		"contents":      contentsDir + "/", // Add trailing slash to indicate directory
	}); err != nil {
		return fmt.Errorf("failed to create bundle archive: %w", err)
	}

	// Marshal the bundle metadata
	metadata, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bundle metadata: %w", err)
	}

	// Update the metadata in the zip file
	if err := utils.UpdateZipFile(path, "metadata.json", metadata); err != nil {
		return fmt.Errorf("failed to update bundle metadata: %w", err)
	}

	return nil
}

// Load loads a bundle from a file
func Load(path string) (*Bundle, error) {
	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "dsp-bundle-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract the bundle archive
	if err := utils.ExtractZipArchive(path, tempDir); err != nil {
		return nil, fmt.Errorf("failed to extract bundle: %w", err)
	}

	// Read and parse metadata
	metadataPath := filepath.Join(tempDir, "metadata.json")
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle metadata: %w", err)
	}

	var bundle Bundle
	if err := json.Unmarshal(metadata, &bundle); err != nil {
		return nil, fmt.Errorf("failed to parse bundle: %w", err)
	}

	// Load file contents
	bundle.FileContents = make(map[string][]byte)
	contentsDir := filepath.Join(tempDir, "contents")
	entries, err := os.ReadDir(contentsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read contents directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		contentPath := filepath.Join(contentsDir, entry.Name())
		content, err := os.ReadFile(contentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file content: %w", err)
		}
		// Find the change that uses this content
		for _, change := range bundle.Changes {
			if change.ContentHash == entry.Name() {
				bundle.FileContents[change.Path] = content
				break
			}
		}
	}

	// Validate bundle
	if err := bundle.Verify(); err != nil {
		return nil, fmt.Errorf("bundle verification failed: %w", err)
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
