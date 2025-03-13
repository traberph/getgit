package getgitfile

import (
	"os/exec"
	"path/filepath"
)

// Manager provides operations for managing .getgit files
type Manager struct {
	workDir string
}

// NewManager creates a new Manager instance
func NewManager(workDir string) *Manager {
	if workDir == "" {
		return nil
	}
	return &Manager{
		workDir: workDir,
	}
}

// Read reads the .getgit file for a tool
func (m *Manager) Read(toolName string) (*GetGitFile, error) {
	repoPath := filepath.Join(m.workDir, toolName)
	return ReadFromRepo(repoPath)
}

// Write writes the .getgit file for a tool
func (m *Manager) Write(toolName string, sourceName, updateTrain, load string) error {
	repoPath := filepath.Join(m.workDir, toolName)
	return WriteToRepo(repoPath, sourceName, updateTrain, load)
}

// GetFilePath returns the full path to the .getgit file for a tool
func (m *Manager) GetFilePath(toolName string) string {
	repoPath := filepath.Join(m.workDir, toolName)
	return filepath.Join(repoPath, GetGitFileName)
}

// GetUpdateTrain determines which update train to use based on flags and existing .getgit file
func (m *Manager) GetUpdateTrain(toolName string, edge, release bool) (string, bool) {
	// If flags are specified, they take precedence
	if edge {
		return UpdateTrainEdge, false
	}
	if release {
		// Check if repository has any tags when release is explicitly requested
		if toolName != "" {
			hasTags, err := m.HasTags(toolName)
			if err == nil && !hasTags {
				// Fall back to edge if no tags exist
				return UpdateTrainEdge, true
			}
		}
		return UpdateTrainRelease, false
	}

	// If .getgit file exists, use its preference
	getgitFile, err := m.Read(toolName)
	if err == nil && getgitFile != nil {
		return getgitFile.UpdateTrain, getgitFile.UpdateTrain == UpdateTrainEdge
	}

	// Check if repository has any tags for default behavior
	if toolName != "" {
		hasTags, err := m.HasTags(toolName)
		if err == nil && !hasTags {
			return UpdateTrainEdge, true
		}
	}

	// Default to release
	return UpdateTrainRelease, false
}

// HasTags checks if a repository has any tags
func (m *Manager) HasTags(toolName string) (bool, error) {
	toolDir := filepath.Join(m.workDir, toolName)
	cmd := exec.Command("git", "tag", "-l")
	cmd.Dir = toolDir
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(output) > 0, nil
}
