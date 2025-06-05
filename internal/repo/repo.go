package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Mattddixo/dsp/config"
	"github.com/Mattddixo/dsp/internal/snapshot"
	"gopkg.in/yaml.v3"
)

// Repository represents a DSP repository
type Repository struct {
	Path      string `yaml:"path"`       // Absolute path to repository root
	Name      string `yaml:"name"`       // User-friendly name for the repository
	IsDefault bool   `yaml:"is_default"` // Whether this is the default repository
	DSPDir    string `yaml:"dsp_dir"`    // The DSP directory path for this repository
}

// Manager handles multiple DSP repositories
type Manager struct {
	Repos       []Repository `yaml:"repos"`
	DefaultRepo string       `yaml:"default_repo"`
	WorkingRepo string       `yaml:"working_repo"` // New field for working repository
	ConfigPath  string       // Path to the manager's config file
}

// NewManager creates a new repository manager
func NewManager() (*Manager, error) {
	// Get user's home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create .dsp-global directory in user's home
	globalDir := filepath.Join(home, ".dsp-global")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create global DSP directory: %w", err)
	}

	// Initialize manager
	manager := &Manager{
		ConfigPath: filepath.Join(globalDir, "repos.yaml"),
	}

	// Load existing config if it exists
	if err := manager.Load(); err != nil {
		return nil, fmt.Errorf("failed to load repository config: %w", err)
	}

	return manager, nil
}

// Load loads the repository configuration
func (m *Manager) Load() error {
	// If config doesn't exist, create empty config
	if _, err := os.Stat(m.ConfigPath); os.IsNotExist(err) {
		m.Repos = []Repository{}
		return m.Save()
	}

	// Read config file
	data, err := os.ReadFile(m.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, m); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

// Save saves the repository configuration
func (m *Manager) Save() error {
	// Marshal to YAML
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(m.ConfigPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// InitializeRepository initializes a new repository
func (m *Manager) InitializeRepository(path string, name string, isDefault bool, dspDir string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if repository is already registered
	for _, repo := range m.Repos {
		if repo.Path == absPath {
			return fmt.Errorf("repository already registered at %s", absPath)
		}
	}

	// Add new repository
	repo := Repository{
		Path:      absPath,
		Name:      name,
		IsDefault: isDefault,
		DSPDir:    dspDir,
	}
	m.Repos = append(m.Repos, repo)

	// Update default if needed
	if isDefault {
		m.DefaultRepo = absPath
	}

	// Save manager state
	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manager state: %w", err)
	}

	return nil
}

// AddRepository adds a previously closed repository
func (m *Manager) AddRepository(path string, name string, isDefault bool) error {
	// Convert DSP directory path to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	fmt.Printf("Debug: Absolute path: %s\n", absPath)

	// Get repository root (parent of DSP directory)
	repoRoot := filepath.Dir(absPath)
	dspDirName := filepath.Base(absPath)

	// Check if repository is already registered
	for _, repo := range m.Repos {
		if repo.Path == repoRoot {
			return fmt.Errorf("repository already registered at %s", repoRoot)
		}
	}

	// Load repository config from the DSP directory
	configPath := filepath.Join(absPath, "config.yaml")
	fmt.Printf("Debug: Looking for config at: %s\n", configPath)

	// Check if file exists first
	if _, err := os.Stat(configPath); err != nil {
		fmt.Printf("Debug: Stat error: %v\n", err)
		if os.IsNotExist(err) {
			return fmt.Errorf("no DSP configuration found at %s. Please use 'dsp init' to create a new repository", absPath)
		}
		return fmt.Errorf("failed to check config file: %w", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("Debug: ReadFile error: %v\n", err)
		return fmt.Errorf("failed to load repository config: %w", err)
	}

	// Parse config to get DSP directory
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse repository config: %w", err)
	}

	// Load and verify tracking config
	trackingConfig, err := snapshot.LoadTrackingConfig(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no tracking configuration found at %s. Please use 'dsp init' to create a new repository", absPath)
		}
		return fmt.Errorf("failed to load repository state: %w", err)
	}

	// If repository was closed, reopen it
	if snapshot.IsRepositoryClosed(trackingConfig) {
		// Reopen the repository by clearing the closed state
		trackingConfig.State = snapshot.RepositoryState{
			IsClosed:     false,
			LastModified: time.Now(),
		}
		if err := snapshot.SaveTrackingConfig(absPath, trackingConfig); err != nil {
			return fmt.Errorf("failed to reopen repository: %w", err)
		}
	}

	// Add new repository
	repo := Repository{
		Path:      repoRoot,
		Name:      name,
		IsDefault: isDefault,
		DSPDir:    dspDirName,
	}
	m.Repos = append(m.Repos, repo)

	// Update default if needed
	if isDefault {
		m.DefaultRepo = repoRoot
	}

	// Save manager state
	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manager state: %w", err)
	}

	fmt.Printf("Added repository '%s' at %s (DSP directory: %s)\n", name, repoRoot, dspDirName)
	return nil
}

// RemoveRepository removes a repository by name or path
func (m *Manager) RemoveRepository(repoArg string) error {
	// First try to find repository by name
	var targetRepo *Repository
	for i, repo := range m.Repos {
		if repo.Name == repoArg {
			// Found by name, store a copy of the repository info
			targetRepo = &m.Repos[i]
			break
		}
	}

	// If not found by name, try as path
	if targetRepo == nil {
		absPath, err := filepath.Abs(repoArg)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Find repository by path
		for i, repo := range m.Repos {
			if repo.Path == absPath {
				targetRepo = &m.Repos[i]
				break
			}
		}
	}

	// If still not found, return error
	if targetRepo == nil {
		return fmt.Errorf("repository not found: '%s' (tried as both name and path). Use 'dsp repo list' to see available repositories", repoArg)
	}

	// Store repository info before removal
	repoPath := targetRepo.Path
	dspDir := targetRepo.DSPDir

	// Find and remove from manager's list
	for i, repo := range m.Repos {
		if repo.Path == repoPath {
			// Remove from manager
			m.Repos = append(m.Repos[:i], m.Repos[i+1:]...)

			// Update default if needed
			if m.DefaultRepo == repoPath {
				if len(m.Repos) > 0 {
					m.DefaultRepo = m.Repos[0].Path
				} else {
					m.DefaultRepo = ""
				}
			}

			// Update working repo if needed
			if m.WorkingRepo == repoPath {
				m.WorkingRepo = ""
			}

			// Save manager state
			if err := m.Save(); err != nil {
				return fmt.Errorf("failed to save manager state: %w", err)
			}

			// Mark repository as closed in tracking config using stored info
			if err := m.closeRepositoryTrackingWithInfo(repoPath, dspDir); err != nil {
				// Log the error but don't fail the removal
				fmt.Printf("Warning: Failed to close repository tracking: %v\n", err)
			}

			return nil
		}
	}

	return fmt.Errorf("repository not found: '%s' (tried as both name and path). Use 'dsp repo list' to see available repositories", repoArg)
}

// closeRepositoryTrackingWithInfo marks a repository as closed using provided info
func (m *Manager) closeRepositoryTrackingWithInfo(repoPath, dspDir string) error {
	// Get DSP directory path
	fullDspDir := filepath.Join(repoPath, dspDir)

	// Load tracking config
	trackingConfig, err := snapshot.LoadTrackingConfig(fullDspDir)
	if err != nil {
		return fmt.Errorf("failed to load tracking config: %w", err)
	}

	// Mark repository as closed
	trackingConfig.State = snapshot.RepositoryState{
		IsClosed:     true,
		ClosedAt:     time.Now(),
		ClosedBy:     os.Getenv("USERNAME"), // Use current user
		LastModified: time.Now(),
	}

	// Save tracking config
	if err := snapshot.SaveTrackingConfig(fullDspDir, trackingConfig); err != nil {
		return fmt.Errorf("failed to save tracking config: %w", err)
	}

	return nil
}

// closeRepositoryTracking marks a repository as closed in its tracking configuration
func (m *Manager) closeRepositoryTracking(repoPath string) error {
	// Get repository to get DSP directory
	repo, err := m.GetRepository(repoPath)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	return m.closeRepositoryTrackingWithInfo(repoPath, repo.DSPDir)
}

// SetDefault sets or unsets the default repository
func (m *Manager) SetDefault(repoArg string) error {
	// If empty string is provided, unset the default
	if repoArg == "" {
		// Clear default flag for all repositories
		for i := range m.Repos {
			m.Repos[i].IsDefault = false
		}
		m.DefaultRepo = ""

		// Save changes
		if err := m.Save(); err != nil {
			return fmt.Errorf("failed to save manager state: %w", err)
		}
		return nil
	}

	// First try to find repository by name
	var targetRepo *Repository
	for i, repo := range m.Repos {
		if repo.Name == repoArg {
			targetRepo = &m.Repos[i]
			break
		}
	}

	// If not found by name, try as path
	if targetRepo == nil {
		absPath, err := filepath.Abs(repoArg)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Find repository by path
		for i, repo := range m.Repos {
			if repo.Path == absPath {
				targetRepo = &m.Repos[i]
				break
			}
		}
	}

	// If still not found, return error
	if targetRepo == nil {
		return fmt.Errorf("repository not found: '%s' (tried as both name and path). Use 'dsp repo list' to see available repositories", repoArg)
	}

	// Update default flag for all repositories
	for i := range m.Repos {
		m.Repos[i].IsDefault = (m.Repos[i].Path == targetRepo.Path)
	}
	m.DefaultRepo = targetRepo.Path

	// Save changes
	if err := m.Save(); err != nil {
		return fmt.Errorf("failed to save manager state: %w", err)
	}

	return nil
}

// GetRepository gets a repository by name or path
func (m *Manager) GetRepository(repoArg string) (*Repository, error) {
	// First try to find repository by name
	for _, repo := range m.Repos {
		if repo.Name == repoArg {
			// Create a copy of the repository
			repoCopy := repo
			return &repoCopy, nil
		}
	}

	// If not found by name, try as path
	absPath, err := filepath.Abs(repoArg)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Find repository by path
	for _, repo := range m.Repos {
		if repo.Path == absPath {
			// Create a copy of the repository
			repoCopy := repo
			return &repoCopy, nil
		}
	}

	// If not found, provide a helpful error message
	return nil, fmt.Errorf("repository not found: '%s' (tried as both name and path). Use 'dsp repo list' to see available repositories", repoArg)
}

// GetDefaultRepository gets the default repository
func (m *Manager) GetDefaultRepository() (*Repository, error) {
	if m.DefaultRepo == "" {
		return nil, fmt.Errorf("no default repository set")
	}

	return m.GetRepository(m.DefaultRepo)
}

// GetCurrentRepo gets the current repository context based on flags and working repo
func (m *Manager) GetCurrentRepo(repoFlag string) (*Repository, error) {
	// If repo flag is set, use that (highest priority)
	if repoFlag != "" {
		return m.GetRepository(repoFlag)
	}

	// If working repo is set, use that (second priority)
	if m.WorkingRepo != "" {
		return m.GetWorkingRepo()
	}

	// If default repo is set, use that (third priority)
	if m.DefaultRepo != "" {
		return m.GetDefaultRepository()
	}

	// Finally, check if we're in a repository root (lowest priority)
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if current directory is a repository root
	for _, repo := range m.Repos {
		if repo.Path == cwd {
			return &repo, nil
		}
	}

	// If we get here, we have no valid repository context
	return nil, fmt.Errorf("no repository context available:\n" +
		"  - No --repo flag specified\n" +
		"  - No working repository set (use 'dsp use <repo>' to set one)\n" +
		"  - No default repository set (use 'dsp repo --set-default <repo>' to set one)\n" +
		"  - Not in a repository root directory\n" +
		"\nTo resolve this, either:\n" +
		"  1. Use --repo flag to specify a repository\n" +
		"  2. Set a working repository with 'dsp use <repo>'\n" +
		"  3. Set a default repository with 'dsp repo --set-default <repo>'\n" +
		"  4. Change to a repository root directory")
}

// FindNearestRepository is deprecated - use GetCurrentRepo instead
func (m *Manager) FindNearestRepository() (*Repository, error) {
	return m.GetCurrentRepo("")
}

// ListRepositories returns a list of all repositories
func (m *Manager) ListRepositories() []Repository {
	return m.Repos
}

// IsRepository checks if a directory is a DSP repository by checking its tracking state
func IsRepository(path string) bool {
	// Try to load repository config
	configPath := filepath.Join(path, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	// Parse config to get DSP directory
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return false
	}

	// Get DSP directory path from config
	dspDir := filepath.Join(path, cfg.DSPDir)

	// Try to load tracking config
	trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
	if err != nil {
		return false
	}

	// A valid repository should have a tracking config
	return trackingConfig != nil
}

// SetWorkingRepo sets the working repository
func (m *Manager) SetWorkingRepo(repoArg string) error {
	// First try to find repository by name
	for _, repo := range m.Repos {
		if repo.Name == repoArg {
			m.WorkingRepo = repo.Path
			return m.Save()
		}
	}

	// If not found by name, try as path
	absPath, err := filepath.Abs(repoArg)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Verify repository exists at this path
	if _, err := m.GetRepository(absPath); err != nil {
		// If not found, provide a helpful error message
		return fmt.Errorf("repository not found: '%s' (tried as both name and path). Use 'dsp repo list' to see available repositories", repoArg)
	}

	// Set as working repository
	m.WorkingRepo = absPath
	return m.Save()
}

// GetWorkingRepo gets the current working repository
func (m *Manager) GetWorkingRepo() (*Repository, error) {
	if m.WorkingRepo == "" {
		return nil, fmt.Errorf("no working repository set")
	}

	return m.GetRepository(m.WorkingRepo)
}

// ClearWorkingRepo clears the working repository
func (m *Manager) ClearWorkingRepo() error {
	m.WorkingRepo = ""
	return m.Save()
}

// GetDSPDir returns the DSP directory path for a repository
func (r *Repository) GetDSPDir() string {
	return filepath.Join(r.Path, r.DSPDir)
}

// reopenRepositoryTracking reopens a previously closed repository
func (m *Manager) reopenRepositoryTracking(repoPath string) error {
	// Get repository to get DSP directory
	repo, err := m.GetRepository(repoPath)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Get DSP directory path
	dspDir := repo.GetDSPDir()

	// Load tracking config
	trackingConfig, err := snapshot.LoadTrackingConfig(dspDir)
	if err != nil {
		return fmt.Errorf("failed to load tracking config: %w", err)
	}

	// If tracking config is empty (closed), create a new one
	if len(trackingConfig.Paths) == 0 {
		trackingConfig.Paths = []snapshot.TrackedPath{}
		if err := snapshot.SaveTrackingConfig(dspDir, trackingConfig); err != nil {
			return fmt.Errorf("failed to save tracking config: %w", err)
		}
	}

	return nil
}
