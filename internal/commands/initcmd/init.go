package initcmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/T-I-M/dsp/config"
	"github.com/T-I-M/dsp/internal/repo"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "init",
	Usage: "Initialize a new DSP repository",
	Description: `Initialize a new DSP repository in the current directory.
This command creates the necessary directory structure and configuration files
for a new DSP repository. The repository will be registered with the DSP
repository manager, allowing you to manage multiple repositories.

Examples:
  # Initialize in current directory
  dsp init

  # Initialize with a specific name
  dsp init --name "my-project"

  # Initialize and set as default repository
  dsp init --name "my-project" --default

  # Initialize in a specific directory
  dsp init /path/to/directory

Note: Each project should have its own DSP repository. Avoid initializing
DSP in your home directory or in the DSP tool's source code directory.`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "name",
			Aliases: []string{"n"},
			Usage:   "Name for the repository",
		},
		&cli.BoolFlag{
			Name:    "default",
			Aliases: []string{"d"},
			Usage:   "Set as default repository",
		},
	},
	Action: func(c *cli.Context) error {
		// Get target directory
		targetDir := "."
		if c.NArg() > 0 {
			targetDir = c.Args().Get(0)
		}

		// Convert to absolute path
		absPath, err := filepath.Abs(targetDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		fmt.Printf("Initializing DSP repository in: %s\n", absPath)

		// Create config.yaml
		cfg, err := config.New()
		if err != nil {
			return fmt.Errorf("failed to create default configuration: %w", err)
		}

		// Ask if user wants to customize the configuration
		fmt.Print("\nWould you like to customize the configuration? (y/N) ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "y" || response == "yes" {
			// Customize DSP directory
			fmt.Printf("\nDSP directory [%s]: ", cfg.DSPDir)
			if dspDir, _ := reader.ReadString('\n'); strings.TrimSpace(dspDir) != "" {
				cfg.DSPDir = strings.TrimSpace(dspDir)
			}

			// Customize data directory
			fmt.Printf("Data directory [%s]: ", cfg.DataDir)
			if dataDir, _ := reader.ReadString('\n'); strings.TrimSpace(dataDir) != "" {
				cfg.DataDir = strings.TrimSpace(dataDir)
			}

			// Get compression level
			fmt.Printf("\nCompression level (1-9) [%d]: ", cfg.CompressionLevel)
			if level, _ := reader.ReadString('\n'); strings.TrimSpace(level) != "" {
				if l, err := strconv.Atoi(strings.TrimSpace(level)); err == nil && l >= 1 && l <= 9 {
					cfg.CompressionLevel = l
				}
			}
		}

		// Create directories using the configured paths
		dspDir := filepath.Join(absPath, cfg.DSPDir)
		if err := os.MkdirAll(dspDir, 0755); err != nil {
			return fmt.Errorf("failed to create DSP directory: %w", err)
		}

		// Create data directory
		dataDir := filepath.Join(absPath, cfg.DataDir)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// Create snapshots directory
		snapshotsDir := filepath.Join(dspDir, "snapshots")
		if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
			return fmt.Errorf("failed to create snapshots directory: %w", err)
		}

		// Create bundles directory
		bundlesDir := filepath.Join(dspDir, "bundles")
		if err := os.MkdirAll(bundlesDir, 0755); err != nil {
			return fmt.Errorf("failed to create bundles directory: %w", err)
		}

		// Create tracking.yaml
		trackingPath := filepath.Join(dspDir, "tracking.yaml")
		if err := os.WriteFile(trackingPath, []byte("paths: []\n"), 0644); err != nil {
			return fmt.Errorf("failed to create tracking.yaml: %w", err)
		}

		// Save config to repository
		configPath := filepath.Join(dspDir, "config.yaml")
		if err := cfg.Save(configPath); err != nil {
			return fmt.Errorf("failed to save config.yaml: %w", err)
		}

		// Create .gitignore
		gitignorePath := filepath.Join(dspDir, ".gitignore")
		gitignoreContent := `# DSP data files
data/
snapshots/
bundles/
`
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			return fmt.Errorf("failed to create .gitignore: %w", err)
		}

		// Register repository with manager
		manager, err := repo.NewManager()
		if err != nil {
			return fmt.Errorf("failed to create repository manager: %w", err)
		}

		// Use directory name as default name if not specified
		name := c.String("name")
		if name == "" {
			name = filepath.Base(absPath)
		}

		if err := manager.InitializeRepository(absPath, name, c.Bool("default"), cfg.DSPDir); err != nil {
			return fmt.Errorf("failed to initialize repository: %w", err)
		}

		fmt.Printf("\nRepository initialized successfully!\n")
		fmt.Printf("Repository name: %s\n", name)
		fmt.Printf("DSP directory: %s\n", cfg.DSPDir)
		if c.Bool("default") {
			fmt.Println("Set as default repository")
		}
		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  1. Track files: dsp track <path>\n")
		fmt.Printf("  2. Create a snapshot: dsp snapshot -m \"Initial snapshot\"\n")
		fmt.Printf("  3. View status: dsp status\n")

		return nil
	},
}
