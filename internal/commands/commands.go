package commands

import (
	"github.com/T-I-M/dsp/config"
	"github.com/T-I-M/dsp/internal/commands/applycmd"
	"github.com/T-I-M/dsp/internal/commands/bundlecmd"
	"github.com/T-I-M/dsp/internal/commands/diffcmd"
	"github.com/T-I-M/dsp/internal/commands/historycmd"
	"github.com/T-I-M/dsp/internal/commands/initcmd"
	"github.com/T-I-M/dsp/internal/commands/repocmd"
	"github.com/T-I-M/dsp/internal/commands/snapshotcmd"
	"github.com/T-I-M/dsp/internal/commands/statuscmd"
	"github.com/T-I-M/dsp/internal/commands/trackcmd"
	"github.com/T-I-M/dsp/internal/commands/untrackcmd"
	"github.com/urfave/cli/v2"
)

// GetConfig retrieves the config from the context
func GetConfig(c *cli.Context) (*config.Config, error) {
	return config.GetConfigFromContext(c.Context)
}

// Common flags used across commands
var (
	VerboseFlag = &cli.BoolFlag{
		Name:    "verbose",
		Aliases: []string{"v"},
		Usage:   "Enable verbose output",
	}
	QuietFlag = &cli.BoolFlag{
		Name:    "quiet",
		Aliases: []string{"q"},
		Usage:   "Suppress non-error output",
	}
)

// Command definitions
var (
	InitCommand     = initcmd.Command
	SnapshotCommand = snapshotcmd.Command
	DiffCommand     = diffcmd.Command
	BundleCommand   = bundlecmd.Command
	ApplyCommand    = applycmd.Command
	StatusCommand   = statuscmd.Command
	HistoryCommand  = historycmd.Command
	TrackCommand    = trackcmd.Command
	UntrackCommand  = untrackcmd.Command
	RepoCommand     = repocmd.Command
)
