package untrackcmd

import (
	"fmt"
	"path/filepath"

	"github.com/T-I-M/dsp/internal/commands/flags"
	"github.com/T-I-M/dsp/internal/repo"
	"github.com/T-I-M/dsp/internal/snapshot"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "untrack",
	Usage: "Remove paths from tracking or remove exclude patterns",
	Description: `Remove paths from tracking or remove exclude patterns from tracked directories.
This command can be used in two ways:
1. Remove paths from tracking (they will no longer be included in snapshots)
2. Remove exclude patterns from tracked directories

Usage Examples:
  # Remove a single path from tracking
  dsp untrack --path file.txt
  dsp untrack -p directory/

  # Remove multiple paths from tracking
  dsp untrack --path dir1/ --path dir2/
  dsp untrack -p file1.txt -p file2.txt -p dir/

  # Remove exclude patterns from tracked directories
  dsp untrack --path dir1/ --path dir2/ --exclude "*.log" --exclude "temp/*"
  dsp untrack -p dir/ --exclude "*.tmp"

  # Remove paths in a specific repository
  dsp untrack --repo /path/to/repo --path file.txt`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "repo",
			Aliases: []string{"r"},
			Usage:   "Path to the repository (default: working repository or nearest repository)",
		},
		&cli.StringSliceFlag{
			Name:    "path",
			Aliases: []string{"p"},
			Usage:   "Path(s) to untrack (can specify multiple paths)",
		},
		&cli.StringSliceFlag{
			Name:    "exclude",
			Aliases: []string{"e"},
			Usage:   "Pattern(s) to remove from tracked directories",
		},
		flags.VerboseFlag,
		flags.QuietFlag,
	},
	Action: func(c *cli.Context) error {
		// Get paths and exclude patterns
		paths := c.StringSlice("path")
		excludes := c.StringSlice("exclude")

		// Validate input
		if len(paths) == 0 {
			return fmt.Errorf("no paths specified. Usage: dsp untrack --path PATH [--path PATH...] [--exclude PATTERN...]")
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

		// Load tracking config
		trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
		if err != nil {
			return fmt.Errorf("failed to load tracking config: %w", err)
		}

		// If exclude patterns are specified, remove them from the paths
		if len(excludes) > 0 {
			if err := snapshot.RemoveExcludePatterns(trackingConfig, paths, excludes); err != nil {
				return fmt.Errorf("failed to remove exclude patterns: %w", err)
			}

			// Save the updated config
			if err := snapshot.SaveTrackingConfig(dspDir, trackingConfig); err != nil {
				return fmt.Errorf("failed to save tracking config: %w", err)
			}

			if !c.Bool("quiet") {
				fmt.Printf("Removed exclude patterns from tracked directories in repository '%s':\n", currentRepo.Name)
				for _, path := range paths {
					fmt.Printf("  - %s\n", path)
				}
				fmt.Printf("Removed patterns:\n")
				for _, pattern := range excludes {
					fmt.Printf("  - %s\n", pattern)
				}
			}
			return nil
		}

		// Otherwise, remove the paths from tracking
		removedPaths := 0
		for _, path := range paths {
			if err := snapshot.RemoveTrackedPath(trackingConfig, path); err != nil {
				if err.Error() == "path is not tracked" {
					if !c.Bool("quiet") {
						fmt.Printf("Path is not tracked: %s\n", path)
					}
					continue
				}
				return fmt.Errorf("failed to remove path from tracking: %w", err)
			}
			removedPaths++
		}

		// Save the updated config
		if err := snapshot.SaveTrackingConfig(dspDir, trackingConfig); err != nil {
			return fmt.Errorf("failed to save tracking config: %w", err)
		}

		if !c.Bool("quiet") {
			if removedPaths > 0 {
				fmt.Printf("Successfully removed %d paths from tracking in repository '%s':\n", removedPaths, currentRepo.Name)
				for _, path := range paths {
					fmt.Printf("  - %s\n", path)
				}
			} else {
				fmt.Printf("No paths were removed from tracking in repository '%s'\n", currentRepo.Name)
			}
		}

		return nil
	},
}
