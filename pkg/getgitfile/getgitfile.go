package getgitfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	GetGitFileName = ".getgit"
	heredocStart   = ": <<'EOF'"
	heredocEnd     = "EOF"
)

// GetGitFile represents the contents of a .getgit file
type GetGitFile struct {
	SourceName  string   `yaml:"sourcefile"` // Name of the source file that installed this tool
	UpdateTrain string   `yaml:"updates"`    // "release" or "edge"
	Collection  []string `yaml:"collection"` // List of collections this tool belongs to
	LoadCommand string   `yaml:"-"`          // Load command to be executed (not part of YAML)
}

// ReadFromRepo reads the .getgit file from a repository directory
func ReadFromRepo(repoPath string) (*GetGitFile, error) {
	filePath := filepath.Join(repoPath, GetGitFileName)

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No .getgit file exists
		}
		return nil, fmt.Errorf("failed to read .getgit file: %w", err)
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
			return nil, fmt.Errorf("invalid .getgit file YAML format: %w", err)
		}
	}

	if getgitFile.SourceName == "" {
		return nil, fmt.Errorf("invalid .getgit file: source name is empty")
	}

	// Validate update train
	if getgitFile.UpdateTrain != "release" && getgitFile.UpdateTrain != "edge" {
		getgitFile.UpdateTrain = "release" // Default to release if not specified
	}

	// Store load commands
	getgitFile.LoadCommand = strings.Join(loadCommands, "\n")

	return &getgitFile, nil
}

// WriteToRepo writes the .getgit file to a repository directory
func WriteToRepo(repoPath string, sourceName string, updateTrain string, collection []string, loadCommand string) error {
	filePath := filepath.Join(repoPath, GetGitFileName)

	// Validate update train
	if updateTrain != "release" && updateTrain != "edge" {
		updateTrain = "release" // Default to release if invalid
	}

	getgitFile := GetGitFile{
		SourceName:  sourceName,
		UpdateTrain: updateTrain,
		Collection:  collection,
	}

	// Marshal the YAML content
	yamlContent, err := yaml.Marshal(getgitFile)
	if err != nil {
		return fmt.Errorf("failed to marshal .getgit file YAML: %w", err)
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
		return fmt.Errorf("failed to write .getgit file: %w", err)
	}

	return nil
}
