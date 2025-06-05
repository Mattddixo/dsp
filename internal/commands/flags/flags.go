package flags

import "github.com/urfave/cli/v2"

// Common flags used across multiple commands

// VerboseFlag enables verbose output
var VerboseFlag = &cli.BoolFlag{
	Name:    "verbose",
	Aliases: []string{"v"},
	Usage:   "Enable verbose output",
}

// QuietFlag suppresses non-error output
var QuietFlag = &cli.BoolFlag{
	Name:    "quiet",
	Aliases: []string{"q"},
	Usage:   "Suppress non-error output",
}

// MessageFlag specifies a message for the operation
var MessageFlag = &cli.StringFlag{
	Name:    "message",
	Aliases: []string{"m"},
	Usage:   "Message describing the operation",
}

// OutputFlag specifies the output file or directory
var OutputFlag = &cli.StringFlag{
	Name:    "output",
	Aliases: []string{"o"},
	Usage:   "Output file or directory",
}

// ForceFlag forces the operation without confirmation
var ForceFlag = &cli.BoolFlag{
	Name:    "force",
	Aliases: []string{"f"},
	Usage:   "Force the operation without confirmation",
}

// RecursiveFlag enables recursive operation
var RecursiveFlag = &cli.BoolFlag{
	Name:    "recursive",
	Aliases: []string{"R"},
	Usage:   "Enable recursive operation",
}

// DryRunFlag shows what would be done without making changes
var DryRunFlag = &cli.BoolFlag{
	Name:    "dry-run",
	Aliases: []string{"n"},
	Usage:   "Show what would be done without making changes",
}
