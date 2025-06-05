package hostcmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/Mattddixo/dsp/internal/commands/flags"
	"github.com/Mattddixo/dsp/internal/host"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "host",
	Usage: "Manage known hosts",
	Description: `Manage known hosts for secure bundle sharing.

A host represents a system that can receive encrypted bundles. Each host has a public key
that is used to encrypt bundles for that host. Hosts can be tagged and aliased for easy
reference.

Commands:
  add           Add a new host
  list          List all known hosts
  show          Show detailed information about a host
  remove        Remove a host
  update        Update host information
  trust         Mark a host as trusted
  untrust       Mark a host as untrusted
  tag           Add tags to a host
  untag         Remove tags from a host
  alias         Set an alias for a host

Examples:
  # Add a new host
  dsp host add --name "Alice's Laptop" --key "age1..." --description "Alice's work laptop"

  # List all hosts
  dsp host list

  # Show host details
  dsp host show "Alice's Laptop"

  # Add tags to a host
  dsp host tag "Alice's Laptop" work laptop

  # Set an alias
  dsp host alias "Alice's Laptop" alice

  # Trust a host
  dsp host trust "Alice's Laptop"

For more information about a specific command, use:
  dsp host <command> --help`,
	Subcommands: []*cli.Command{
		{
			Name:  "add",
			Usage: "Add a new host",
			Description: `Add a new host to the system.

This command adds a new host with their public key. The host can then be used
as a recipient for encrypted bundles.`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "name",
					Usage:    "Name of the host",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "key",
					Usage:    "Public key of the host",
					Required: true,
				},
				&cli.StringFlag{
					Name:  "description",
					Usage: "Description of the host",
				},
				&cli.StringFlag{
					Name:  "alias",
					Usage: "Short alias for the host",
				},
				&cli.StringSliceFlag{
					Name:  "tag",
					Usage: "Tags for the host (can specify multiple)",
				},
				&cli.BoolFlag{
					Name:  "trust",
					Usage: "Mark the host as trusted",
					Value: false,
				},
			},
			Action: func(c *cli.Context) error {
				manager, err := host.NewManager()
				if err != nil {
					return fmt.Errorf("failed to create host manager: %w", err)
				}

				h := &host.Host{
					Name:        c.String("name"),
					PublicKey:   c.String("key"),
					Description: c.String("description"),
					Alias:       c.String("alias"),
					Tags:        c.StringSlice("tag"),
					Trusted:     c.Bool("trust"),
					AddedAt:     time.Now(),
					LastUsed:    time.Now(),
				}

				if err := manager.AddHost(h); err != nil {
					return fmt.Errorf("failed to add host: %w", err)
				}

				fmt.Printf("Added host '%s' successfully!\n", h.Name)
				return nil
			},
		},
		{
			Name:  "list",
			Usage: "List all hosts",
			Description: `List all known hosts.

This command displays all hosts that have been added to your system,
including their names, aliases, and tags.`,
			Flags: []cli.Flag{
				flags.VerboseFlag,
				flags.QuietFlag,
			},
			Action: func(c *cli.Context) error {
				manager, err := host.NewManager()
				if err != nil {
					return fmt.Errorf("failed to create host manager: %w", err)
				}

				hosts := manager.ListHosts()
				if len(hosts) == 0 {
					fmt.Println("No hosts found.")
					return nil
				}

				verbose := c.Bool("verbose")
				quiet := c.Bool("quiet")

				if !quiet {
					fmt.Println("Hosts:")
					for _, h := range hosts {
						if verbose {
							fmt.Printf("\nName: %s\n", h.Name)
							if h.Alias != "" {
								fmt.Printf("Alias: %s\n", h.Alias)
							}
							if h.Description != "" {
								fmt.Printf("Description: %s\n", h.Description)
							}
							if len(h.Tags) > 0 {
								fmt.Printf("Tags: %s\n", strings.Join(h.Tags, ", "))
							}
							fmt.Printf("Trusted: %v\n", h.Trusted)
							fmt.Printf("Added: %s\n", h.AddedAt.Format(time.RFC3339))
							fmt.Printf("Last Used: %s\n", h.LastUsed.Format(time.RFC3339))
							if h.IPAddress != "" {
								fmt.Printf("Last IP: %s\n", h.IPAddress)
							}
							if h.LastPort != 0 {
								fmt.Printf("Last Port: %d\n", h.LastPort)
							}
						} else {
							fmt.Printf("\n%s", h.Name)
							if h.Alias != "" {
								fmt.Printf(" (%s)", h.Alias)
							}
							if len(h.Tags) > 0 {
								fmt.Printf(" [%s]", strings.Join(h.Tags, ", "))
							}
							if h.Trusted {
								fmt.Printf(" (trusted)")
							}
						}
					}
					fmt.Println()
				}

				return nil
			},
		},
		{
			Name:  "show",
			Usage: "Show host details",
			Description: `Show detailed information about a host.

This command displays all information about a specific host, including
their public key, tags, and usage history.`,
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					return fmt.Errorf("expected exactly one host argument")
				}

				manager, err := host.NewManager()
				if err != nil {
					return fmt.Errorf("failed to create host manager: %w", err)
				}

				// Try to get host by name or alias
				h, err := manager.GetHost(c.Args().Get(0))
				if err != nil {
					// Try alias if name not found
					h, err = manager.GetHostByAlias(c.Args().Get(0))
					if err != nil {
						return fmt.Errorf("host not found: %w", err)
					}
				}

				fmt.Printf("Host: %s\n", h.Name)
				if h.Alias != "" {
					fmt.Printf("Alias: %s\n", h.Alias)
				}
				if h.Description != "" {
					fmt.Printf("Description: %s\n", h.Description)
				}
				fmt.Printf("Public Key: %s\n", h.PublicKey)
				if len(h.Tags) > 0 {
					fmt.Printf("Tags: %s\n", strings.Join(h.Tags, ", "))
				}
				fmt.Printf("Trusted: %v\n", h.Trusted)
				fmt.Printf("Added: %s\n", h.AddedAt.Format(time.RFC3339))
				fmt.Printf("Last Used: %s\n", h.LastUsed.Format(time.RFC3339))
				if h.IPAddress != "" {
					fmt.Printf("Last IP: %s\n", h.IPAddress)
				}
				if h.LastPort != 0 {
					fmt.Printf("Last Port: %d\n", h.LastPort)
				}

				return nil
			},
		},
		{
			Name:  "remove",
			Usage: "Remove a host",
			Description: `Remove a host from the system.

This command removes a host and their public key from your system.
After removal, you will no longer be able to encrypt bundles for this host.`,
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					return fmt.Errorf("expected exactly one host argument")
				}

				manager, err := host.NewManager()
				if err != nil {
					return fmt.Errorf("failed to create host manager: %w", err)
				}

				// Try to get host by name or alias
				h, err := manager.GetHost(c.Args().Get(0))
				if err != nil {
					// Try alias if name not found
					h, err = manager.GetHostByAlias(c.Args().Get(0))
					if err != nil {
						return fmt.Errorf("host not found: %w", err)
					}
				}

				if err := manager.RemoveHost(h.Name); err != nil {
					return fmt.Errorf("failed to remove host: %w", err)
				}

				fmt.Printf("Removed host '%s' successfully!\n", h.Name)
				return nil
			},
		},
		{
			Name:  "trust",
			Usage: "Trust a host",
			Description: `Mark a host as trusted.

Trusted hosts are considered safe for receiving encrypted bundles.
This is a security measure to prevent accidental sharing with untrusted hosts.`,
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					return fmt.Errorf("expected exactly one host argument")
				}

				manager, err := host.NewManager()
				if err != nil {
					return fmt.Errorf("failed to create host manager: %w", err)
				}

				// Try to get host by name or alias
				h, err := manager.GetHost(c.Args().Get(0))
				if err != nil {
					// Try alias if name not found
					h, err = manager.GetHostByAlias(c.Args().Get(0))
					if err != nil {
						return fmt.Errorf("host not found: %w", err)
					}
				}

				h.Trusted = true
				if err := manager.UpdateHost(h); err != nil {
					return fmt.Errorf("failed to update host: %w", err)
				}

				fmt.Printf("Marked host '%s' as trusted\n", h.Name)
				return nil
			},
		},
		{
			Name:  "untrust",
			Usage: "Untrust a host",
			Description: `Mark a host as untrusted.

Untrusted hosts will require explicit confirmation before encrypting bundles for them.
This is a security measure to prevent accidental sharing with untrusted hosts.`,
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 {
					return fmt.Errorf("expected exactly one host argument")
				}

				manager, err := host.NewManager()
				if err != nil {
					return fmt.Errorf("failed to create host manager: %w", err)
				}

				// Try to get host by name or alias
				h, err := manager.GetHost(c.Args().Get(0))
				if err != nil {
					// Try alias if name not found
					h, err = manager.GetHostByAlias(c.Args().Get(0))
					if err != nil {
						return fmt.Errorf("host not found: %w", err)
					}
				}

				h.Trusted = false
				if err := manager.UpdateHost(h); err != nil {
					return fmt.Errorf("failed to update host: %w", err)
				}

				fmt.Printf("Marked host '%s' as untrusted\n", h.Name)
				return nil
			},
		},
		{
			Name:  "tag",
			Usage: "Add tags to a host",
			Description: `Add tags to a host.

Tags can be used to organize and filter hosts. For example, you might tag
hosts as "work" or "personal" to easily find them later.`,
			Action: func(c *cli.Context) error {
				if c.NArg() < 2 {
					return fmt.Errorf("expected host name and at least one tag")
				}

				manager, err := host.NewManager()
				if err != nil {
					return fmt.Errorf("failed to create host manager: %w", err)
				}

				// Try to get host by name or alias
				h, err := manager.GetHost(c.Args().Get(0))
				if err != nil {
					// Try alias if name not found
					h, err = manager.GetHostByAlias(c.Args().Get(0))
					if err != nil {
						return fmt.Errorf("host not found: %w", err)
					}
				}

				// Add new tags
				newTags := c.Args().Tail()
				for _, tag := range newTags {
					// Check if tag already exists
					found := false
					for _, t := range h.Tags {
						if t == tag {
							found = true
							break
						}
					}
					if !found {
						h.Tags = append(h.Tags, tag)
					}
				}

				if err := manager.UpdateHost(h); err != nil {
					return fmt.Errorf("failed to update host: %w", err)
				}

				fmt.Printf("Added tags to host '%s': %s\n", h.Name, strings.Join(newTags, ", "))
				return nil
			},
		},
		{
			Name:  "untag",
			Usage: "Remove tags from a host",
			Description: `Remove tags from a host.

This command removes one or more tags from a host.`,
			Action: func(c *cli.Context) error {
				if c.NArg() < 2 {
					return fmt.Errorf("expected host name and at least one tag")
				}

				manager, err := host.NewManager()
				if err != nil {
					return fmt.Errorf("failed to create host manager: %w", err)
				}

				// Try to get host by name or alias
				h, err := manager.GetHost(c.Args().Get(0))
				if err != nil {
					// Try alias if name not found
					h, err = manager.GetHostByAlias(c.Args().Get(0))
					if err != nil {
						return fmt.Errorf("host not found: %w", err)
					}
				}

				// Remove tags
				tagsToRemove := c.Args().Tail()
				var newTags []string
				for _, tag := range h.Tags {
					keep := true
					for _, remove := range tagsToRemove {
						if tag == remove {
							keep = false
							break
						}
					}
					if keep {
						newTags = append(newTags, tag)
					}
				}
				h.Tags = newTags

				if err := manager.UpdateHost(h); err != nil {
					return fmt.Errorf("failed to update host: %w", err)
				}

				fmt.Printf("Removed tags from host '%s': %s\n", h.Name, strings.Join(tagsToRemove, ", "))
				return nil
			},
		},
		{
			Name:  "alias",
			Usage: "Set an alias for a host",
			Description: `Set a short alias for a host.

Aliases provide a quick way to reference hosts without typing their full names.
For example, you might set "alice" as an alias for "Alice's Laptop".`,
			Action: func(c *cli.Context) error {
				if c.NArg() != 2 {
					return fmt.Errorf("expected host name and alias")
				}

				manager, err := host.NewManager()
				if err != nil {
					return fmt.Errorf("failed to create host manager: %w", err)
				}

				// Try to get host by name or alias
				h, err := manager.GetHost(c.Args().Get(0))
				if err != nil {
					// Try alias if name not found
					h, err = manager.GetHostByAlias(c.Args().Get(0))
					if err != nil {
						return fmt.Errorf("host not found: %w", err)
					}
				}

				// Check if alias is already used
				if _, err := manager.GetHostByAlias(c.Args().Get(1)); err == nil {
					return fmt.Errorf("alias '%s' is already in use", c.Args().Get(1))
				}

				h.Alias = c.Args().Get(1)
				if err := manager.UpdateHost(h); err != nil {
					return fmt.Errorf("failed to update host: %w", err)
				}

				fmt.Printf("Set alias '%s' for host '%s'\n", h.Alias, h.Name)
				return nil
			},
		},
	},
}
