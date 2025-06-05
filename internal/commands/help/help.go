package help

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

// Custom help template that removes default values for boolean flags
const customHelpTemplate = `{{define "helpNameTemplate"}}{{.Name}}{{if .Usage}} - {{.Usage}}{{end}}{{end}}
{{define "helpUsageTemplate"}}{{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}{{end}}

NAME:
   {{template "helpNameTemplate" .}}

USAGE:
   {{template "helpUsageTemplate" .}}

{{if .Description}}DESCRIPTION:
   {{.Description}}{{end}}

{{if .VisibleCommands}}COMMANDS:{{range .VisibleCategories}}
{{if .Name}}
   {{.Name}}:{{end}}{{range .VisibleCommands}}
     {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}
{{end}}{{end}}

{{if .VisibleFlags}}OPTIONS:{{range .VisibleFlagCategories}}
{{if .Name}}
   {{.Name}}:{{end}}{{range .VisibleFlags}}
   {{.String}}{{end}}
{{end}}{{end}}

{{if .Copyright}}COPYRIGHT:
   {{.Copyright}}{{end}}
`

// isBoolFlag returns true if the flag is a boolean flag
func isBoolFlag(f cli.Flag) bool {
	switch f.(type) {
	case *cli.BoolFlag:
		return true
	default:
		return false
	}
}

// SetupHelp configures the custom help template and printer for the CLI app
func SetupHelp(app *cli.App) {
	// Create a custom flag stringer function
	flagStringer := func(f cli.Flag) string {
		// Get the basic flag string
		names := []string{}
		for _, name := range f.Names() {
			if len(name) > 1 {
				names = append(names, "--"+name)
			} else {
				names = append(names, "-"+name)
			}
		}

		// Get usage text based on flag type
		var usage string
		switch v := f.(type) {
		case *cli.BoolFlag:
			usage = v.Usage
		case *cli.StringFlag:
			usage = v.Usage
		case *cli.IntFlag:
			usage = v.Usage
		case *cli.Float64Flag:
			usage = v.Usage
		case *cli.DurationFlag:
			usage = v.Usage
		default:
			usage = "no usage provided"
		}

		// For boolean flags, don't show default value
		if isBoolFlag(f) {
			return strings.Join(names, ", ") + "\t" + usage
		}

		// For other flags, show default value if available
		defaultText := ""
		switch v := f.(type) {
		case *cli.StringFlag:
			if v.DefaultText != "" {
				defaultText = v.DefaultText
			} else if v.Value != "" {
				defaultText = v.Value
			}
		case *cli.IntFlag:
			if v.DefaultText != "" {
				defaultText = v.DefaultText
			} else if v.Value != 0 {
				defaultText = fmt.Sprintf("%d", v.Value)
			}
		case *cli.Float64Flag:
			if v.DefaultText != "" {
				defaultText = v.DefaultText
			} else if v.Value != 0 {
				defaultText = fmt.Sprintf("%f", v.Value)
			}
		case *cli.DurationFlag:
			if v.DefaultText != "" {
				defaultText = v.DefaultText
			} else if v.Value != 0 {
				defaultText = v.Value.String()
			}
		}

		if defaultText != "" {
			return strings.Join(names, ", ") + " value\t" + usage + " (default: " + defaultText + ")"
		}
		return strings.Join(names, ", ") + " value\t" + usage
	}

	// Set the global flag stringer
	cli.FlagStringer = flagStringer

	// Use the custom template
	app.CustomAppHelpTemplate = customHelpTemplate
}
