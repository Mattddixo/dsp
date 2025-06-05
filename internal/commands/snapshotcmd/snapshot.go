package snapshotcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/T-I-M/dsp/config"
	"github.com/T-I-M/dsp/internal/repo"
	"github.com/T-I-M/dsp/internal/snapshot"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "snapshot",
	Usage: "Create a new snapshot of tracked files",
	Description: `Create a new snapshot of all tracked files in the repository.
This command captures the current state of all tracked files and stores it
in the repository's snapshots directory. The snapshot can be used to track
changes over time and create bundles for synchronization.

Examples:
  # Create a snapshot with a message
  dsp snapshot -m "Initial snapshot"

  # Create a snapshot in a specific repository
  dsp snapshot -m "Update" --repo /path/to/repo

Note: This command works from any directory within the repository. If you
have multiple repositories, use --repo to specify which one to use.`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "message",
			Aliases:  []string{"m"},
			Usage:    "Message describing the snapshot",
			Required: true,
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

		// Load repository configuration
		repoConfig, err := config.NewWithRepo(currentRepo.Path, currentRepo.DSPDir)
		if err != nil {
			return fmt.Errorf("failed to load repository configuration: %w", err)
		}

		// Load tracking configuration
		trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
		if err != nil {
			return fmt.Errorf("failed to load tracking configuration: %w", err)
		}

		if len(trackingConfig.Paths) == 0 {
			return fmt.Errorf("no paths are being tracked in repository '%s'", currentRepo.Name)
		}

		// Create snapshot directory using repository's snapshots path
		timestamp := time.Now().Format("20060102-150405")
		snapshotDir := filepath.Join(dspDir, "snapshots", timestamp)
		if err := os.MkdirAll(snapshotDir, 0755); err != nil {
			return fmt.Errorf("failed to create snapshot directory: %w", err)
		}

		// Create snapshot with repository configuration
		snap, err := snapshot.CreateSnapshot(trackingConfig.Paths, os.Getenv("USERNAME"), c.String("message"), repoConfig)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		// Save snapshot
		if err := snap.Save(filepath.Join(snapshotDir, "snapshot.json")); err != nil {
			return fmt.Errorf("failed to save snapshot: %w", err)
		}

		fmt.Printf("Created snapshot in repository '%s': %s\n", currentRepo.Name, timestamp)
		fmt.Printf("Message: %s\n", snap.Message)
		fmt.Printf("Files: %d\n", len(snap.Files))
		fmt.Printf("Total size: %d bytes\n", snap.Stats.TotalSize)
		fmt.Printf("Hash algorithm: %s\n", repoConfig.HashAlgorithm)

		return nil
	},
}
