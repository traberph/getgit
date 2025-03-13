package load

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

const (
	// LoadFileName is the name of the load file in the tools directory
	LoadFileName = ".load"
	// LoadFileHeader is the header comment in the load file
	LoadFileHeader = `# This file is managed by getgit. Do not edit manually.
# It contains aliases for binary tools and source commands for non-binary tools.
`
)

// LoadError represents an error that occurred while processing the load file
type LoadError struct {
	Op  string
	Err error
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("load file error: %s: %v", e.Op, e.Err)
}

// LoadManager handles the .load file operations for managing tool aliases and source commands
type LoadManager struct {
	filePath string
	aliases  map[string]string // Maps tool name to binary path
	sources  map[string]string // Maps tool name to .getgit file path
	workDir  string            // Root directory for tools
}

// NewLoadManager creates a new load manager
func NewLoadManager() (*LoadManager, error) {
	filePath, err := config.GetAliasFile() // TODO: Rename to GetLoadFile in config package
	if err != nil {
		return nil, &LoadError{
			Op:  "init",
			Err: fmt.Errorf("failed to get load file path: %w", err),
		}
	}

	workDir, err := config.GetWorkDir()
	if err != nil {
		return nil, &LoadError{
			Op:  "init",
			Err: fmt.Errorf("failed to get work directory: %w", err),
		}
	}

	lm := &LoadManager{
		filePath: filePath,
		aliases:  make(map[string]string),
		sources:  make(map[string]string),
		workDir:  workDir,
	}

	// Load existing aliases and sources if file exists
	if err := lm.readFile(); err != nil {
		return nil, err
	}

	return lm, nil
}

// readFile reads the existing aliases and sources from the .load file
func (lm *LoadManager) readFile() error {
	file, err := os.Open(lm.filePath)
	if os.IsNotExist(err) {
		return nil // File doesn't exist yet, start fresh
	}
	if err != nil {
		return &LoadError{
			Op:  "read",
			Err: fmt.Errorf("failed to open load file: %w", err),
		}
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
			lm.aliases[name] = path
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

			lm.sources[toolName] = path
		}
	}

	if err := scanner.Err(); err != nil {
		return &LoadError{
			Op:  "read",
			Err: fmt.Errorf("failed to scan load file: %w", err),
		}
	}

	return nil
}

// processTemplate processes template variables in a load command
func (lm *LoadManager) processTemplate(loadCommand string) (string, error) {
	if !strings.Contains(loadCommand, "{{") {
		return loadCommand, nil
	}

	tmpl, err := template.New("load").Parse(loadCommand)
	if err != nil {
		return "", &LoadError{
			Op:  "template",
			Err: fmt.Errorf("failed to parse load command template: %w", err),
		}
	}

	data := struct {
		GetGit struct {
			Root string
		}
	}{
		GetGit: struct {
			Root string
		}{
			Root: lm.workDir,
		},
	}

	var processedCmd strings.Builder
	if err := tmpl.Execute(&processedCmd, data); err != nil {
		return "", &LoadError{
			Op:  "template",
			Err: fmt.Errorf("failed to process load command template: %w", err),
		}
	}

	return processedCmd.String(), nil
}

// AddAlias adds or updates an alias for a binary tool
func (lm *LoadManager) AddAlias(toolName, binaryPath string) error {
	lm.aliases[toolName] = binaryPath
	return lm.writeFile()
}

// AddSource adds a source line to the load file for a .getgit file
func (lm *LoadManager) AddSource(name, getgitFile string) error {
	// Read the .getgit file to get the load command
	gf, err := getgitfile.ReadFromRepo(filepath.Dir(getgitFile))
	if err != nil {
		return &LoadError{
			Op:  "source",
			Err: fmt.Errorf("failed to read .getgit file: %w", err),
		}
	}

	// Only add source if there's a load command
	if gf != nil && gf.LoadCommand != "" {
		// Process template to validate it
		if _, err := lm.processTemplate(gf.LoadCommand); err != nil {
			return err
		}

		lm.sources[name] = getgitFile
	}

	return lm.writeFile()
}

// RemoveTool removes both alias and source entries for a tool
func (lm *LoadManager) RemoveTool(toolName string) error {
	delete(lm.aliases, toolName)
	delete(lm.sources, toolName)
	return lm.writeFile()
}

// writeFile writes all aliases and sources to the .load file
func (lm *LoadManager) writeFile() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(lm.filePath), 0755); err != nil {
		return &LoadError{
			Op:  "save",
			Err: fmt.Errorf("failed to create load directory: %w", err),
		}
	}

	file, err := os.Create(lm.filePath)
	if err != nil {
		return &LoadError{
			Op:  "save",
			Err: fmt.Errorf("failed to create load file: %w", err),
		}
	}
	defer file.Close()

	// Write header
	fmt.Fprint(file, LoadFileHeader)
	fmt.Fprintln(file)

	// Write aliases sorted by name
	for name, path := range lm.aliases {
		fmt.Fprintf(file, "alias %s=\"%s\"\n", name, path)
	}

	// Write source lines
	for name, path := range lm.sources {
		fmt.Fprintf(file, "source \"%s\" # %s\n", path, name)
	}

	return nil
}

// GetAliases returns a copy of the current aliases map
func (lm *LoadManager) GetAliases() map[string]string {
	aliases := make(map[string]string)
	for k, v := range lm.aliases {
		aliases[k] = v
	}
	return aliases
}

// GetSources returns a copy of the current sources map
func (lm *LoadManager) GetSources() map[string]string {
	sources := make(map[string]string)
	for k, v := range lm.sources {
		sources[k] = v
	}
	return sources
}

// GetLoadFileContent returns the current content of the load file
func (lm *LoadManager) GetLoadFileContent() (string, error) {
	file, err := os.Open(lm.filePath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", &LoadError{
			Op:  "read",
			Err: fmt.Errorf("failed to open load file: %w", err),
		}
	}
	defer file.Close()

	content, err := os.ReadFile(lm.filePath)
	if err != nil {
		return "", &LoadError{
			Op:  "read",
			Err: fmt.Errorf("failed to read load file: %w", err),
		}
	}
	return string(content), nil
}

// GetLoadFilePath returns the path to the load file
func (lm *LoadManager) GetLoadFilePath() string {
	return lm.filePath
}

// EnsureLoadFile ensures the load file exists and has the correct header
func (lm *LoadManager) EnsureLoadFile() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(lm.filePath), 0755); err != nil {
		return &LoadError{
			Op:  "init",
			Err: fmt.Errorf("failed to create load directory: %w", err),
		}
	}

	// Check if file exists
	if _, err := os.Stat(lm.filePath); os.IsNotExist(err) {
		// Create new file with header
		return lm.writeFile()
	}

	// Read existing content
	content, err := lm.GetLoadFileContent()
	if err != nil {
		return err
	}

	// If file exists but doesn't have header, add it
	if !strings.Contains(content, LoadFileHeader) {
		return lm.writeFile()
	}

	return nil
}
