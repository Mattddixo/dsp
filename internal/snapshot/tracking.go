package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// TrackedPath represents a single tracked path
type TrackedPath struct {
	Path     string   `yaml:"path"`               // Absolute path to the file or directory
	IsDir    bool     `yaml:"is_dir"`             // Whether this is a directory
	Excludes []string `yaml:"excludes,omitempty"` // Patterns to exclude within this path
	// Exclude patterns use Go's filepath.Match syntax:
	//   * matches any sequence of non-separator characters
	//   ? matches any single non-separator character
	//   [sequence] matches any single character in sequence
	//   [!sequence] matches any single character not in sequence
	//
	// Examples:
	//   - "*.log" ignores all .log files
	//   - "temp/*" ignores everything in temp directories
	//   - "node_modules" ignores node_modules directories
	//   - "*.{log,tmp}" ignores files ending in .log or .tmp
	//   - "[Tt]emp/*" ignores everything in Temp or temp directories
	//
	// Note: Exclude patterns are only valid for directories.
	// When a pattern matches a directory, its entire contents are excluded.
	// Patterns are matched against the relative path from the tracked directory.
}

// Change represents a change to a tracked path
type Change struct {
	Timestamp time.Time `yaml:"timestamp"`
	Type      string    `yaml:"type"` // "add", "modify", "delete"
	User      string    `yaml:"user"`
	Details   string    `yaml:"details,omitempty"`
}

// RepositoryState represents the current state of a repository
type RepositoryState struct {
	IsClosed     bool      `yaml:"is_closed"`               // Whether the repository is closed
	ClosedAt     time.Time `yaml:"closed_at,omitempty"`     // When the repository was closed
	ClosedBy     string    `yaml:"closed_by,omitempty"`     // Who closed the repository
	LastModified time.Time `yaml:"last_modified,omitempty"` // Last modification time
}

// TrackingConfig holds the configuration for tracked paths
type TrackingConfig struct {
	State RepositoryState `yaml:"state"` // Repository state information
	Paths []TrackedPath   `yaml:"paths"`
}

// LoadTrackingConfig loads the tracking configuration from the DSP directory
func LoadTrackingConfig(dspDir string) (*TrackingConfig, error) {
	configPath := filepath.Join(dspDir, "tracking.yaml")

	// If file doesn't exist, create empty config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &TrackingConfig{Paths: []TrackedPath{}}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tracking config: %w", err)
	}

	var config TrackingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse tracking config: %w", err)
	}

	return &config, nil
}

// SaveTrackingConfig saves the tracking configuration to the DSP directory
func SaveTrackingConfig(dspDir string, config *TrackingConfig) error {
	// Ensure DSP directory exists
	if err := os.MkdirAll(dspDir, 0755); err != nil {
		return fmt.Errorf("failed to create DSP directory: %w", err)
	}

	// Initialize excludes for each path if nil
	for i := range config.Paths {
		if config.Paths[i].Excludes == nil {
			config.Paths[i].Excludes = make([]string, 0)
		}
	}

	// Convert to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal tracking config: %w", err)
	}

	// Write to file
	trackingFile := filepath.Join(dspDir, "tracking.yaml")
	if err := os.WriteFile(trackingFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write tracking config: %w", err)
	}

	return nil
}

// AddTrackedPathWithExcludes adds a new path to the tracking configuration with exclude patterns
// The exclude patterns will be used to filter files and directories when creating snapshots.
// Patterns are matched against the relative path from the tracked directory.
// For example, if tracking "/path/to/project" with exclude "*.log", then
// "/path/to/project/file.log" will be excluded.
func AddTrackedPathWithExcludes(config *TrackingConfig, path TrackedPath) error {
	// Check if path exists
	info, err := os.Stat(path.Path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	// Verify path is a directory if excludes are specified
	if len(path.Excludes) > 0 && !info.IsDir() {
		return fmt.Errorf("exclude patterns can only be specified for directories")
	}

	// Check if path is already tracked
	for _, p := range config.Paths {
		if p.Path == path.Path {
			return fmt.Errorf("path is already tracked")
		}
	}

	// Add new tracked path
	config.Paths = append(config.Paths, path)

	return nil
}

// AddTrackedPath adds a new path to the tracking configuration
func AddTrackedPath(config *TrackingConfig, path string) error {
	// Normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	// Create tracked path
	trackedPath := TrackedPath{
		Path:  absPath,
		IsDir: info.IsDir(),
	}

	return AddTrackedPathWithExcludes(config, trackedPath)
}

// RemoveTrackedPath removes a path from the tracking configuration
func RemoveTrackedPath(config *TrackingConfig, path string) error {
	// Normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Find and remove the path
	for i, p := range config.Paths {
		if p.Path == absPath {
			config.Paths = append(config.Paths[:i], config.Paths[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("path is not tracked")
}

// GetTrackedPaths returns all currently tracked paths
func GetTrackedPaths(config *TrackingConfig) []TrackedPath {
	return config.Paths
}

// UpdateLastSync is no longer needed as we don't track sync state
func UpdateLastSync(config *TrackingConfig, path string) error {
	return nil // Sync state is now tracked in snapshots
}

// AddChange is no longer needed as changes are tracked in snapshots
func AddChange(config *TrackingConfig, path string, changeType string, username string, details string) error {
	return nil // Changes are now tracked in snapshots
}

// CloseRepository marks a repository as closed
func CloseRepository(config *TrackingConfig, username string) error {
	config.State = RepositoryState{
		IsClosed:     true,
		ClosedAt:     time.Now(),
		ClosedBy:     username,
		LastModified: time.Now(),
	}
	return nil
}

// ReopenRepository reopens a closed repository
func ReopenRepository(config *TrackingConfig, username string) error {
	if !config.State.IsClosed {
		return fmt.Errorf("repository is not closed")
	}

	// Keep the closed history but mark as reopened
	config.State = RepositoryState{
		IsClosed:     false,
		LastModified: time.Now(),
	}
	return nil
}

// IsRepositoryClosed checks if a repository is closed
func IsRepositoryClosed(config *TrackingConfig) bool {
	return config.State.IsClosed
}

// RemoveExcludePatterns removes specified exclude patterns from tracked paths
func RemoveExcludePatterns(config *TrackingConfig, paths []string, patterns []string) error {
	// Convert all paths to absolute paths
	absPaths := make([]string, len(paths))
	for i, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
		}
		absPaths[i] = absPath
	}

	// Track if we found and modified any paths
	foundAny := false

	// For each tracked path
	for i, trackedPath := range config.Paths {
		// Check if this path is in our list
		for _, absPath := range absPaths {
			if trackedPath.Path == absPath {
				// Only process directories
				if !trackedPath.IsDir {
					return fmt.Errorf("exclude patterns can only be modified for directories, but %s is a file", absPath)
				}

				// Remove specified patterns
				if len(trackedPath.Excludes) > 0 {
					newExcludes := make([]string, 0, len(trackedPath.Excludes))
					for _, exclude := range trackedPath.Excludes {
						// Keep exclude if it's not in the patterns to remove
						shouldKeep := true
						for _, pattern := range patterns {
							if exclude == pattern {
								shouldKeep = false
								break
							}
						}
						if shouldKeep {
							newExcludes = append(newExcludes, exclude)
						}
					}
					config.Paths[i].Excludes = newExcludes
					foundAny = true
				}
				break
			}
		}
	}

	if !foundAny {
		return fmt.Errorf("none of the specified paths are currently tracked")
	}

	return nil
}

// AddExcludePatterns adds exclude patterns to existing tracked paths
func AddExcludePatterns(config *TrackingConfig, paths []string, patterns []string) error {
	// Convert all paths to absolute paths
	absPaths := make([]string, len(paths))
	for i, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
		}
		absPaths[i] = absPath
	}

	// Track if we found and modified any paths
	foundAny := false

	// For each tracked path
	for i, trackedPath := range config.Paths {
		// Check if this path is in our list
		for _, absPath := range absPaths {
			if trackedPath.Path == absPath {
				// Only process directories
				if !trackedPath.IsDir {
					return fmt.Errorf("exclude patterns can only be added to directories, but %s is a file", absPath)
				}

				// Initialize excludes if nil
				if config.Paths[i].Excludes == nil {
					config.Paths[i].Excludes = make([]string, 0)
				}

				// Add new patterns, avoiding duplicates
				for _, pattern := range patterns {
					// Check if pattern already exists
					exists := false
					for _, existing := range config.Paths[i].Excludes {
						if existing == pattern {
							exists = true
							break
						}
					}
					if !exists {
						config.Paths[i].Excludes = append(config.Paths[i].Excludes, pattern)
					}
				}
				foundAny = true
				break
			}
		}
	}

	if !foundAny {
		return fmt.Errorf("none of the specified paths are currently tracked")
	}

	return nil
}

// IsPathInRepository checks if a path is within the repository root
func IsPathInRepository(path, repoRoot string) (bool, error) {
	// Convert both paths to absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("failed to get absolute path: %w", err)
	}
	absRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return false, fmt.Errorf("failed to get absolute repository root: %w", err)
	}

	// Check if path is within repository
	relPath, err := filepath.Rel(absRepoRoot, absPath)
	if err != nil {
		return false, fmt.Errorf("failed to get relative path: %w", err)
	}

	// If path starts with "..", it's outside the repository
	return !strings.HasPrefix(relPath, ".."), nil
}
