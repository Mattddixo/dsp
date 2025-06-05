package diffcmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/Mattddixo/dsp/internal/commands/common"
	"github.com/Mattddixo/dsp/internal/commands/flags"
	"github.com/Mattddixo/dsp/internal/repo"
	"github.com/Mattddixo/dsp/internal/snapshot"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "diff",
	Usage: "Compare snapshots or current state",
	Description: `Compare snapshots or current state to see changes.
This command can be used in three ways:
  1. Compare latest snapshot with current state: dsp diff
  2. Compare specific snapshot with current state: dsp diff <snapshot-id>
  3. Compare two snapshots: dsp diff <snapshot-id1> <snapshot-id2>

Examples:
  # Compare latest snapshot with current state
  dsp diff

  # Compare specific snapshot with current state
  dsp diff 20240101-120000

  # Compare two snapshots
  dsp diff 20240101-120000 20240102-120000

  # Show only summary of changes
  dsp diff --summary

  # Filter changes to specific path
  dsp diff --path "src/"

  # Show changes in a specific repository
  dsp diff --repo /path/to/repo`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "repo",
			Aliases: []string{"r"},
			Usage:   "Path to the repository (default: nearest repository)",
		},
		&cli.StringFlag{
			Name:    "path",
			Aliases: []string{"p"},
			Usage:   "Filter changes to specific path",
		},
		&cli.BoolFlag{
			Name:    "summary",
			Aliases: []string{"s"},
			Usage:   "Show only summary of changes",
		},
		flags.VerboseFlag,
		flags.QuietFlag,
	},
	Action: func(c *cli.Context) error {
		pathFilter := c.String("path")
		summaryOnly := c.Bool("summary")

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

		// Get config
		cfg, err := common.GetConfig(c)
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		var snap1, snap2 *snapshot.Snapshot

		// Handle different snapshot comparison modes
		if c.NArg() == 0 {
			// Compare latest snapshot with current state
			snap1, err = getLatestSnapshot(dspDir)
			if err != nil {
				return fmt.Errorf("failed to get latest snapshot: %w", err)
			}
			// Create current state snapshot
			currentUser, err := user.Current()
			if err != nil {
				return fmt.Errorf("failed to get current user: %w", err)
			}
			trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
			if err != nil {
				return fmt.Errorf("failed to load tracking config: %w", err)
			}
			snap2, err = snapshot.CreateSnapshot(trackingConfig.Paths, currentUser.Username, "", cfg)
			if err != nil {
				return fmt.Errorf("failed to create current state snapshot: %w", err)
			}
		} else if c.NArg() == 1 {
			// Compare specified snapshot with current state
			snap1, err = loadSnapshot(dspDir, c.Args().Get(0))
			if err != nil {
				return fmt.Errorf("failed to load snapshot: %w", err)
			}
			// Create current state snapshot
			currentUser, err := user.Current()
			if err != nil {
				return fmt.Errorf("failed to get current user: %w", err)
			}
			trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
			if err != nil {
				return fmt.Errorf("failed to load tracking config: %w", err)
			}
			snap2, err = snapshot.CreateSnapshot(trackingConfig.Paths, currentUser.Username, "", cfg)
			if err != nil {
				return fmt.Errorf("failed to create current state snapshot: %w", err)
			}
		} else if c.NArg() == 2 {
			// Compare two specified snapshots
			snap1, err = loadSnapshot(dspDir, c.Args().Get(0))
			if err != nil {
				return fmt.Errorf("failed to load first snapshot: %w", err)
			}
			snap2, err = loadSnapshot(dspDir, c.Args().Get(1))
			if err != nil {
				return fmt.Errorf("failed to load second snapshot: %w", err)
			}
		}

		// Compare snapshots
		diff, err := calculateDiff(snap1, snap2, pathFilter)
		if err != nil {
			return fmt.Errorf("failed to calculate differences: %w", err)
		}

		// Print results
		if !c.Bool("quiet") {
			if summaryOnly {
				displayDiffSummary(diff)
			} else {
				displayDiff(diff, c.Bool("verbose"))
			}
		}

		return nil
	},
}

// getLatestSnapshot returns the most recent snapshot
func getLatestSnapshot(dspDir string) (*snapshot.Snapshot, error) {
	snapshotsDir := filepath.Join(dspDir, "snapshots")
	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshots directory: %w", err)
	}

	var latestSnapshot *snapshot.Snapshot
	var latestTime int64

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		snapshotPath := filepath.Join(snapshotsDir, entry.Name(), "snapshot.json")
		snap, err := snapshot.Load(snapshotPath)
		if err != nil {
			continue // Skip invalid snapshots
		}
		if snap.Timestamp.UnixNano() > latestTime {
			latestTime = snap.Timestamp.UnixNano()
			latestSnapshot = snap
		}
	}

	if latestSnapshot == nil {
		return nil, fmt.Errorf("no snapshots found")
	}

	return latestSnapshot, nil
}

// loadSnapshot loads a snapshot by ID
func loadSnapshot(dspDir, snapshotID string) (*snapshot.Snapshot, error) {
	snapshotPath := filepath.Join(dspDir, "snapshots", snapshotID, "snapshot.json")
	return snapshot.Load(snapshotPath)
}

// Diff represents the differences between two snapshots
type Diff struct {
	Added     []snapshot.File
	Modified  []snapshot.File
	Deleted   []snapshot.File
	Unchanged []snapshot.File
}

// calculateDiff calculates the differences between two snapshots
func calculateDiff(snap1, snap2 *snapshot.Snapshot, pathFilter string) (*Diff, error) {
	diff := &Diff{
		Added:     make([]snapshot.File, 0),
		Modified:  make([]snapshot.File, 0),
		Deleted:   make([]snapshot.File, 0),
		Unchanged: make([]snapshot.File, 0),
	}

	// Create maps for faster lookup
	snap1Files := make(map[string]snapshot.File)
	snap2Files := make(map[string]snapshot.File)

	for _, f := range snap1.Files {
		snap1Files[f.Path] = f
	}
	for _, f := range snap2.Files {
		snap2Files[f.Path] = f
	}

	// Find added and modified files
	for path, file2 := range snap2Files {
		if pathFilter != "" && path != pathFilter {
			continue
		}
		if file1, exists := snap1Files[path]; !exists {
			diff.Added = append(diff.Added, file2)
		} else if file1.Hash != file2.Hash {
			diff.Modified = append(diff.Modified, file2)
		} else {
			diff.Unchanged = append(diff.Unchanged, file2)
		}
	}

	// Find deleted files
	for path, file1 := range snap1Files {
		if pathFilter != "" && path != pathFilter {
			continue
		}
		if _, exists := snap2Files[path]; !exists {
			diff.Deleted = append(diff.Deleted, file1)
		}
	}

	return diff, nil
}

// displayDiff displays the differences between snapshots
func displayDiff(diff *Diff, verbose bool) {
	if len(diff.Added) > 0 {
		fmt.Println("\nAdded files:")
		for _, f := range diff.Added {
			fmt.Printf("  + %s\n", f.Path)
			if verbose {
				fmt.Printf("    Size: %d bytes\n", f.Size)
				fmt.Printf("    Hash: %s\n", f.Hash)
			}
		}
	}

	if len(diff.Modified) > 0 {
		fmt.Println("\nModified files:")
		for _, f := range diff.Modified {
			fmt.Printf("  M %s\n", f.Path)
			if verbose {
				fmt.Printf("    Size: %d bytes\n", f.Size)
				fmt.Printf("    Hash: %s\n", f.Hash)
			}
		}
	}

	if len(diff.Deleted) > 0 {
		fmt.Println("\nDeleted files:")
		for _, f := range diff.Deleted {
			fmt.Printf("  - %s\n", f.Path)
			if verbose {
				fmt.Printf("    Size: %d bytes\n", f.Size)
				fmt.Printf("    Hash: %s\n", f.Hash)
			}
		}
	}

	if len(diff.Added) == 0 && len(diff.Modified) == 0 && len(diff.Deleted) == 0 {
		fmt.Println("No changes found")
	}
}

// displayDiffSummary displays a summary of the differences
func displayDiffSummary(diff *Diff) {
	totalChanges := len(diff.Added) + len(diff.Modified) + len(diff.Deleted)
	if totalChanges == 0 {
		fmt.Println("No changes found")
		return
	}

	fmt.Printf("\nSummary of changes:\n")
	fmt.Printf("  Added:    %d files\n", len(diff.Added))
	fmt.Printf("  Modified: %d files\n", len(diff.Modified))
	fmt.Printf("  Deleted:  %d files\n", len(diff.Deleted))
	fmt.Printf("  Total:    %d changes\n", totalChanges)
}
