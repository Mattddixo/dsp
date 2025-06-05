package trackcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/T-I-M/dsp/internal/commands/flags"
	"github.com/T-I-M/dsp/internal/repo"
	"github.com/T-I-M/dsp/internal/snapshot"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "track",
	Usage: "Track files or directories [--path PATH [PATH...]] [--exclude PATTERN...]",
	Description: `Add files or directories to the tracking configuration.
This command adds the specified paths to the repository's tracking configuration.
The paths can be relative to the current directory or absolute paths.

Usage Examples:
  # Track a file
  dsp track --path file.txt

  # Track a directory
  dsp track --path directory/

  # Track multiple paths with a single --path flag
  dsp track --path dir1/ dir2/ dir3/
  dsp track -p file1.txt file2.txt dir/

  # Track paths and exclude certain files/patterns
  dsp track --path my_project/ --exclude "*.log" --exclude "temp/*"
  dsp track --exclude "*.tmp" --path src/ test/ --exclude "test/*"

  # Track multiple paths with excludes
  dsp track --path dir1/ dir2/ --exclude "*.log" --exclude "temp/*"

  # Track a path in a specific repository
  dsp track --repo /path/to/repo --path file.txt

  # List currently tracked paths
  dsp track --list

  # List with detailed information
  dsp track --list --verbose

Exclude Patterns:
  Use --exclude to specify patterns for files/directories to ignore within tracked directories.
  Multiple --exclude flags can be used to specify different patterns.
  
  Pattern Syntax (using Go's filepath.Match):
    * matches any sequence of non-separator characters
    ? matches any single non-separator character
    [sequence] matches any single character in sequence
    [!sequence] matches any single character not in sequence

  Common Examples:
    *.log           - ignore all .log files
    temp/*         - ignore everything in temp directories
    node_modules   - ignore node_modules directories
    *.{log,tmp}    - ignore files ending in .log or .tmp
    [Tt]emp/*      - ignore everything in Temp or temp directories
    *.bak          - ignore backup files
    .git/*         - ignore git directory contents
    **/cache/*     - ignore cache directories at any depth

  Note: Exclude patterns only work for directories, not individual files.
        Patterns are relative to each tracked directory.
        For example, if tracking "dir1/" and "dir2/" with --exclude "*.log",
        it will ignore all .log files within both dir1/ and dir2/.`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "repo",
			Aliases: []string{"r"},
			Usage:   "Path to the repository (default: nearest repository)",
		},
		&cli.BoolFlag{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "List currently tracked paths",
		},
		&cli.StringSliceFlag{
			Name:    "path",
			Aliases: []string{"p"},
			Usage:   "Path(s) to track (can specify multiple paths)",
		},
		&cli.StringSliceFlag{
			Name:    "exclude",
			Aliases: []string{"e"},
			Usage:   "Pattern to exclude within tracked directories",
		},
		flags.VerboseFlag,
		flags.QuietFlag,
	},
	Action: func(c *cli.Context) error {
		// Get exclude patterns if any
		excludes := c.StringSlice("exclude")

		// Get paths from the --path flag
		paths := c.StringSlice("path")

		// If no paths specified and not listing, show usage
		if len(paths) == 0 && !c.Bool("list") {
			return fmt.Errorf("no paths specified. Usage: dsp track --path PATH [--path PATH...] [--exclude PATTERN...]")
		}

		// Create repository manager
		manager, err := repo.NewManager()
		if err != nil {
			return fmt.Errorf("failed to create repository manager: %w", err)
		}

		// Get current repository context
		currentRepo, err := manager.GetCurrentRepo(c.String("repo"))
		if err != nil {
			return fmt.Errorf("failed to get repository context: %w", err)
		}

		// Get DSP directory path from repository config
		dspDir := filepath.Join(currentRepo.Path, currentRepo.DSPDir)

		// Load tracking configuration
		trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
		if err != nil {
			return fmt.Errorf("failed to load tracking configuration: %w", err)
		}

		// Check if repository is closed
		if snapshot.IsRepositoryClosed(trackingConfig) {
			return fmt.Errorf("repository is closed. Please re-add it using 'dsp repo add' before tracking files")
		}

		// Handle list flag
		if c.Bool("list") {
			if len(trackingConfig.Paths) == 0 {
				if !c.Bool("quiet") {
					fmt.Printf("No files or directories are currently tracked in repository: %s\n", currentRepo.Name)
				}
				return nil
			}

			// Print header
			if !c.Bool("quiet") {
				fmt.Printf("Found %d tracked paths in repository '%s':\n\n", len(trackingConfig.Paths), currentRepo.Name)
			}

			// Print each tracked path
			for _, path := range trackingConfig.Paths {
				// Get current file info
				info, err := os.Stat(path.Path)
				if err != nil {
					fmt.Printf("Warning: Could not access %s: %v\n", path.Path, err)
					continue
				}

				// Print basic info using absolute path
				fmt.Printf("%s (%s)\n", path.Path, formatType(info.IsDir()))
				if len(path.Excludes) > 0 {
					fmt.Printf("  Excludes: %s\n", strings.Join(path.Excludes, ", "))
				}

				if c.Bool("verbose") {
					// Print detailed info
					fmt.Printf("  Last Modified: %s\n", formatTime(info.ModTime()))
					fmt.Println()
				}
			}
			return nil
		}

		// If we have exclude patterns, try to add them to existing paths first
		if len(excludes) > 0 {
			// Validate and normalize exclude patterns
			normalizedExcludes := make([]string, 0, len(excludes))
			for _, pattern := range excludes {
				// Remove leading slashes to ensure patterns are relative
				pattern = strings.TrimLeft(pattern, "/\\")

				// Validate pattern format
				if strings.Contains(pattern, "\\") {
					return fmt.Errorf("invalid exclude pattern '%s': use forward slashes (/) instead of backslashes (\\)", pattern)
				}

				// Check for absolute paths
				if filepath.IsAbs(pattern) {
					return fmt.Errorf("invalid exclude pattern '%s': patterns must be relative to the tracked directory", pattern)
				}

				normalizedExcludes = append(normalizedExcludes, pattern)
			}

			// Try to add excludes to existing paths
			err := snapshot.AddExcludePatterns(trackingConfig, paths, normalizedExcludes)
			if err == nil {
				// Successfully added excludes to existing paths
				if err := snapshot.SaveTrackingConfig(dspDir, trackingConfig); err != nil {
					return fmt.Errorf("failed to save tracking configuration: %w", err)
				}

				if !c.Bool("quiet") {
					fmt.Printf("Added exclude patterns to tracked directories in repository '%s':\n", currentRepo.Name)
					for _, path := range paths {
						fmt.Printf("  - %s\n", path)
					}
					fmt.Printf("Added patterns (relative to tracked directory):\n")
					for _, pattern := range normalizedExcludes {
						fmt.Printf("  - %s\n", pattern)
					}
				}
				return nil
			} else if err.Error() != "none of the specified paths are currently tracked" {
				// If there was an error other than "paths not tracked", return it
				return fmt.Errorf("failed to add exclude patterns: %w", err)
			}
			// If paths aren't tracked, continue to try adding them as new paths
		}

		// Process each path as new paths to track
		addedPaths := 0
		for _, path := range paths {
			// Convert to absolute path
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
			}

			// Check if path exists
			info, err := os.Stat(absPath)
			if os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", absPath)
			}
			if err != nil {
				return fmt.Errorf("failed to get path info: %w", err)
			}

			// Validate path is within repository
			isInRepo, err := snapshot.IsPathInRepository(absPath, currentRepo.Path)
			if err != nil {
				return fmt.Errorf("failed to validate path: %w", err)
			}
			if !isInRepo {
				return fmt.Errorf("path %s is outside repository root %s. Please move the file/directory into the repository or create a symlink", absPath, currentRepo.Path)
			}

			// Create tracked path with excludes if specified
			trackedPath := snapshot.TrackedPath{
				Path:  absPath,
				IsDir: info.IsDir(),
			}
			if len(excludes) > 0 {
				if !info.IsDir() {
					return fmt.Errorf("exclude patterns can only be used with directories, but %s is a file", path)
				}

				// Normalize exclude patterns for new paths too
				normalizedExcludes := make([]string, 0, len(excludes))
				for _, pattern := range excludes {
					// Remove leading slashes to ensure patterns are relative
					pattern = strings.TrimLeft(pattern, "/\\")

					// Validate pattern format
					if strings.Contains(pattern, "\\") {
						return fmt.Errorf("invalid exclude pattern '%s': use forward slashes (/) instead of backslashes (\\)", pattern)
					}

					// Check for absolute paths
					if filepath.IsAbs(pattern) {
						return fmt.Errorf("invalid exclude pattern '%s': patterns must be relative to the tracked directory", pattern)
					}

					normalizedExcludes = append(normalizedExcludes, pattern)
				}

				trackedPath.Excludes = normalizedExcludes
			}

			// Add to tracking config
			if err := snapshot.AddTrackedPathWithExcludes(trackingConfig, trackedPath); err != nil {
				if err.Error() == "path is already tracked" {
					if !c.Bool("quiet") {
						fmt.Printf("Path already tracked: %s\n", path)
					}
					continue
				}
				return fmt.Errorf("failed to add path to tracking: %w", err)
			}

			addedPaths++
			if !c.Bool("quiet") {
				if info.IsDir() {
					fmt.Printf("Added directory to tracking: %s\n", path)
					if len(excludes) > 0 {
						fmt.Printf("  Excluding patterns:\n")
						for _, pattern := range excludes {
							fmt.Printf("    - %s\n", pattern)
						}
					}
				} else {
					fmt.Printf("Added file to tracking: %s\n", path)
				}
			}
		}

		// Save tracking configuration
		if err := snapshot.SaveTrackingConfig(dspDir, trackingConfig); err != nil {
			return fmt.Errorf("failed to save tracking configuration: %w", err)
		}

		// Update success message to include repository name and summary
		if !c.Bool("quiet") {
			if addedPaths > 0 {
				fmt.Printf("\nTracking summary for repository '%s':\n", currentRepo.Name)
				fmt.Printf("  Added %d paths to tracking\n", addedPaths)
				if len(excludes) > 0 {
					fmt.Printf("  Configured %d exclude patterns\n", len(excludes))
				}
			} else if len(excludes) > 0 {
				fmt.Printf("\nNo new paths were added to tracking in repository '%s'\n", currentRepo.Name)
			}
		}
		return nil
	},
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func formatType(isDir bool) string {
	if isDir {
		return "Directory"
	}
	return "File"
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	return t.Format("2006-01-02 15:04:05")
}
