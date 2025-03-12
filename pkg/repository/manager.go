package repository

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/getgitfile"
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

// StartStage starts a new stage with the given message
func (om *OutputManager) StartStage(message string) {
	if !om.verbose {
		om.spinner.Suffix = fmt.Sprintf(" %s", message)
		om.spinner.Start()
	} else {
		fmt.Fprintf(os.Stderr, "==> %s\n", message)
	}
}

// CompleteStage marks the current stage as completed
func (om *OutputManager) CompleteStage() {
	if !om.verbose {
		om.spinner.Stop()
	}
}

// StopStage stops the current stage without printing completion
func (om *OutputManager) StopStage() {
	if !om.verbose {
		om.spinner.Stop()
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

// Manager handles Git repository operations and tool management
type Manager struct {
	workDir string
	Output  *OutputManager
	aliases *AliasManager
}

// NewManager creates a new repository manager instance
func NewManager(workDir string, verbose bool) (*Manager, error) {
	// Load the config to get the base folder
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Use the configured root directory
	workDir = cfg.Root

	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	// Create alias manager
	aliasManager, err := NewAliasManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create alias manager: %w", err)
	}

	return &Manager{
		workDir: workDir,
		Output:  NewOutputManager(verbose),
		aliases: aliasManager,
	}, nil
}

// getDefaultBranch gets the default branch name from the repository
func (m *Manager) getDefaultBranch(repoPath string) (string, error) {
	// First try to get the symbolic ref of HEAD
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	// If that fails, try to get it from the remote
	cmd = exec.Command("git", "remote", "show", "origin")
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get remote info: %s", output)
	}

	// Parse the output to find the default branch
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "HEAD branch:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "main", nil // Default to main if we can't determine it
}

// CloneOrUpdate either clones a new repository or updates an existing one
func (m *Manager) CloneOrUpdate(repoURL, name string) (string, error) {
	repoPath := filepath.Join(m.workDir, name)

	// Check if repository already exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		// Repository exists, update it
		m.Output.StartStage("Updating repository")

		// Get current branch or tag
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = repoPath
		output, err := cmd.CombinedOutput()
		currentRef := strings.TrimSpace(string(output))
		isDetached := currentRef == "HEAD"

		if isDetached {
			// We're in detached HEAD state (probably on a tag)
			// Fetch updates
			cmd = exec.Command("git", "fetch", "origin")
			cmd.Dir = repoPath
			output, err = cmd.CombinedOutput()
			if err != nil {
				m.Output.StopStage()
				return "", fmt.Errorf("failed to fetch updates: %s", output)
			}
			m.Output.AddOutput(string(output))
		} else {
			// We're on a branch, get the default branch name
			defaultBranch, err := m.getDefaultBranch(repoPath)
			if err != nil {
				defaultBranch = "main" // Fallback to main
			}

			// Pull updates from the default branch
			cmd = exec.Command("git", "pull", "origin", defaultBranch)
			cmd.Dir = repoPath
			output, err = cmd.CombinedOutput()
			if err != nil {
				m.Output.StopStage()
				return "", fmt.Errorf("failed to update repository: %s", output)
			}
			m.Output.AddOutput(string(output))
		}

		m.Output.CompleteStage()
		return repoPath, nil
	}

	// Repository doesn't exist, clone it
	m.Output.StartStage("Cloning repository")
	cmd := exec.Command("git", "clone", repoURL, repoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.Output.StopStage()
		return "", fmt.Errorf("failed to clone repository: %s", output)
	}
	m.Output.AddOutput(string(output))
	m.Output.CompleteStage()

	return repoPath, nil
}

// GetLatestTag returns the latest tag from the repository
func (m *Manager) GetLatestTag(repoPath string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", nil // No tags available
	}
	m.Output.AddOutput(string(output))
	return strings.TrimSpace(string(output)), nil
}

// CheckoutTag checks out a specific tag
func (m *Manager) CheckoutTag(repoPath, tag string) error {
	cmd := exec.Command("git", "checkout", tag)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to checkout tag: %s", output)
	}
	m.Output.AddOutput(string(output))
	return nil
}

// UpdatePackage updates a specific tool
func (m *Manager) UpdatePackage(repo Repository) error {
	// Clone or update the repository
	repoPath, err := m.CloneOrUpdate(repo.URL, repo.Name)
	if err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}

	if !repo.UseEdge {
		// Get latest tag if available and not in edge mode
		tag, err := m.GetLatestTag(repoPath)
		if err == nil && tag != "" {
			if err := m.CheckoutTag(repoPath, tag); err != nil {
				return fmt.Errorf("failed to checkout tag: %w", err)
			}
		}
	}

	// Only run build if a build command is specified and not skipped
	if repo.Build != "" && !repo.SkipBuild {
		m.Output.StartStage("Building")
		cmd := exec.Command("bash", "-c", repo.Build)
		cmd.Dir = repoPath
		output, err := cmd.CombinedOutput()
		if err != nil {
			m.Output.StopStage()
			return fmt.Errorf("build failed: %s", output)
		}
		m.Output.AddOutput(string(output))
		m.Output.CompleteStage()
	}

	// Handle load command if specified
	if repo.Load != "" {
		// Replace template variables
		loadCmd := repo.Load
		loadCmd = strings.ReplaceAll(loadCmd, "{{ .getgit.root }}", m.workDir)

		// Create or update .getgit file
		var collection []string
		if repo.UseEdge {
			collection = []string{"edge"}
		}

		if err := getgitfile.WriteToRepo(repoPath, repo.SourceName,
			map[bool]string{true: "edge", false: "release"}[repo.UseEdge],
			collection,
			loadCmd); err != nil {
			return fmt.Errorf("failed to write .getgit file: %w", err)
		}

		// Add source line to .alias file
		getgitFile := filepath.Join(repoPath, getgitfile.GetGitFileName)
		if err := m.aliases.AddSource(repo.Name, getgitFile); err != nil {
			return fmt.Errorf("failed to add source to .alias file: %w", err)
		}
	}

	// Create alias for the executable only if an executable is specified
	if repo.Executable != "" {
		execPath := filepath.Join(repoPath, repo.Executable)
		if err := m.aliases.AddAlias(repo.Name, execPath); err != nil {
			return fmt.Errorf("failed to create alias: %w", err)
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
	m.Output.StartStage("Fetching updates")
	cmd := exec.Command("git", "fetch", "--tags", "origin")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.Output.StopStage()
		return fmt.Errorf("failed to fetch updates: %s", output)
	}
	m.Output.AddOutput(string(output))
	m.Output.CompleteStage()
	return nil
}

// HasEdgeUpdates checks if there are new commits in the remote repository
func (m *Manager) HasEdgeUpdates(repoPath string) (bool, error) {
	// Get the default branch name
	defaultBranch, err := m.getDefaultBranch(repoPath)
	if err != nil {
		return false, fmt.Errorf("failed to get default branch: %w", err)
	}

	// Get current and remote HEADs
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	localHead, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to get local HEAD: %s", localHead)
	}

	cmd = exec.Command("git", "rev-parse", fmt.Sprintf("origin/%s", defaultBranch))
	cmd.Dir = repoPath
	remoteHead, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to get remote HEAD: %s", remoteHead)
	}

	return strings.TrimSpace(string(localHead)) != strings.TrimSpace(string(remoteHead)), nil
}

// GetCurrentTag gets the current tag of the repository
func (m *Manager) GetCurrentTag(repoPath string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--exact-match")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		// No tag on current commit is not an error
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}

// IsTagNewer checks if newTag is newer than currentTag
func (m *Manager) IsTagNewer(repoPath, currentTag, newTag string) (bool, error) {
	// Get commit timestamps for both tags
	getTimestamp := func(tag string) (int64, error) {
		cmd := exec.Command("git", "log", "-1", "--format=%ct", tag)
		cmd.Dir = repoPath
		output, err := cmd.CombinedOutput()
		if err != nil {
			return 0, fmt.Errorf("failed to get timestamp for tag %s: %s", tag, output)
		}
		timestamp, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse timestamp for tag %s: %w", tag, err)
		}
		return timestamp, nil
	}

	currentTime, err := getTimestamp(currentTag)
	if err != nil {
		return false, err
	}

	newTime, err := getTimestamp(newTag)
	if err != nil {
		return false, err
	}

	return newTime > currentTime, nil
}
