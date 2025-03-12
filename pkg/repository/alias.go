package repository

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/getgitfile"
)

// AliasManager handles the .alias file operations
type AliasManager struct {
	aliasFile string
	aliases   map[string]string
	sources   map[string]string // Maps tool name to its .getgit file path
	workDir   string            // Root directory for tools
}

// NewAliasManager creates a new alias manager
func NewAliasManager() (*AliasManager, error) {
	aliasFile, err := config.GetAliasFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get alias file path: %w", err)
	}

	workDir, err := config.GetWorkDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get work directory: %w", err)
	}

	am := &AliasManager{
		aliasFile: aliasFile,
		aliases:   make(map[string]string),
		sources:   make(map[string]string),
		workDir:   workDir,
	}

	// Load existing aliases and sources if file exists
	if err := am.loadAliases(); err != nil {
		return nil, err
	}

	return am, nil
}

// loadAliases reads the existing aliases and sources from the .alias file
func (am *AliasManager) loadAliases() error {
	file, err := os.Open(am.aliasFile)
	if os.IsNotExist(err) {
		return nil // File doesn't exist yet, start fresh
	}
	if err != nil {
		return fmt.Errorf("failed to open alias file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse "alias name=/path/to/binary"
		if strings.HasPrefix(line, "alias ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "alias "), "=", 2)
			if len(parts) != 2 {
				continue
			}

			name := strings.TrimSpace(parts[0])
			path := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			am.aliases[name] = path
		}

		// Parse "source /path/to/.getgit"
		if strings.HasPrefix(line, "source ") {
			// Split the line into source command and comment
			parts := strings.SplitN(strings.TrimPrefix(line, "source "), "#", 2)
			path := strings.Trim(strings.TrimSpace(parts[0]), "\"'")

			// Get tool name from comment if available, otherwise from path
			var toolName string
			if len(parts) > 1 {
				toolName = strings.TrimSpace(parts[1])
			} else {
				toolName = filepath.Base(filepath.Dir(path))
			}

			am.sources[toolName] = path
		}
	}

	return scanner.Err()
}

// AddAlias adds or updates an alias for a tool
func (am *AliasManager) AddAlias(toolName, binaryPath string) error {
	am.aliases[toolName] = binaryPath
	return am.saveAliases()
}

// AddSource adds a source line to the alias file for a .getgit file
func (am *AliasManager) AddSource(name, getgitFile string) error {
	// Read the .getgit file to get the load command
	gf, err := getgitfile.ReadFromRepo(filepath.Dir(getgitFile))
	if err != nil {
		return fmt.Errorf("failed to read .getgit file: %w", err)
	}

	// Only add source if there's a load command
	if gf != nil && gf.LoadCommand != "" {
		loadCommand := gf.LoadCommand

		// Only process template if it contains template variables
		if strings.Contains(loadCommand, "{{") {
			// Process template variables in the load command
			tmpl, err := template.New("load").Parse(loadCommand)
			if err != nil {
				return fmt.Errorf("failed to parse load command template: %w", err)
			}

			data := struct {
				GetGit struct {
					Root string
				}
			}{
				GetGit: struct {
					Root string
				}{
					Root: am.workDir,
				},
			}

			var processedCmd strings.Builder
			if err := tmpl.Execute(&processedCmd, data); err != nil {
				return fmt.Errorf("failed to process load command template: %w", err)
			}

			loadCommand = processedCmd.String()
		}

		am.sources[name] = getgitFile
	}

	return am.saveAliases()
}

// RemoveAlias removes an alias for a tool
func (am *AliasManager) RemoveAlias(toolName string) error {
	delete(am.aliases, toolName)
	delete(am.sources, toolName)
	return am.saveAliases()
}

// saveAliases writes all aliases and sources to the .alias file
func (am *AliasManager) saveAliases() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(am.aliasFile), 0755); err != nil {
		return fmt.Errorf("failed to create alias directory: %w", err)
	}

	file, err := os.Create(am.aliasFile)
	if err != nil {
		return fmt.Errorf("failed to create alias file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintln(file, "# This file is managed by getgit. Do not edit manually.")
	fmt.Fprintln(file, "# It contains aliases for installed tools.")
	fmt.Fprintln(file)

	// Write aliases sorted by name
	for name, path := range am.aliases {
		fmt.Fprintf(file, "alias %s=\"%s\"\n", name, path)
	}

	// Write source lines
	for name, path := range am.sources {
		fmt.Fprintf(file, "source \"%s\" # %s\n", path, name)
	}

	return nil
}

// GetAliases returns a copy of the current aliases map
func (am *AliasManager) GetAliases() map[string]string {
	aliases := make(map[string]string)
	for k, v := range am.aliases {
		aliases[k] = v
	}
	return aliases
}

// GetSources returns a copy of the current sources map
func (am *AliasManager) GetSources() map[string]string {
	sources := make(map[string]string)
	for k, v := range am.sources {
		sources[k] = v
	}
	return sources
}
