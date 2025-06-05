package statuscmd

import (
	"fmt"

	"github.com/Mattddixo/dsp/internal/commands/common"
	"github.com/Mattddixo/dsp/internal/commands/flags"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "status",
	Usage: "Show the current status",
	Description: `Show the current status of the repository.
This will display information about the current state, including:
- Number of snapshots
- Latest snapshot
- Tracked files
- Pending changes`,
	Flags: []cli.Flag{
		flags.VerboseFlag,
		flags.QuietFlag,
	},
	Action: func(c *cli.Context) error {
		// Get config - will be used when implementing status logic
		cfg, err := common.GetConfig(c)
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}
		_ = cfg // TODO: Use cfg when implementing status logic

		verbose := c.Bool("verbose")
		quiet := c.Bool("quiet")

		if verbose {
			fmt.Println("Checking repository status...")
		}

		// TODO: Implement status logic
		// This would involve:
		// 1. Reading the snapshots directory
		// 2. Reading the tracking configuration
		// 3. Comparing current state with latest snapshot
		// 4. Displaying status information

		if !quiet {
			fmt.Println("Status functionality not yet implemented")
		}

		return nil
	},
}
