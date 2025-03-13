package repository

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/getgitfile"
	"github.com/traberph/getgit/pkg/load"
	"github.com/traberph/getgit/pkg/sources"
)

const (
	colorGreen  = "\033[32m"
	colorOrange = "\033[31m"
	colorReset  = "\033[0m"
)

// OutputManager handles command output and progress indication
type OutputManager struct {
	spinner *spinner.Spinner
	verbose bool
	mu      sync.Mutex
}

// NewOutputManager creates a new output manager with a spinner
func NewOutputManager(verbose bool) *OutputManager {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = os.Stderr // Use stderr for spinner to avoid mixing with command output
	return &OutputManager{
		spinner: s,
		verbose: verbose,
	}
}

// IsVerbose returns the current verbose setting
func (om *OutputManager) IsVerbose() bool {
	om.mu.Lock()
	defer om.mu.Unlock()
	return om.verbose
}

// SetVerbose sets the verbose mode
func (om *OutputManager) SetVerbose(verbose bool) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.verbose = verbose
}

// StartStage starts a new stage with the given message
func (om *OutputManager) StartStage(message string) {
	om.mu.Lock()
	defer om.mu.Unlock()

	if !om.verbose {
		if om.spinner.Active() {
			om.spinner.Stop()
		}
		om.spinner.Suffix = fmt.Sprintf(" %s", message)
		om.spinner.Start()
	} else {
		fmt.Fprintf(os.Stderr, "==> %s\n", message)
	}
}

// CompleteStage marks the current stage as completed
func (om *OutputManager) CompleteStage() {
	om.mu.Lock()
	defer om.mu.Unlock()

	if !om.verbose {
		if om.spinner.Active() {
			om.spinner.Stop()
		}
	}
}

// StopStage stops the current stage without printing completion
func (om *OutputManager) StopStage() {
	om.mu.Lock()
	defer om.mu.Unlock()

	if !om.verbose {
		if om.spinner.Active() {
			om.spinner.Stop()
		}
		// Clear the line
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
}

// AddOutput adds output to the buffer
func (om *OutputManager) AddOutput(output string) {
	if om.verbose {
		fmt.Fprint(os.Stderr, output)
	}
}

// PrintStatus prints a status message with a checkmark
func (om *OutputManager) PrintStatus(message string) {
	if !om.verbose {
		fmt.Fprintf(os.Stderr, "\r\033[K") // Clear the line first
	}
	fmt.Fprintf(os.Stderr, "✓ %s\n", message)
}

// PrintError prints an error message
func (om *OutputManager) PrintError(message string) {
	if !om.verbose {
		fmt.Fprintf(os.Stderr, "\r\033[K") // Clear the line first
	}
	fmt.Fprintf(os.Stderr, "✗ %s\n", message)
}

// PrintInfo prints an informational message
func (om *OutputManager) PrintInfo(message string) {
	if !om.verbose {
		fmt.Fprintf(os.Stderr, "\r\033[K") // Clear the line first
	}
	fmt.Fprintf(os.Stderr, "%s\n", message)
}

// ManagerError represents an error that occurred in the repository manager
type ManagerError struct {
	Op  string
	Err error
}

func (e *ManagerError) Error() string {
	return fmt.Sprintf("manager error: %s: %v", e.Op, e.Err)
}

// Manager handles Git repository operations and tool management
type Manager struct {
	workDir string
	Output  *OutputManager
	load    *load.LoadManager
	Getgit  *getgitfile.Manager // Expose getgitfile manager
}

// NewManager creates a new repository manager instance
func NewManager(workDir string, verbose bool) (*Manager, error) {
	// Load the config to get the base folder
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, &ManagerError{
			Op:  "init",
			Err: fmt.Errorf("failed to load config: %w", err),
		}
	}

	// Use workDir if provided, otherwise use config root
	if workDir == "" {
		workDir = cfg.Root
	}

	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, &ManagerError{
			Op:  "init",
			Err: fmt.Errorf("failed to create work directory: %w", err),
		}
	}

	// Create load manager
	loadManager, err := load.NewLoadManager()
	if err != nil {
		return nil, &ManagerError{
			Op:  "init",
			Err: fmt.Errorf("failed to create load manager: %w", err),
		}
	}

	// Create getgitfile manager
	getgitManager, err := getgitfile.NewManager(workDir)
	if err != nil {
		return nil, &ManagerError{
			Op:  "init",
			Err: fmt.Errorf("failed to create getgitfile manager: %w", err),
		}
	}

	// Ensure load file exists with correct header
	if err := loadManager.EnsureLoadFile(); err != nil {
		return nil, &ManagerError{
			Op:  "init",
			Err: fmt.Errorf("failed to ensure load file: %w", err),
		}
	}

	return &Manager{
		workDir: workDir,
		Output:  NewOutputManager(verbose),
		load:    loadManager,
		Getgit:  getgitManager,
	}, nil
}

// CloneOrUpdate either clones a new repository or updates an existing one
func (m *Manager) CloneOrUpdate(repoURL, name string) (string, error) {
	repoPath := filepath.Join(m.workDir, name)
	gitOps := NewGitOps(repoPath, m.Output)

	// Check if repository already exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		// Repository exists, update it
		if err := gitOps.FetchUpdates(); err != nil {
			return "", err
		}

		// Get current ref
		currentRef, err := gitOps.GetCurrentRef()
		if err != nil {
			return "", fmt.Errorf("failed to get current ref: %w", err)
		}

		// Check if we're in detached HEAD state
		isDetached := currentRef == "HEAD"

		if isDetached {
			// We're in detached HEAD state (probably on a tag)
			// No need to pull, as we'll switch to the appropriate tag later
			return repoPath, nil
		}

		// We're on a branch, check for updates
		hasUpdates, err := gitOps.HasEdgeUpdates()
		if err != nil {
			return "", fmt.Errorf("failed to check for updates: %w", err)
		}

		if hasUpdates {
			if err := gitOps.UpdateRepo(true); err != nil {
				return "", fmt.Errorf("failed to update repository: %w", err)
			}
		}

		return repoPath, nil
	}

	// Repository doesn't exist, clone it
	if err := gitOps.Clone(repoURL); err != nil {
		return "", err
	}

	return repoPath, nil
}

// UpdatePackage updates a specific tool
func (m *Manager) UpdatePackage(repo Repository) error {
	// Start spinner only if not in verbose mode and not already running
	if !m.Output.IsVerbose() && !m.Output.IsSpinnerRunning() {
		m.Output.StartStage("Checking for updates...")
	}

	// Get the repository path
	repoPath := filepath.Join(m.workDir, repo.Name)
	gitOps := NewGitOps(repoPath, m.Output)

	// Check if repository exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return &ManagerError{
			Op:  "update",
			Err: fmt.Errorf("repository not found at %s", repoPath),
		}
	}

	// Get current state
	currentRef, err := m.GetRepoState(repoPath)
	if err != nil {
		return &ManagerError{
			Op:  "update",
			Err: fmt.Errorf("failed to get current state: %w", err),
		}
	}

	// Update repository based on update train
	m.Output.StartStage("Updating repository...")
	if err := gitOps.UpdateRepo(repo.UseEdge); err != nil {
		m.Output.StopStage()
		return &ManagerError{
			Op:  "update",
			Err: fmt.Errorf("failed to update repository: %w", err),
		}
	}

	// Get new state
	newRef, err := m.GetRepoState(repoPath)
	if err != nil {
		return &ManagerError{
			Op:  "update",
			Err: fmt.Errorf("failed to get new state: %w", err),
		}
	}

	// If refs are different, we need to rebuild
	if currentRef != newRef {
		if repo.UseEdge {
			m.Output.PrintStatus(fmt.Sprintf("Repository updated to latest commit: %s", newRef))
		} else {
			// For release mode, verify we're on a tag
			tag, err := gitOps.GetCurrentTag()
			if err != nil {
				return &ManagerError{
					Op:  "update",
					Err: fmt.Errorf("failed to get current tag: %w", err),
				}
			}
			m.Output.PrintStatus(fmt.Sprintf("Repository updated to tag: %s", tag))
		}

		if !repo.SkipBuild {
			m.Output.StartStage(fmt.Sprintf("Building %s...", repo.Name))
			if err := m.buildTool(repo); err != nil {
				m.Output.StopStage()
				return &ManagerError{
					Op:  "build",
					Err: fmt.Errorf("failed to build tool: %w", err),
				}
			}
			m.Output.PrintStatus("Build completed")
		}
	} else {
		m.Output.StopStage()
		m.Output.PrintInfo(fmt.Sprintf("Tool '%s' is already up to date!", repo.Name))
	}

	// Create or update alias for the tool
	if repo.Executable != "" {
		m.Output.StartStage("Updating alias...")
		execPath := filepath.Join(repoPath, repo.Executable)
		if err := m.load.AddAlias(repo.Name, execPath); err != nil {
			m.Output.StopStage()
			return &ManagerError{
				Op:  "alias",
				Err: fmt.Errorf("failed to create alias: %w", err),
			}
		}
		m.Output.PrintStatus("Updated alias")
	}

	// Add source command if tool has a .getgit file
	getgitPath := m.Getgit.GetFilePath(repo.Name)
	if err := m.load.AddSource(repo.Name, getgitPath); err != nil {
		return &ManagerError{
			Op:  "source",
			Err: fmt.Errorf("failed to add source command: %w", err),
		}
	}

	return nil
}

// Repository represents a tool repository configuration
type Repository struct {
	Name       string
	URL        string
	Build      string
	Executable string
	Load       string // Load command to be executed
	UseEdge    bool   // When true, use latest commit instead of latest tag
	SkipBuild  bool   // When true, skip the build step
	SourceName string
}

// FetchUpdates fetches updates from the remote repository
func (m *Manager) FetchUpdates(repoPath string) error {
	gitOps := NewGitOps(repoPath, m.Output)
	return gitOps.FetchUpdates()
}

// GetRepoState gets the current state of the repository (tag or commit hash)
func (m *Manager) GetRepoState(repoPath string) (string, error) {
	gitOps := NewGitOps(repoPath, m.Output)
	return gitOps.GetCurrentRef()
}

// GetTagInfo gets information about tags in the repository
func (m *Manager) GetTagInfo(repoPath string) (hasTags bool, currentTag string, err error) {
	gitOps := NewGitOps(repoPath, m.Output)

	// Check for tags
	hasTags, err = gitOps.HasTags()
	if err != nil {
		return false, "", &ManagerError{
			Op:  "tags",
			Err: fmt.Errorf("failed to list tags: %w", err),
		}
	}

	// Get current tag if any
	if hasTags {
		currentTag, err = gitOps.GetCurrentTag()
		if err != nil {
			// Not on a tag, that's okay
			currentTag = ""
		}
	}

	return hasTags, currentTag, nil
}

// IsTagNewer checks if newTag is newer than currentTag
func (m *Manager) IsTagNewer(repoPath, currentTag, newTag string) (bool, error) {
	gitOps := NewGitOps(repoPath, m.Output)
	return gitOps.IsTagNewer(currentTag, newTag)
}

// Update the HasTags method to use GitOps directly
func (m *Manager) HasTags(repoPath string) (bool, error) {
	gitOps := NewGitOps(repoPath, m.Output)
	return gitOps.HasTags()
}

func (o *OutputManager) IsSpinnerRunning() bool {
	return o.spinner != nil && o.spinner.Active()
}

// buildTool builds the tool using the specified build command
func (m *Manager) buildTool(repo Repository) error {
	cmd := exec.Command("bash", "-c", repo.Build)
	cmd.Dir = filepath.Join(m.workDir, repo.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.Output.StopStage()
		return fmt.Errorf("build failed: %s", output)
	}
	m.Output.AddOutput(string(output))
	return nil
}

// GetUpdateTrain determines which update train to use based on flags and existing .getgit file
func (m *Manager) GetUpdateTrain(getgitFile *getgitfile.GetGitFile, toolName string, useEdge, useRelease bool) (string, bool) {
	return m.Getgit.GetUpdateTrain(toolName, useEdge, useRelease)
}

// IsToolInstalled checks if a tool is already installed
func (m *Manager) IsToolInstalled(toolName string) (bool, error) {
	repoPath := filepath.Join(m.workDir, toolName)
	_, err := os.Stat(repoPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, &ManagerError{
			Op:  "check",
			Err: fmt.Errorf("failed to check tool installation: %w", err),
		}
	}
	return true, nil
}

// GetToolConfig gets the configuration for a tool
func (m *Manager) GetToolConfig(toolName string) (*getgitfile.GetGitFile, error) {
	return m.Getgit.ReadConfig(toolName)
}

// WriteToolConfig writes the configuration for a tool
func (m *Manager) WriteToolConfig(toolName, sourceName, updateTrain, loadCommand string) error {
	return m.Getgit.WriteConfig(toolName, sourceName, updateTrain, loadCommand)
}

// RepoStatus represents the current status of a repository
type RepoStatus struct {
	sources.RepoInfo
	Installed   bool
	UpdateTrain string
	InstallPath string
}

// GetRepoStatus returns the current status of a repository
func (rm *Manager) GetRepoStatus(repo sources.RepoInfo) RepoStatus {
	status := RepoStatus{
		RepoInfo: repo,
	}

	// Check if tool is installed
	repoPath := filepath.Join(rm.workDir, repo.Name)
	if _, err := os.Stat(repoPath); err == nil {
		status.Installed = true
		status.InstallPath = repoPath

		// Check for .getgit file
		if getgitFile, err := getgitfile.ReadFromRepo(repoPath); err == nil && getgitFile != nil {
			status.UpdateTrain = getgitFile.UpdateTrain
		} else {
			status.UpdateTrain = "release" // Default to release if no .getgit file
		}
	}

	return status
}

// PrintRepoInfo prints repository information to the given writer
func (rm *Manager) PrintRepoInfo(w *tabwriter.Writer, repo RepoStatus, verbose, veryVerbose bool) {
	// Helper function to format multi-line values
	formatMultiLine := func(value string) string {
		if strings.Contains(value, "\n") {
			lines := strings.Split(strings.TrimSpace(value), "\n")
			// First line goes after the tab, subsequent lines are indented
			for i := range lines {
				lines[i] = strings.TrimSpace(lines[i])
			}
			return lines[0] + "\n\t" + strings.Join(lines[1:], "\n\t")
		}
		return strings.TrimSpace(value)
	}

	// Basic info always shown
	fmt.Fprintf(w, "name:\t%s\n", repo.Name)
	fmt.Fprintf(w, "repository url:\t%s\n", repo.URL)
	if repo.Installed {
		fmt.Fprintf(w, "status:\t%sinstalled%s\n", colorGreen, colorReset)
	} else {
		fmt.Fprintf(w, "status:\tnot installed\n")
	}

	// Additional info with -v
	if verbose || veryVerbose {
		if repo.Installed {
			if repo.UpdateTrain == "edge" {
				fmt.Fprintf(w, "update train:\t%sedge%s\n", colorOrange, colorReset)
			} else {
				fmt.Fprintf(w, "update train:\t%s\n", repo.UpdateTrain)
			}
		}
		fmt.Fprintf(w, "source name:\t%s\n", repo.SourceName)
	}

	// Full info with -V
	if veryVerbose {
		if repo.Build != "" {
			fmt.Fprintf(w, "build command:\t%s\n", formatMultiLine(repo.Build))
		}
		if repo.Executable != "" {
			fmt.Fprintf(w, "executable:\t%s\n", formatMultiLine(repo.Executable))
		}
		fmt.Fprintf(w, "source file:\t%s\n", repo.SourceFile)
		if repo.Installed {
			fmt.Fprintf(w, "install path:\t%s\n", formatMultiLine(repo.InstallPath))
		}
		if repo.Load != "" {
			fmt.Fprintf(w, "load command:\t%s\n", formatMultiLine(repo.Load))
		}
	}
}

// GetUniqueRepos returns a map of unique repositories based on installation status
func (rm *Manager) GetUniqueRepos(repos []sources.RepoInfo, installedOnly bool) map[string]RepoStatus {
	uniqueTools := make(map[string]RepoStatus)

	for _, repo := range repos {
		status := rm.GetRepoStatus(repo)

		// If tool is installed and there's a conflict
		if status.Installed {
			if existing, exists := uniqueTools[repo.Name]; exists {
				// Check .getgit file to determine which one is actually installed
				repoPath := filepath.Join(rm.workDir, repo.Name)
				if getgitFile, err := getgitfile.ReadFromRepo(repoPath); err == nil && getgitFile != nil {
					// If .getgit file exists, use the source specified in it
					if getgitFile.SourceName == repo.SourceName {
						uniqueTools[repo.Name] = status
					} else if getgitFile.SourceName == existing.SourceName {
						// Keep existing if it matches .getgit file
						continue
					}
				}
			} else {
				uniqueTools[repo.Name] = status
			}
		} else if !installedOnly {
			// For non-installed tools, only add if not showing installed only
			if _, exists := uniqueTools[repo.Name]; !exists {
				uniqueTools[repo.Name] = status
			}
		}
	}

	return uniqueTools
}

// Close closes the repository manager and cleans up resources
func (rm *Manager) Close() error {
	return nil // No cleanup needed at the moment
}
