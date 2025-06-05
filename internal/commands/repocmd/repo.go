package repocmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mattddixo/dsp/config"
	"github.com/Mattddixo/dsp/internal/repo"
	"github.com/Mattddixo/dsp/internal/snapshot"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

var Command = &cli.Command{
	Name:  "repo",
	Usage: "Manage DSP repositories",
	Description: `Manage DSP repositories and their tracking state.

A DSP repository consists of a directory containing your files and a DSP directory
(by default named .dsp) that stores tracking information, snapshots, and bundles. Requires dsp init to be run first.

Repository Management:
  dsp repo --add <name> <dsp-dir>     # Re-open a closed repository
  dsp repo --move <repo> <path>       # Move a repository to a new location
  dsp repo --set-default <repo>       # Set a repository as the default
  dsp repo --unset-default            # Remove the default repository setting

Repository Information:
  dsp repo --list                     # List all managed repositories
  dsp repo --show <repo>              # Show detailed repository information
  dsp repo --status <repo>            # Show repository tracking state

Examples:
  # Re-open a closed repository with DSP directory at .test
  dsp repo -a my-repo .test

  # Show status of repository named "test"
  dsp repo --status test

  # Show status of repository at specific path
  dsp repo --status /path/to/repo

  # List all repositories with detailed information
  dsp repo --list --verbose

Note: Repository arguments can be specified by either name or path.
      The DSP directory should contain config.yaml and tracking.yaml.`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:     "add",
			Aliases:  []string{"a"},
			Usage:    "Add a closed repository (requires name and DSP directory path)",
			Category: "Repository Management",
		},
		&cli.BoolFlag{
			Name:     "list",
			Aliases:  []string{"l"},
			Usage:    "List all managed repositories",
			Category: "Repository Management",
		},
		&cli.BoolFlag{
			Name:     "move",
			Aliases:  []string{"m"},
			Usage:    "Move a repository to a new location (requires repository and new path)",
			Category: "Repository Management",
		},
		&cli.BoolFlag{
			Name:     "remove",
			Aliases:  []string{"r"},
			Usage:    "Remove a repository from management (does not delete files)",
			Category: "Repository Management",
		},
		&cli.BoolFlag{
			Name:     "rename",
			Aliases:  []string{"n"},
			Usage:    "Rename a repository (requires old and new names)",
			Category: "Repository Management",
		},
		&cli.BoolFlag{
			Name:     "set-default",
			Aliases:  []string{"d"},
			Usage:    "Set a repository as the default for commands",
			Category: "Repository Management",
		},
		&cli.BoolFlag{
			Name:     "unset-default",
			Aliases:  []string{"D"},
			Usage:    "Remove the default repository setting",
			Category: "Repository Management",
		},
		&cli.BoolFlag{
			Name:     "show",
			Aliases:  []string{"s"},
			Usage:    "Show detailed repository information including configuration and tracking",
			Category: "Repository Information",
		},
		&cli.BoolFlag{
			Name:     "status",
			Aliases:  []string{"t"},
			Usage:    "Show repository tracking state and file statistics",
			Category: "Repository Information",
		},
		&cli.StringFlag{
			Name:        "repo",
			Aliases:     []string{"R"},
			Usage:       "Specify target repository by name or path",
			Category:    "Options",
			DefaultText: "nearest repository",
		},
		&cli.BoolFlag{
			Name:     "verbose",
			Aliases:  []string{"v"},
			Usage:    "Show detailed information in listings",
			Category: "Output Options",
		},
		&cli.BoolFlag{
			Name:     "quiet",
			Aliases:  []string{"q"},
			Usage:    "Suppress non-error messages",
			Category: "Output Options",
		},
	},
	Action: func(c *cli.Context) error {
		manager, err := repo.NewManager()
		if err != nil {
			return fmt.Errorf("failed to create repository manager: %w", err)
		}

		// Count how many actions are requested
		actionCount := 0
		actions := []string{
			"add", "list", "move", "remove", "rename",
			"set-default", "unset-default", "show", "status",
		}
		for _, action := range actions {
			if c.Bool(action) {
				actionCount++
			}
		}

		if actionCount == 0 {
			return fmt.Errorf("no action specified. Use --add, --list, --move, --remove, --rename, --set-default, --unset-default, --show, or --status")
		}
		if actionCount > 1 {
			return fmt.Errorf("only one action can be specified at a time")
		}

		// Handle add action
		if c.Bool("add") {
			if c.NArg() != 2 {
				return fmt.Errorf("expected exactly two arguments: repository name and DSP directory path\nUsage: dsp repo -a <name> <dsp-dir>\nExamples:\n  dsp repo -a my-repo .test\n  dsp repo -a my-repo C:\\path\\to\\repo\\.test\n\nNote: The DSP directory should contain config.yaml and tracking.yaml")
			}
			name := c.Args().Get(0)
			dspPath := c.Args().Get(1)

			// Convert DSP path to absolute path
			absDspPath, err := filepath.Abs(dspPath)
			if err != nil {
				return fmt.Errorf("failed to get absolute path: %w", err)
			}

			// Get repository root (parent of DSP directory)
			repoPath := filepath.Dir(absDspPath)
			dspDirName := filepath.Base(absDspPath)

			// Check if DSP directory exists
			dspInfo, err := os.Stat(absDspPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("DSP directory does not exist: %s", absDspPath)
				}
				return fmt.Errorf("failed to access DSP directory: %w", err)
			}
			if !dspInfo.IsDir() {
				return fmt.Errorf("DSP directory path must be a directory: %s", absDspPath)
			}

			// Verify config.yaml exists
			configPath := filepath.Join(absDspPath, "config.yaml")
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				return fmt.Errorf("no DSP configuration found at %s. Please use 'dsp init' to create a new repository", absDspPath)
			}

			// Verify tracking.yaml exists
			trackingPath := filepath.Join(absDspPath, "tracking.yaml")
			if _, err := os.Stat(trackingPath); os.IsNotExist(err) {
				return fmt.Errorf("no tracking configuration found at %s. Please use 'dsp init' to create a new repository", absDspPath)
			}

			fmt.Printf("Adding repository '%s' at %s (DSP directory: %s)...\n", name, repoPath, dspDirName)
			if err := manager.AddRepository(absDspPath, name, false); err != nil {
				// Provide more helpful error messages
				switch {
				case strings.Contains(err.Error(), "no DSP configuration found"):
					return fmt.Errorf("no DSP configuration found at %s. Please use 'dsp init' to create a new repository", absDspPath)
				case strings.Contains(err.Error(), "no tracking configuration found"):
					return fmt.Errorf("no tracking configuration found at %s. Please use 'dsp init' to create a new repository", absDspPath)
				case strings.Contains(err.Error(), "not in a closed state"):
					return fmt.Errorf("repository at %s is not in a closed state. Please use 'dsp repo --remove' first if you want to re-add it", repoPath)
				default:
					return fmt.Errorf("failed to add repository: %w", err)
				}
			}
			fmt.Printf("Successfully added repository: %s (%s)\n", name, repoPath)
			return nil
		}

		// Handle list action
		if c.Bool("list") {
			return listRepos(c)
		}

		// Handle move action
		if c.Bool("move") {
			if c.NArg() != 2 {
				return fmt.Errorf("expected exactly two arguments: repository name/path and new path\n" +
					"Usage: dsp repo --move <repo> <new-path>\n" +
					"Examples:\n" +
					"  dsp repo --move test C:\\new\\path\n" +
					"  dsp repo --move C:\\old\\path C:\\new\\path\n\n" +
					"Note: This will move the entire repository, including all files and DSP metadata.")
			}

			return moveRepository(manager, c.Args().Get(0), c.Args().Get(1))
		}

		// Handle rename action
		if c.Bool("rename") {
			if c.NArg() != 2 {
				return fmt.Errorf("expected exactly two arguments: old name and new name")
			}

			// Get current repository
			currentRepo, err := manager.GetRepository(c.Args().Get(0))
			if err != nil {
				return fmt.Errorf("failed to get repository: %w", err)
			}

			newName := c.Args().Get(1)

			// Find the repository in the manager's list
			for i, repo := range manager.Repos {
				if repo.Path == currentRepo.Path {
					// Update only the name
					manager.Repos[i].Name = newName

					// Save manager state
					if err := manager.Save(); err != nil {
						return fmt.Errorf("failed to save manager state: %w", err)
					}

					fmt.Printf("Renamed repository from '%s' to '%s'\n", currentRepo.Name, newName)
					return nil
				}
			}

			return fmt.Errorf("repository not found: '%s'", currentRepo.Name)
		}

		// Handle remove action
		if c.Bool("remove") {
			if c.NArg() != 1 {
				return fmt.Errorf("expected exactly one repository argument")
			}

			// Get repository details before removal
			repo, err := manager.GetRepository(c.Args().Get(0))
			if err != nil {
				return fmt.Errorf("failed to get repository: %w", err)
			}

			if err := manager.RemoveRepository(c.Args().Get(0)); err != nil {
				return fmt.Errorf("failed to remove repository: %w", err)
			}

			fmt.Printf("Removed repository: %s (%s)\n", repo.Name, repo.Path)
			fmt.Println("Note: Repository files were not deleted")
			return nil
		}

		// Handle set-default action
		if c.Bool("set-default") {
			if c.Bool("unset-default") {
				return fmt.Errorf("cannot use --set-default and --unset-default together")
			}
			if c.NArg() != 1 {
				return fmt.Errorf("expected exactly one repository argument")
			}

			repoArg := c.Args().Get(0)
			if repoArg == "" {
				// Handle unsetting default via empty string
				if err := manager.SetDefault(""); err != nil {
					return fmt.Errorf("failed to unset default repository: %w", err)
				}
				fmt.Println("Default repository setting removed")
				return nil
			}

			// Handle setting a default repository
			if err := manager.SetDefault(repoArg); err != nil {
				return fmt.Errorf("failed to set default repository: %w", err)
			}

			// Get repository details for confirmation
			repo, err := manager.GetRepository(repoArg)
			if err != nil {
				return fmt.Errorf("failed to get repository: %w", err)
			}

			fmt.Printf("Set default repository to: %s (%s)\n", repo.Name, repo.Path)
			return nil
		}

		// Handle unset-default action
		if c.Bool("unset-default") {
			if c.NArg() > 0 {
				return fmt.Errorf("unexpected arguments with --unset-default")
			}

			if err := manager.SetDefault(""); err != nil {
				return fmt.Errorf("failed to unset default repository: %w", err)
			}

			fmt.Println("Default repository setting removed")
			return nil
		}

		// Handle show action
		if c.Bool("show") {
			return showRepo(c)
		}

		// Handle status action
		if c.Bool("status") {
			return showStatus(c)
		}

		return nil
	},
}

// Helper function to get repository status
func getRepoStatus(r *repo.Repository, m *repo.Manager) string {
	var status []string
	if r.IsDefault {
		status = append(status, "default")
	}
	if r.Path == m.WorkingRepo {
		status = append(status, "working")
	}
	if len(status) == 0 {
		return "inactive"
	}
	return strings.Join(status, ", ")
}

// List repositories
func listRepos(_ *cli.Context) error {
	manager, err := repo.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create repository manager: %w", err)
	}

	repos := manager.ListRepositories()
	if len(repos) == 0 {
		fmt.Println("No repositories found. Use 'dsp init' to create a new repository.")
		return nil
	}

	// Print repositories
	fmt.Printf("Found %d repositories:\n\n", len(repos))
	for _, r := range repos {
		// Print repository info
		fmt.Printf("Repository: %s\n", r.Name)
		fmt.Printf("  Path: %s\n", r.Path) // Always use absolute path
		fmt.Printf("  DSP Directory: %s\n", r.DSPDir)
		if r.IsDefault {
			fmt.Println("  Default: Yes")
		}
		if r.Path == manager.WorkingRepo {
			fmt.Println("  Working Repository: Yes")
		}

		// Load tracking config to show tracked paths
		dspDir := filepath.Join(r.Path, r.DSPDir)
		trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
		if err != nil {
			fmt.Printf("  Warning: Could not load tracking config: %v\n", err)
			continue
		}

		// Print tracked paths
		if len(trackingConfig.Paths) > 0 {
			fmt.Println("  Tracked Paths:")
			for _, path := range trackingConfig.Paths {
				// Always use absolute path for clarity
				fmt.Printf("    - %s (%s)\n", path.Path, formatType(path.IsDir))
			}
		}

		fmt.Println()
	}

	return nil
}

// Show repository details
func showRepo(c *cli.Context) error {
	manager, err := repo.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create repository manager: %w", err)
	}

	// Get repository
	repo, err := manager.GetRepository(c.Args().First())
	if err != nil {
		return err
	}

	// Load tracking config
	dspDir := filepath.Join(repo.Path, repo.DSPDir)
	trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
	if err != nil {
		return fmt.Errorf("failed to load tracking config: %w", err)
	}

	// Load repository config
	configPath := filepath.Join(dspDir, "config.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read repository config: %w", err)
	}

	var repoConfig config.Config
	if err := yaml.Unmarshal(configData, &repoConfig); err != nil {
		return fmt.Errorf("failed to parse repository config: %w", err)
	}

	// Print repository details
	fmt.Printf("Repository Information:\n")
	fmt.Printf("  Name: %s\n", repo.Name)
	fmt.Printf("  Path: %s\n", repo.Path)
	fmt.Printf("  DSP Directory: %s\n", repo.DSPDir)

	// Print repository status
	fmt.Printf("\nRepository Status:\n")
	status := getRepoStatus(repo, manager)
	if status != "inactive" {
		fmt.Printf("  Status: %s\n", status)
	} else {
		fmt.Printf("  Status: inactive\n")
	}

	// Print working directory status
	if repo.Path == manager.WorkingRepo {
		fmt.Printf("  Working Directory: Yes\n")
	} else {
		fmt.Printf("  Working Directory: No\n")
	}

	// Print configuration
	fmt.Printf("\nRepository Configuration:\n")
	fmt.Printf("  Data Directory: %s\n", repoConfig.DataDir)
	fmt.Printf("  Hash Algorithm: %s\n", repoConfig.HashAlgorithm)
	fmt.Printf("  Compression Level: %d\n", repoConfig.CompressionLevel)

	// Print tracking state
	fmt.Printf("\nTracking State:\n")
	if snapshot.IsRepositoryClosed(trackingConfig) {
		fmt.Printf("  Status: Closed\n")
		fmt.Printf("  Closed At: %s\n", trackingConfig.State.ClosedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Closed By: %s\n", trackingConfig.State.ClosedBy)
	} else {
		fmt.Printf("  Status: Active\n")
		if !trackingConfig.State.LastModified.IsZero() {
			fmt.Printf("  Last Modified: %s\n", trackingConfig.State.LastModified.Format("2006-01-02 15:04:05"))
		}
	}

	// Print tracked paths
	if len(trackingConfig.Paths) > 0 {
		fmt.Printf("\nTracked Paths (%d):\n", len(trackingConfig.Paths))
		for _, path := range trackingConfig.Paths {
			// Always use absolute path for clarity
			fmt.Printf("  - %s (%s)\n", path.Path, formatType(path.IsDir))
			if len(path.Excludes) > 0 {
				fmt.Printf("    Excludes: %s\n", strings.Join(path.Excludes, ", "))
			}
		}
	} else {
		fmt.Printf("\nNo tracked paths\n")
	}

	return nil
}

// Handle status action
func showStatus(c *cli.Context) error {
	manager, err := repo.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create repository manager: %w", err)
	}

	// Get repository - use argument if provided, otherwise use working repo
	var currentRepo *repo.Repository
	if c.NArg() > 0 {
		currentRepo, err = manager.GetRepository(c.Args().Get(0))
		if err != nil {
			return fmt.Errorf("failed to get repository '%s': %w", c.Args().Get(0), err)
		}
	} else {
		currentRepo, err = manager.GetCurrentRepo("")
		if err != nil {
			return fmt.Errorf("failed to get current repository: %w", err)
		}
	}

	// Get DSP directory path
	dspDir := currentRepo.GetDSPDir()

	// Load tracking config
	trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
	if err != nil {
		return fmt.Errorf("failed to load tracking config: %w", err)
	}

	// Print repository state
	fmt.Printf("Repository: %s\n", currentRepo.Name)
	fmt.Printf("Path: %s\n", currentRepo.Path)
	fmt.Printf("DSP Directory: %s\n", currentRepo.DSPDir)
	fmt.Printf("Management Status: %s\n", getRepoStatus(currentRepo, manager))

	// Print tracking state
	if snapshot.IsRepositoryClosed(trackingConfig) {
		fmt.Printf("\nTracking State: Closed\n")
		fmt.Printf("  Closed At: %s\n", trackingConfig.State.ClosedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Closed By: %s\n", trackingConfig.State.ClosedBy)
	} else {
		fmt.Printf("\nTracking State: Active\n")
		if !trackingConfig.State.LastModified.IsZero() {
			fmt.Printf("  Last Modified: %s\n", trackingConfig.State.LastModified.Format("2006-01-02 15:04:05"))
		}
	}

	// Print tracked paths
	if len(trackingConfig.Paths) > 0 {
		fmt.Printf("\nTracked Paths (%d):\n", len(trackingConfig.Paths))
		for _, path := range trackingConfig.Paths {
			// Always use absolute path for clarity
			fmt.Printf("  - %s (%s)\n", path.Path, formatType(path.IsDir))
			if len(path.Excludes) > 0 {
				fmt.Printf("    Excludes: %s\n", strings.Join(path.Excludes, ", "))
			}
		}
	} else {
		fmt.Printf("\nNo tracked paths\n")
	}

	return nil
}

// Helper function to format file type
func formatType(isDir bool) string {
	if isDir {
		return "directory"
	}
	return "file"
}

// moveRepository handles the complete process of moving a repository
func moveRepository(manager *repo.Manager, repoArg, newPath string) error {
	// Get current repository by name or path
	currentRepo, err := manager.GetRepository(repoArg)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Convert new path to absolute path
	absNewPath, err := filepath.Abs(newPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Don't allow moving to the same location
	if currentRepo.Path == absNewPath {
		return fmt.Errorf("repository is already at %s", absNewPath)
	}

	// Check if destination is inside the repository
	relPath, err := filepath.Rel(currentRepo.Path, absNewPath)
	if err == nil && !strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("cannot move repository into itself: %s is inside %s", absNewPath, currentRepo.Path)
	}

	// Check if destination directory exists
	destInfo, err := os.Stat(absNewPath)
	if err == nil {
		// Directory exists, check if it's a directory
		if !destInfo.IsDir() {
			return fmt.Errorf("destination exists but is not a directory: %s", absNewPath)
		}

		// Check if destination is already registered as a repository root
		for _, repo := range manager.Repos {
			if repo.Path == absNewPath {
				return fmt.Errorf("destination is already registered as a repository root: %s", absNewPath)
			}
		}
	} else if !os.IsNotExist(err) {
		// Some other error occurred
		return fmt.Errorf("failed to check destination directory: %w", err)
	}

	// Load repository config to get DSP and data directory paths
	dspDir := filepath.Join(currentRepo.Path, currentRepo.DSPDir)
	configPath := filepath.Join(dspDir, "config.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read repository config: %w", err)
	}

	var repoConfig config.Config
	if err := yaml.Unmarshal(configData, &repoConfig); err != nil {
		return fmt.Errorf("failed to parse repository config: %w", err)
	}

	// Get absolute paths for both directories
	srcDspDir := filepath.Join(currentRepo.Path, currentRepo.DSPDir)
	srcDataDir := filepath.Join(currentRepo.Path, repoConfig.DataDir)
	dstDspDir := filepath.Join(absNewPath, currentRepo.DSPDir)
	dstDataDir := filepath.Join(absNewPath, repoConfig.DataDir)

	// Check if data directory is a subdirectory of DSP directory
	isDataInDsp := false
	if relPath, err := filepath.Rel(srcDspDir, srcDataDir); err == nil && !strings.HasPrefix(relPath, "..") {
		isDataInDsp = true
	}

	// Print what will be moved
	fmt.Printf("\nMoving DSP repository '%s':\n", currentRepo.Name)
	fmt.Printf("  From: %s\n", currentRepo.Path)
	fmt.Printf("  To:   %s\n\n", absNewPath)
	fmt.Println("This will move the following DSP directories:")
	fmt.Printf("  1. DSP directory (%s) containing:\n", currentRepo.DSPDir)
	fmt.Printf("     - config.yaml (repository configuration)\n")
	fmt.Printf("     - tracking.yaml (tracked files and state)\n")
	fmt.Printf("     - .gitignore (if present)\n")

	if isDataInDsp {
		fmt.Printf("  2. Data directory (%s) containing:\n", repoConfig.DataDir)
		fmt.Printf("     - snapshots/\n")
		fmt.Printf("     - bundles/\n")
		fmt.Printf("     - other DSP data files\n")
	} else {
		fmt.Printf("\nNote: Data directory (%s) is not a subdirectory of the DSP directory.\n", repoConfig.DataDir)
		fmt.Printf("      It will remain in its current location.\n")
		fmt.Printf("      You may need to update the data directory path in the repository configuration.\n")
	}

	fmt.Println()
	fmt.Println("Note: Other files in the repository directory will remain in place.")
	fmt.Println("      Only DSP's own files and directories will be moved.")
	fmt.Println()

	// Ask for confirmation
	fmt.Print("Do you want to continue? (y/N) ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		return fmt.Errorf("move operation cancelled")
	}

	// Create a temporary directory for the move operation
	tempDir, err := os.MkdirTemp("", "dsp-move-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Start the move operation
	fmt.Printf("\nMoving DSP directories...\n")

	// 1. First, copy DSP directory to the temp directory
	tempDspDir := filepath.Join(tempDir, filepath.Base(srcDspDir))
	if err := copyDir(srcDspDir, tempDspDir); err != nil {
		return fmt.Errorf("failed to copy DSP directory to temporary location: %w", err)
	}

	// Copy data directory only if it's a subdirectory of DSP directory
	var tempDataDir string
	if isDataInDsp {
		tempDataDir = filepath.Join(tempDir, filepath.Base(srcDataDir))
		if err := copyDir(srcDataDir, tempDataDir); err != nil {
			return fmt.Errorf("failed to copy data directory to temporary location: %w", err)
		}
	}

	// 2. Verify the copy was successful
	if _, err := os.Stat(filepath.Join(tempDspDir, "config.yaml")); os.IsNotExist(err) {
		return fmt.Errorf("failed to copy DSP directory: missing config.yaml")
	}
	if _, err := os.Stat(filepath.Join(tempDspDir, "tracking.yaml")); os.IsNotExist(err) {
		return fmt.Errorf("failed to copy DSP directory: missing tracking.yaml")
	}

	// 3. Move from temp to final destination
	fmt.Printf("Moving to final location...\n")

	// Create parent directories for destination
	if err := os.MkdirAll(filepath.Dir(dstDspDir), 0755); err != nil {
		return fmt.Errorf("failed to create destination parent directory: %w", err)
	}

	// Move DSP directory
	if err := os.Rename(tempDspDir, dstDspDir); err != nil {
		// If rename fails, try copy and delete
		if err := copyDir(tempDspDir, dstDspDir); err != nil {
			return fmt.Errorf("failed to move DSP directory to final location: %w", err)
		}
		// Only delete the original if the move was successful
		if err := os.RemoveAll(srcDspDir); err != nil {
			fmt.Printf("Warning: Failed to remove old DSP directory: %v\n", err)
		}
	}

	// Move data directory only if it's a subdirectory of DSP directory
	if isDataInDsp {
		if err := os.MkdirAll(filepath.Dir(dstDataDir), 0755); err != nil {
			return fmt.Errorf("failed to create destination parent directory: %w", err)
		}

		if err := os.Rename(tempDataDir, dstDataDir); err != nil {
			// If rename fails, try copy and delete
			if err := copyDir(tempDataDir, dstDataDir); err != nil {
				return fmt.Errorf("failed to move data directory to final location: %w", err)
			}
			// Only delete the original if the move was successful
			if err := os.RemoveAll(srcDataDir); err != nil {
				fmt.Printf("Warning: Failed to remove old data directory: %v\n", err)
			}
		}
	}

	// 4. Update repository registration
	fmt.Printf("Updating repository registration...\n")
	if err := manager.RemoveRepository(currentRepo.Path); err != nil {
		// If this fails, we should try to restore the original location
		if restoreErr := os.Rename(dstDspDir, srcDspDir); restoreErr != nil {
			fmt.Printf("Warning: Failed to restore DSP directory after registration error: %v\n", restoreErr)
		}
		if isDataInDsp {
			if restoreErr := os.Rename(dstDataDir, srcDataDir); restoreErr != nil {
				fmt.Printf("Warning: Failed to restore data directory after registration error: %v\n", restoreErr)
			}
		}
		return fmt.Errorf("failed to update repository registration: %w", err)
	}

	// 5. Add repository at new location
	if err := manager.AddRepository(dstDspDir, currentRepo.Name, currentRepo.IsDefault); err != nil {
		// If this fails, try to restore the original location
		if restoreErr := os.Rename(dstDspDir, srcDspDir); restoreErr != nil {
			fmt.Printf("Warning: Failed to restore DSP directory after registration error: %v\n", restoreErr)
		}
		if isDataInDsp {
			if restoreErr := os.Rename(dstDataDir, srcDataDir); restoreErr != nil {
				fmt.Printf("Warning: Failed to restore data directory after registration error: %v\n", restoreErr)
			}
		}
		// Try to restore the original registration
		_ = manager.AddRepository(srcDspDir, currentRepo.Name, currentRepo.IsDefault)
		return fmt.Errorf("failed to register repository at new location: %w", err)
	}

	fmt.Printf("\nSuccessfully moved DSP directories to %s\n", absNewPath)
	fmt.Printf("  - DSP directory: %s\n", dstDspDir)
	if isDataInDsp {
		fmt.Printf("  - Data directory: %s\n", dstDataDir)
	} else {
		fmt.Printf("  - Data directory remains at: %s\n", srcDataDir)
		fmt.Printf("    You may need to update the data directory path in the repository configuration.\n")
	}
	fmt.Printf("Note: Only DSP directories were moved. Other files in %s remain unchanged.\n", currentRepo.Path)
	fmt.Printf("You can verify the move with: dsp repo -l\n")
	return nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get source directory info: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectories
			if err := copyDir(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", srcPath, err)
			}
		} else {
			// Copy files
			if err := copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", srcPath, err)
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Get source file info
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy file contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}
