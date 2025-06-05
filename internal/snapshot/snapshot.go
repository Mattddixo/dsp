package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/T-I-M/dsp/config"
	"github.com/T-I-M/dsp/pkg/utils"
)

// Snapshot represents a snapshot of tracked files
type Snapshot struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Files     []File    `json:"files"`
	User      string    `json:"user"`
	Message   string    `json:"message"`
	Stats     Stats     `json:"stats"`
}

// Stats represents statistics about the snapshot
type Stats struct {
	TotalFiles     int   `json:"total_files"`
	TotalSize      int64 `json:"total_size"`
	SymlinkCount   int   `json:"symlink_count"`
	RegularFiles   int   `json:"regular_files"`
	ExcludedFiles  int   `json:"excluded_files"`
	ProcessingTime int64 `json:"processing_time_ms"`
}

// File represents a file in the snapshot
type File struct {
	Path          string    `json:"path"`
	Hash          string    `json:"hash"`
	Size          int64     `json:"size"`
	ModifiedTime  time.Time `json:"modified_time"`
	IsSymlink     bool      `json:"is_symlink"`
	SymlinkTarget string    `json:"symlink_target,omitempty"`
	ChangeType    string    `json:"change_type,omitempty"` // "added", "modified", "unchanged"
}

// CreateSnapshot creates a new snapshot of tracked files
func CreateSnapshot(trackedPaths []TrackedPath, user, message string, cfg *config.Config) (*Snapshot, error) {
	startTime := time.Now()

	snapshot := &Snapshot{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		User:      user,
		Message:   message,
		Files:     make([]File, 0),
		Stats:     Stats{},
	}

	// Process each tracked path
	for _, path := range trackedPaths {
		if err := processPath(path, snapshot, cfg); err != nil {
			return nil, fmt.Errorf("failed to process path %s: %w", path.Path, err)
		}
	}

	// Calculate processing time
	snapshot.Stats.ProcessingTime = time.Since(startTime).Milliseconds()

	return snapshot, nil
}

// processPath processes a path and adds its files to the snapshot
func processPath(path TrackedPath, snapshot *Snapshot, cfg *config.Config) error {
	// Check if path exists
	info, err := os.Stat(path.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// Skip non-existent paths
			return nil
		}
		return fmt.Errorf("failed to stat path: %w", err)
	}

	if !info.IsDir() {
		// Process single file
		hash, err := utils.HashFile(path.Path, cfg.HashAlgorithm)
		if err != nil {
			return fmt.Errorf("failed to hash file: %w", err)
		}

		// Get symlink info if it's a symlink
		var isSymlink bool
		var symlinkTarget string
		if info.Mode()&os.ModeSymlink != 0 {
			isSymlink = true
			symlinkTarget, err = os.Readlink(path.Path)
			if err != nil {
				return fmt.Errorf("failed to read symlink: %w", err)
			}
		}

		// Add file to snapshot
		snapshot.Files = append(snapshot.Files, File{
			Path:          path.Path,
			Hash:          hash,
			Size:          info.Size(),
			ModifiedTime:  info.ModTime(),
			IsSymlink:     isSymlink,
			SymlinkTarget: symlinkTarget,
		})

		// Update stats
		snapshot.Stats.TotalFiles++
		snapshot.Stats.TotalSize += info.Size()
		if isSymlink {
			snapshot.Stats.SymlinkCount++
		} else {
			snapshot.Stats.RegularFiles++
		}
		return nil
	}

	// Process directory
	return filepath.Walk(path.Path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if filePath == path.Path {
			return nil
		}

		// Check if file should be excluded
		relPath, err := filepath.Rel(path.Path, filePath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Check against exclude patterns
		for _, pattern := range path.Excludes {
			matched, err := filepath.Match(pattern, relPath)
			if err != nil {
				return fmt.Errorf("invalid exclude pattern %s: %w", pattern, err)
			}
			if matched {
				snapshot.Stats.ExcludedFiles++
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip if it's a directory (we'll process its contents)
		if info.IsDir() {
			return nil
		}

		// Process file using repository's hash algorithm
		hash, err := utils.HashFile(filePath, cfg.HashAlgorithm)
		if err != nil {
			return fmt.Errorf("failed to hash file: %w", err)
		}

		// Get symlink info if it's a symlink
		var isSymlink bool
		var symlinkTarget string
		if info.Mode()&os.ModeSymlink != 0 {
			isSymlink = true
			symlinkTarget, err = os.Readlink(filePath)
			if err != nil {
				return fmt.Errorf("failed to read symlink: %w", err)
			}
		}

		// Add file to snapshot
		snapshot.Files = append(snapshot.Files, File{
			Path:          filePath,
			Hash:          hash,
			Size:          info.Size(),
			ModifiedTime:  info.ModTime(),
			IsSymlink:     isSymlink,
			SymlinkTarget: symlinkTarget,
		})

		// Update stats
		snapshot.Stats.TotalFiles++
		snapshot.Stats.TotalSize += info.Size()
		if isSymlink {
			snapshot.Stats.SymlinkCount++
		} else {
			snapshot.Stats.RegularFiles++
		}

		return nil
	})
}

// Save saves the snapshot to a file
func (s *Snapshot) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return nil
}

// Load loads a snapshot from a file
func Load(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &snapshot, nil
}
