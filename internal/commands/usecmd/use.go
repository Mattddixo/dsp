package usecmd

import (
	"fmt"

	"github.com/T-I-M/dsp/internal/repo"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "use",
	Usage: "Set the working repository",
	Description: `Set the working repository for DSP commands.
This command sets the repository that will be used by default for all DSP commands
unless explicitly overridden with the --repo flag.

The repository can be specified by either its name or its path. If a name is used,
it must match exactly with a repository name from 'dsp repo list'.

Examples:
  # Set working repository by name
  dsp use "field-work"

  # Set working repository by path
  dsp use /path/to/repo

  # Show current working repository
  dsp use --current

  # Clear working repository
  dsp use --unset

  # List available repositories
  dsp repo list`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "current",
			Aliases: []string{"c"},
			Usage:   "Show current working repository",
		},
		&cli.BoolFlag{
			Name:    "unset",
			Aliases: []string{"u"},
			Usage:   "Clear working repository",
		},
	},
	Action: func(c *cli.Context) error {
		// Create repository manager
		manager, err := repo.NewManager()
		if err != nil {
			return fmt.Errorf("failed to create repository manager: %w", err)
		}

		// Handle --current flag
		if c.Bool("current") {
			repo, err := manager.GetWorkingRepo()
			if err != nil {
				fmt.Println("No working repository set")
				return nil
			}
			fmt.Printf("Current working repository: %s (%s)\n", repo.Name, repo.Path)
			return nil
		}

		// Handle --unset flag
		if c.Bool("unset") {
			if err := manager.ClearWorkingRepo(); err != nil {
				return fmt.Errorf("failed to clear working repository: %w", err)
			}
			fmt.Println("Working repository cleared")
			return nil
		}

		// Require repository argument
		if c.NArg() != 1 {
			return fmt.Errorf("expected exactly one repository argument")
		}

		// Set working repository
		repoArg := c.Args().Get(0)
		if err := manager.SetWorkingRepo(repoArg); err != nil {
			return fmt.Errorf("failed to set working repository: %w", err)
		}

		// Get repository details for confirmation
		repo, err := manager.GetWorkingRepo()
		if err != nil {
			return fmt.Errorf("failed to get working repository: %w", err)
		}

		fmt.Printf("Set working repository to: %s (%s)\n", repo.Name, repo.Path)
		return nil
	},
}
