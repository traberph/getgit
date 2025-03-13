package repository

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GitOps handles all Git operations for a repository
type GitOps struct {
	repoPath string
	output   *OutputManager
}

// NewGitOps creates a new GitOps instance
func NewGitOps(repoPath string, output *OutputManager) *GitOps {
	return &GitOps{
		repoPath: repoPath,
		output:   output,
	}
}

// runCommand executes a Git command and returns its output
func (g *GitOps) runCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git command failed: %w - %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetDefaultBranch gets the default branch name from the repository
func (g *GitOps) GetDefaultBranch() (string, error) {
	// First try to get the symbolic ref of HEAD
	output, err := g.runCommand("symbolic-ref", "--short", "HEAD")
	if err == nil {
		return output, nil
	}

	// If that fails, try to get it from the remote
	output, err = g.runCommand("remote", "show", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to get remote info: %s", output)
	}

	// Parse the output to find the default branch
	lines := strings.Split(output, "\n")
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

// GetLatestTag returns the latest tag from the repository
func (g *GitOps) GetLatestTag() (string, error) {
	output, err := g.runCommand("describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", nil // No tags available
	}
	g.output.AddOutput(output)
	return output, nil
}

// GetCurrentTag gets the current tag of the repository
func (g *GitOps) GetCurrentTag() (string, error) {
	output, err := g.runCommand("describe", "--tags", "--exact-match")
	if err != nil {
		// No tag on current commit is not an error
		return "", nil
	}
	return output, nil
}

// GetCurrentRef returns the current git reference (commit hash or tag)
func (g *GitOps) GetCurrentRef() (string, error) {
	// First try to get tag
	output, err := g.runCommand("describe", "--tags", "--exact-match")
	if err == nil {
		return output, nil
	}

	// If no tag found, get commit hash
	output, err = g.runCommand("rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current ref: %s", output)
	}
	return output, nil
}

// FetchUpdates fetches updates from the remote repository
func (g *GitOps) FetchUpdates() error {
	// Make sure the repository directory exists
	if _, err := os.Stat(g.repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository directory does not exist: %s", g.repoPath)
	}

	// Use absolute path for the repository directory
	absPath, err := filepath.Abs(g.repoPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := exec.Command("git", "fetch", "--tags", "origin")
	cmd.Dir = absPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fetch updates: %w - %s", err, output)
	}
	g.output.AddOutput(string(output))
	return nil
}

// HasEdgeUpdates checks if there are new commits in the remote repository
func (g *GitOps) HasEdgeUpdates() (bool, error) {
	// Get the default branch name
	defaultBranch, err := g.GetDefaultBranch()
	if err != nil {
		return false, fmt.Errorf("failed to get default branch: %w", err)
	}

	// Get current and remote HEADs
	localHead, err := g.runCommand("rev-parse", "HEAD")
	if err != nil {
		return false, fmt.Errorf("failed to get local HEAD: %s", localHead)
	}

	remoteHead, err := g.runCommand("rev-parse", fmt.Sprintf("origin/%s", defaultBranch))
	if err != nil {
		return false, fmt.Errorf("failed to get remote HEAD: %s", remoteHead)
	}

	return localHead != remoteHead, nil
}

// IsTagNewer checks if newTag is newer than currentTag
func (g *GitOps) IsTagNewer(currentTag, newTag string) (bool, error) {
	// Get commit timestamps for both tags
	getTimestamp := func(tag string) (int64, error) {
		output, err := g.runCommand("log", "-1", "--format=%ct", tag)
		if err != nil {
			return 0, fmt.Errorf("failed to get timestamp for tag %s: %s", tag, output)
		}
		return strconv.ParseInt(output, 10, 64)
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

// HasTags checks if a repository has any tags
func (g *GitOps) HasTags() (bool, error) {
	output, err := g.runCommand("tag")
	if err != nil {
		return false, fmt.Errorf("failed to list tags: %s", output)
	}

	// If there's any output, there are tags
	return len(output) > 0, nil
}

// UpdateRepo updates the git repository based on useEdge flag
func (g *GitOps) UpdateRepo(useEdge bool) error {
	if useEdge {
		// Get default branch
		output, err := g.runCommand("symbolic-ref", "refs/remotes/origin/HEAD")
		if err != nil {
			// Fallback to main if we can't get the default branch
			defaultBranch := "main"
			_, err = g.runCommand("checkout", defaultBranch)
			if err != nil {
				return fmt.Errorf("failed to checkout default branch: %w", err)
			}
		} else {
			defaultBranch := strings.TrimSpace(output)
			defaultBranch = strings.TrimPrefix(defaultBranch, "refs/remotes/origin/")
			_, err = g.runCommand("checkout", defaultBranch)
			if err != nil {
				return fmt.Errorf("failed to checkout default branch: %w", err)
			}
		}

		// Pull latest changes
		_, err = g.runCommand("pull", "origin")
		if err != nil {
			return fmt.Errorf("failed to pull latest changes: %w", err)
		}
	} else {
		// Get latest tag
		_, err := g.runCommand("fetch", "--tags")
		if err != nil {
			return fmt.Errorf("failed to fetch tags: %w", err)
		}

		tag, err := g.GetLatestTag()
		if err != nil {
			return fmt.Errorf("no tags found: %s", err)
		}

		_, err = g.runCommand("checkout", tag)
		if err != nil {
			return fmt.Errorf("failed to checkout tag %s: %w", tag, err)
		}
	}
	return nil
}

// Clone clones a new repository
func (g *GitOps) Clone(repoURL string) error {
	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(g.repoPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// For clone, we need to run the command in the parent directory
	// The last part of g.repoPath will be the directory name for the clone
	repoName := filepath.Base(g.repoPath)
	cmd := exec.Command("git", "clone", repoURL, repoName)
	cmd.Dir = parentDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w - %s", err, output)
	}
	g.output.AddOutput(string(output))
	return nil
}

// CheckoutTag checks out a specific tag
func (g *GitOps) CheckoutTag(tag string) error {
	output, err := g.runCommand("checkout", tag)
	if err != nil {
		return fmt.Errorf("failed to checkout tag: %s", output)
	}
	g.output.AddOutput(output)
	return nil
}

// ListTags returns a list of all tags in the repository
func (g *GitOps) ListTags() ([]string, error) {
	cmd := exec.Command("git", "tag")
	cmd.Dir = g.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %s", output)
	}

	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(tags) == 1 && tags[0] == "" {
		return []string{}, nil
	}
	return tags, nil
}

// GetTagTimestamp returns the timestamp of a tag's commit
func (g *GitOps) GetTagTimestamp(tag string) (int64, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%ct", tag)
	cmd.Dir = g.repoPath
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
