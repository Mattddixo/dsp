package main

import (
	"fmt"
	"os"

	"github.com/Mattddixo/dsp/config"
	"github.com/Mattddixo/dsp/internal/commands"
	"github.com/Mattddixo/dsp/internal/commands/cryptocmd"
	"github.com/Mattddixo/dsp/internal/commands/exportcmd"
	"github.com/Mattddixo/dsp/internal/commands/help"
	"github.com/Mattddixo/dsp/internal/commands/hostcmd"
	"github.com/Mattddixo/dsp/internal/commands/usecmd"
	"github.com/urfave/cli/v2"
)

func main() {
	// Load configuration
	cfg, err := config.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Create app
	app := &cli.App{
		Name:  "dsp",
		Usage: "Disconnected Sync Protocol",
		Description: `A tool for managing disconnected synchronization of files.
DSP allows you to track, snapshot, and sync files across different systems.
Each project can have its own DSP repository, and you can manage multiple repositories.

Basic workflow:
  1. Initialize a repository: dsp init
  2. Set working repository: dsp use <repo>
  3. Track files: dsp track <path>
  4. Create snapshots: dsp snapshot -m "message"
  5. View status: dsp status
  6. Create bundles: dsp bundle
  7. Apply changes: dsp apply

For more information about a command, use: dsp <command> -h`,
		Commands: []*cli.Command{
			commands.InitCommand,
			commands.TrackCommand,
			commands.UntrackCommand,
			commands.SnapshotCommand,
			commands.DiffCommand,
			commands.BundleCommand,
			commands.ApplyCommand,
			commands.StatusCommand,
			commands.HistoryCommand,
			commands.RepoCommand,
			usecmd.Command,
			cryptocmd.Command(),
			hostcmd.Command,
			exportcmd.Command,
		},
		Before: func(c *cli.Context) error {
			// Add config to context
			c.Context = cfg.WithContext(c.Context)
			return nil
		},
		ExitErrHandler: func(c *cli.Context, err error) {
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Setup custom help template
	help.SetupHelp(app)

	// Run app
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error running command: %v\n", err)
		os.Exit(1)
	}
}
