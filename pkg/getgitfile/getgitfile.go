package getgitfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// GetGitFileName is the name of the metadata file in repositories
	GetGitFileName = ".getgit"
	// heredocStart is the marker for the start of a heredoc section
	heredocStart = ": <<'EOF'"
	// heredocEnd is the marker for the end of a heredoc section
	heredocEnd = "EOF"

	// UpdateTrainRelease represents the stable release update train
	UpdateTrainRelease = "release"
	// UpdateTrainEdge represents the bleeding edge update train
	UpdateTrainEdge = "edge"
)

// GetGitFileError represents an error that occurred while processing a .getgit file
type GetGitFileError struct {
	Op  string
	Err error
}

func (e *GetGitFileError) Error() string {
	return fmt.Sprintf("getgit file error: %s: %v", e.Op, e.Err)
}

// GetGitFile represents the contents of a .getgit file
type GetGitFile struct {
	SourceName  string `yaml:"sourcefile"` // Name of the source file that installed this tool
	UpdateTrain string `yaml:"updates"`    // "release" or "edge"
	Load        string `yaml:"load"`       // Shell commands to be executed
}

// Validate checks if the GetGitFile is valid
func (g *GetGitFile) Validate() error {
	if g.SourceName == "" {
		return &GetGitFileError{
			Op:  "validate",
			Err: fmt.Errorf("source name is empty"),
		}
	}
	if g.UpdateTrain != UpdateTrainRelease && g.UpdateTrain != UpdateTrainEdge {
		return &GetGitFileError{
			Op:  "validate",
			Err: fmt.Errorf("invalid update train: %s", g.UpdateTrain),
		}
	}
	return nil
}

// ReadFromRepo reads the .getgit file from a repository directory.
// It returns a GetGitFile struct containing the parsed contents and any error encountered.
// If the file doesn't exist, it returns nil, nil.
func ReadFromRepo(repoPath string) (*GetGitFile, error) {
	filePath := filepath.Join(repoPath, GetGitFileName)

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No .getgit file exists
		}
		return nil, &GetGitFileError{
			Op:  "read",
			Err: fmt.Errorf("failed to read .getgit file: %w", err),
		}
	}

	// Split content into lines
	lines := strings.Split(string(content), "\n")

	var yamlContent []string
	var loadCommands []string
	inHeredoc := false

	// Parse the file content
	for _, line := range lines {
		if strings.TrimSpace(line) == heredocStart {
			inHeredoc = true
			continue
		} else if strings.TrimSpace(line) == heredocEnd {
			inHeredoc = false
			continue
		}

		if inHeredoc {
			yamlContent = append(yamlContent, line)
		} else if line != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			loadCommands = append(loadCommands, line)
		}
	}

	var getgitFile GetGitFile
	if len(yamlContent) > 0 {
		if err := yaml.Unmarshal([]byte(strings.Join(yamlContent, "\n")), &getgitFile); err != nil {
			return nil, &GetGitFileError{
				Op:  "parse",
				Err: fmt.Errorf("invalid .getgit file YAML format: %w", err),
			}
		}
	}

	if err := getgitFile.Validate(); err != nil {
		return nil, err
	}

	// Store load commands
	getgitFile.Load = strings.Join(loadCommands, "\n")

	return &getgitFile, nil
}

// WriteToRepo writes the .getgit file to a repository directory.
// It takes the repository path, source name, update train, and load command as parameters.
// The update train must be either "release" or "edge", defaulting to "release" if invalid.
func WriteToRepo(repoPath string, sourceName string, updateTrain string, loadCommand string) error {
	filePath := filepath.Join(repoPath, GetGitFileName)

	// Validate update train
	if updateTrain != UpdateTrainRelease && updateTrain != UpdateTrainEdge {
		updateTrain = UpdateTrainRelease // Default to release if invalid
	}

	getgitFile := GetGitFile{
		SourceName:  sourceName,
		UpdateTrain: updateTrain,
		Load:        loadCommand,
	}

	if err := getgitFile.Validate(); err != nil {
		return err
	}

	// Marshal the YAML content
	yamlContent, err := yaml.Marshal(getgitFile)
	if err != nil {
		return &GetGitFileError{
			Op:  "marshal",
			Err: fmt.Errorf("failed to marshal .getgit file YAML: %w", err),
		}
	}

	// Build the complete file content
	var content strings.Builder
	content.WriteString("#!/bin/bash\n\n")
	content.WriteString(heredocStart + "\n")
	content.Write(yamlContent)
	content.WriteString(heredocEnd + "\n\n")
	content.WriteString(loadCommand + "\n")

	// Write the file with execute permissions
	if err := os.WriteFile(filePath, []byte(content.String()), 0755); err != nil {
		return &GetGitFileError{
			Op:  "write",
			Err: fmt.Errorf("failed to write .getgit file: %w", err),
		}
	}

	return nil
}
