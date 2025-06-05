package bundlecmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Mattddixo/dsp/internal/bundle"
	"github.com/Mattddixo/dsp/internal/repo"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "bundle",
	Usage: "Create a bundle of changes between snapshots",
	Description: `Create a bundle of changes between two snapshots.
The bundle contains the changes between the source and target snapshots.

Examples:
  # Create a bundle between the latest and previous snapshots
  dsp bundle

  # Create a bundle between specific snapshots
  dsp bundle -s 20240101-120000 -t 20240102-150000`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "source",
			Aliases: []string{"s"},
			Usage:   "Source snapshot ID (default: previous snapshot)",
		},
		&cli.StringFlag{
			Name:    "target",
			Aliases: []string{"t"},
			Usage:   "Target snapshot ID (default: latest snapshot)",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output bundle file path (default: bundles/<timestamp>.json)",
		},
		&cli.StringFlag{
			Name:    "description",
			Aliases: []string{"d"},
			Usage:   "Description of the bundle",
		},
		&cli.StringFlag{
			Name:    "repo",
			Aliases: []string{"r"},
			Usage:   "Path to the repository (default: nearest repository)",
		},
	},
	Action: func(c *cli.Context) error {
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

		// Get DSP directory path from repository
		dspDir := currentRepo.GetDSPDir()

		// Get source and target snapshots
		sourceSnapshot, targetSnapshot, err := getSnapshots(dspDir, c.String("source"), c.String("target"))
		if err != nil {
			return fmt.Errorf("failed to get snapshots: %w", err)
		}

		// Create bundle
		bundle, err := bundle.New(sourceSnapshot, targetSnapshot)
		if err != nil {
			return fmt.Errorf("failed to create bundle: %w", err)
		}

		// Set bundle description if provided
		if desc := c.String("description"); desc != "" {
			bundle.Description = desc
		}

		// Determine output path
		outputPath := c.String("output")
		if outputPath == "" {
			// Create bundles directory
			bundlesDir := filepath.Join(dspDir, "bundles")
			if err := os.MkdirAll(bundlesDir, 0755); err != nil {
				return fmt.Errorf("failed to create bundles directory: %w", err)
			}

			// Use timestamp-based filename
			outputPath = filepath.Join(bundlesDir, fmt.Sprintf("%s.json", bundle.ID))
		}

		// Save bundle
		if err := bundle.Save(outputPath); err != nil {
			return fmt.Errorf("failed to save bundle: %w", err)
		}

		// Print success message
		fmt.Printf("Created bundle: %s\n", outputPath)
		fmt.Printf("Source snapshot: %s\n", filepath.Base(sourceSnapshot))
		fmt.Printf("Target snapshot: %s\n", filepath.Base(targetSnapshot))
		fmt.Printf("Changes: %d\n", len(bundle.Changes))

		return nil
	},
}

// getSnapshots returns the source and target snapshot paths
func getSnapshots(dspDir, sourceID, targetID string) (string, string, error) {
	snapshotsDir := filepath.Join(dspDir, "snapshots")

	// Get target snapshot
	var targetSnapshot string
	if targetID != "" {
		targetSnapshot = filepath.Join(snapshotsDir, targetID, "snapshot.json")
		if _, err := os.Stat(targetSnapshot); err != nil {
			return "", "", fmt.Errorf("target snapshot not found: %w", err)
		}
	} else {
		// Find latest snapshot
		entries, err := os.ReadDir(snapshotsDir)
		if err != nil {
			return "", "", fmt.Errorf("failed to read snapshots directory: %w", err)
		}

		var latestTime int64
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			snapshotPath := filepath.Join(snapshotsDir, entry.Name(), "snapshot.json")
			if info, err := os.Stat(snapshotPath); err == nil {
				if t := info.ModTime().UnixNano(); t > latestTime {
					latestTime = t
					targetSnapshot = snapshotPath
				}
			}
		}

		if targetSnapshot == "" {
			return "", "", fmt.Errorf("no snapshots found")
		}
	}

	// Get source snapshot
	var sourceSnapshot string
	if sourceID != "" {
		sourceSnapshot = filepath.Join(snapshotsDir, sourceID, "snapshot.json")
		if _, err := os.Stat(sourceSnapshot); err != nil {
			return "", "", fmt.Errorf("source snapshot not found: %w", err)
		}
	} else {
		// Find previous snapshot
		entries, err := os.ReadDir(snapshotsDir)
		if err != nil {
			return "", "", fmt.Errorf("failed to read snapshots directory: %w", err)
		}

		var prevTime int64
		targetTimeStr := filepath.Base(filepath.Dir(targetSnapshot))
		targetTime, err := strconv.ParseInt(targetTimeStr, 10, 64)
		if err != nil {
			return "", "", fmt.Errorf("invalid target snapshot timestamp: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			snapshotPath := filepath.Join(snapshotsDir, entry.Name(), "snapshot.json")
			if info, err := os.Stat(snapshotPath); err == nil {
				if t := info.ModTime().UnixNano(); t < targetTime && t > prevTime {
					prevTime = t
					sourceSnapshot = snapshotPath
				}
			}
		}

		if sourceSnapshot == "" {
			return "", "", fmt.Errorf("no previous snapshot found")
		}
	}

	return sourceSnapshot, targetSnapshot, nil
}
