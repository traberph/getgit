package sources

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/traberph/getgit/pkg/config"
	"gopkg.in/yaml.v3"
)

const (
	colorGreen  = "\033[32m"
	colorOrange = "\033[31m"
	colorReset  = "\033[0m"
)

// Repository represents a single repository configuration
type Repository struct {
	Name       string `yaml:"name"`
	URL        string `yaml:"url"`                  // Git repository URL
	Build      string `yaml:"build"`                // Build command
	Executable string `yaml:"executable,omitempty"` // Path to the executable after build
	Load       string `yaml:"load"`                 // Load command
}

// Permission defines allowed commands and origins for a source
type Permission struct {
	Origins []string `yaml:"origins,omitempty"` // Allowed repository origins
}

// SourceData represents the YAML configuration data for a source
type SourceData struct {
	Name        string       `yaml:"name"`
	Origin      string       `yaml:"origin"`      // URL where the source file is hosted
	Permissions []Permission `yaml:"permissions"` // Security permissions
	Repos       []Repository `yaml:"repos"`
}

// Source represents a source configuration file and implements SourceInterface
type Source struct {
	data       SourceData
	filePath   string // Internal use to track source file
	newContent []byte // Internal use to store new content for later use
}

// SourceChanges represents different types of changes in a source
type SourceChanges struct {
	IdentityChanges     []string // Changes to name or origin
	PermissionChanges   []string // Changes to permissions
	RepositoryChanges   []string // Changes to repositories
	RequiredPermissions []string // New permissions that need approval
}

// SourceManager provides operations for managing tool sources.
// It handles loading, updating, and validating source configurations
// as well as finding and validating repositories.
type SourceManager struct {
	configDir string
	Sources   []SourceInterface
	db        *sql.DB
}

// RepoMatch represents a repository match with its source
type RepoMatch struct {
	Repo   Repository
	Source Source
}

// RepoInfo represents repository information stored in the index
type RepoInfo struct {
	Name       string
	URL        string
	Build      string
	Executable string
	SourceFile string
	SourceName string
	Load       string
}

// SourceInterface represents a source of tools
type SourceInterface interface {
	// FindRepo finds a repository by name
	FindRepo(name string) []RepoMatch
	// ValidateURLHost validates the host of a URL
	ValidateURLHost(url string) error
	// GetName returns the name of the source
	GetName() string
	// GetOrigin returns the origin URL of the source
	GetOrigin() string
	// GetRepos returns the repositories of the source
	GetRepos() []Repository
}

// BaseSource implements common SourceInterface functionality
type BaseSource struct {
	Name string
}

// GetName returns the name of the source
func (s *BaseSource) GetName() string {
	return s.Name
}

// getDBPath returns the path to the index database
func getDBPath() (string, error) {
	cacheDir, err := config.GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "index.db"), nil
}

// NewSourceManager creates a new source manager instance
func NewSourceManager() (*SourceManager, error) {
	sourcesDir, err := config.GetSourcesDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get sources directory: %w", err)
	}

	if err := os.MkdirAll(sourcesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sources directory: %w", err)
	}

	// Initialize database
	dbPath, err := getDBPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get database path: %w", err)
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	manager := &SourceManager{
		configDir: sourcesDir,
		db:        db,
	}

	if err := manager.initDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return manager, nil
}

// LoadSources reads all source files from the configuration directory
func (sm *SourceManager) LoadSources() error {
	entries, err := os.ReadDir(sm.configDir)
	if err != nil {
		return fmt.Errorf("failed to read sources directory: %w", err)
	}

	var sources []SourceInterface
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			sourcePath := filepath.Join(sm.configDir, entry.Name())
			data, err := os.ReadFile(sourcePath)
			if err != nil {
				return fmt.Errorf("error reading source file %s: %w", entry.Name(), err)
			}

			var source Source
			if err := yaml.Unmarshal(data, &source.data); err != nil {
				return fmt.Errorf("error parsing source file %s: %w", entry.Name(), err)
			}
			source.filePath = sourcePath
			sources = append(sources, &source)
		}
	}

	sm.Sources = sources
	return nil
}

// FindRepo searches for a repository by name across all sources
func (sm *SourceManager) FindRepo(name string) []RepoMatch {
	var matches []RepoMatch
	for _, source := range sm.Sources {
		for _, repo := range source.FindRepo(name) {
			matches = append(matches, repo)
		}
	}
	return matches
}

// isURLAllowed checks if a URL is allowed based on the source's permissions
// GitHub URLs are allowed by default if no origin restrictions are specified
func (s *Source) isURLAllowed(url string) bool {
	// Check if there are any origin restrictions
	hasOriginRestrictions := false
	for _, perm := range s.data.Permissions {
		if len(perm.Origins) > 0 {
			hasOriginRestrictions = true
			break
		}
	}

	// If no origin restrictions, GitHub URLs are allowed by default
	if !hasOriginRestrictions && strings.HasPrefix(url, "https://github.com/") {
		return true
	}

	// Check if the URL matches any of the allowed origins
	for _, perm := range s.data.Permissions {
		// If no origins are specified in this permission, all are allowed
		if len(perm.Origins) == 0 {
			return true
		}

		for _, origin := range perm.Origins {
			if strings.HasPrefix(url, origin) {
				return true
			}
		}
	}

	return false
}

// ValidatePermissions checks if the repository's URL and build command are allowed
func (s *Source) ValidatePermissions(repo Repository) error {
	// Check URL permissions using the helper method
	if !s.isURLAllowed(repo.URL) {
		return fmt.Errorf("URL '%s' is not allowed in source %s - add its domain to the permissions.origins list", repo.URL, s.data.Name)
	}

	return nil
}

// FetchSource downloads a source file from its origin
func FetchSource(origin string) ([]byte, error) {
	resp, err := http.Get(origin)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch source: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch source: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// ValidateSourceChanges compares two sources and returns the changes
func ValidateSourceChanges(oldSource, newSource SourceInterface) (bool, SourceChanges) {
	changes := SourceChanges{}

	// Get the underlying Source structs
	oldS, ok1 := oldSource.(*Source)
	newS, ok2 := newSource.(*Source)
	if !ok1 || !ok2 {
		return false, changes
	}

	// Check identity changes (name and origin)
	if oldS.data.Name != newS.data.Name {
		changes.IdentityChanges = append(changes.IdentityChanges,
			fmt.Sprintf("Name changed from '%s' to '%s'", oldS.data.Name, newS.data.Name))
	}

	if oldS.data.Origin != newS.data.Origin {
		changes.IdentityChanges = append(changes.IdentityChanges,
			fmt.Sprintf("Origin changed from '%s' to '%s'", oldS.data.Origin, newS.data.Origin))
	}

	// Compare permissions
	oldOrigins := make(map[string]bool)
	for _, perm := range oldS.data.Permissions {
		for _, origin := range perm.Origins {
			oldOrigins[origin] = true
		}
	}

	// Check for new permissions in the updated source
	newOrigins := make(map[string]bool)
	for _, perm := range newS.data.Permissions {
		for _, origin := range perm.Origins {
			newOrigins[origin] = true
			if !oldOrigins[origin] {
				changes.RequiredPermissions = append(changes.RequiredPermissions,
					fmt.Sprintf("New origin permission requested: '%s'", origin))
			}
		}
	}

	// Check for removed permissions
	for origin := range oldOrigins {
		if !newOrigins[origin] {
			changes.PermissionChanges = append(changes.PermissionChanges,
				fmt.Sprintf("Origin permission removed: '%s'", origin))
		}
	}

	// Compare repositories
	oldRepos := make(map[string]Repository)
	for _, repo := range oldS.data.Repos {
		oldRepos[repo.Name] = repo
	}

	newRepos := make(map[string]Repository)
	for _, repo := range newS.data.Repos {
		newRepos[repo.Name] = repo
	}

	// Check for added or modified repos
	for name, newRepo := range newRepos {
		oldRepo, exists := oldRepos[name]
		if !exists {
			changes.RepositoryChanges = append(changes.RepositoryChanges,
				fmt.Sprintf("New repository added: '%s'", name))
			continue
		}

		if oldRepo.URL != newRepo.URL {
			changes.RepositoryChanges = append(changes.RepositoryChanges,
				fmt.Sprintf("Repository '%s' URL changed from '%s' to '%s'",
					name, oldRepo.URL, newRepo.URL))
		}
		if oldRepo.Build != newRepo.Build {
			changes.RepositoryChanges = append(changes.RepositoryChanges,
				fmt.Sprintf("Repository '%s' build command changed from '%s' to '%s'",
					name, oldRepo.Build, newRepo.Build))
		}
		if oldRepo.Executable != newRepo.Executable {
			changes.RepositoryChanges = append(changes.RepositoryChanges,
				fmt.Sprintf("Repository '%s' executable path changed from '%s' to '%s'",
					name, oldRepo.Executable, newRepo.Executable))
		}
	}

	// Check for removed repos
	for name := range oldRepos {
		if _, exists := newRepos[name]; !exists {
			changes.RepositoryChanges = append(changes.RepositoryChanges,
				fmt.Sprintf("Repository removed: '%s'", name))
		}
	}

	hasChanges := len(changes.IdentityChanges) > 0 ||
		len(changes.PermissionChanges) > 0 ||
		len(changes.RepositoryChanges) > 0 ||
		len(changes.RequiredPermissions) > 0

	return hasChanges, changes
}

// UpdateSource fetches and checks for changes in a source file
func (sm *SourceManager) UpdateSource(source SourceInterface) (bool, SourceChanges, error) {
	// Fetch new content
	newContent, err := FetchSource(source.GetOrigin())
	if err != nil {
		return false, SourceChanges{}, fmt.Errorf("failed to fetch source: %w", err)
	}

	// Parse new content
	var newSource Source
	if err := yaml.Unmarshal(newContent, &newSource.data); err != nil {
		return false, SourceChanges{}, fmt.Errorf("failed to parse new source: %w", err)
	}

	// Compare with current source
	hasChanges, changes := ValidateSourceChanges(source, &newSource)
	if !hasChanges {
		return false, SourceChanges{}, nil
	}

	// Store the new content in the source for later use
	if s, ok := source.(*Source); ok {
		s.newContent = newContent
	}

	// Validate all repositories in the new source
	for _, repo := range newSource.data.Repos {
		if err := newSource.ValidatePermissions(repo); err != nil {
			return true, changes, fmt.Errorf("permission validation failed: %w", err)
		}
	}

	return true, changes, nil
}

// ApplySourceUpdate writes the previously fetched update to disk
func (sm *SourceManager) ApplySourceUpdate(source *Source) error {
	if source.newContent == nil {
		return fmt.Errorf("no pending update for source %s", source.data.Name)
	}

	// Write the updated source file
	if err := os.WriteFile(source.filePath, source.newContent, 0644); err != nil {
		return fmt.Errorf("failed to write source file: %w", err)
	}

	// Clear the pending update
	source.newContent = nil
	return nil
}

// ValidateURLHost checks if the given URL's host is allowed by the source's permissions
func (s *Source) ValidateURLHost(url string) error {
	// Use the helper method to check if the URL is allowed
	if s.isURLAllowed(url) {
		return nil
	}

	return fmt.Errorf("URL host not allowed by source permissions")
}

// NormalizeAndValidateURL normalizes and validates a URL
func (sm *SourceManager) NormalizeAndValidateURL(url string) (string, error) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		// Validate URL host
		for _, source := range sm.Sources {
			if err := source.ValidateURLHost(url); err != nil {
				return "", fmt.Errorf("URL host not allowed: %w", err)
			}
		}
		return url, nil
	}

	// Normalize GitHub URLs
	cleanURL := strings.TrimPrefix(url, "github.com/")
	cleanURL = strings.TrimPrefix(cleanURL, "https://github.com/")
	cleanURL = strings.TrimPrefix(cleanURL, "http://github.com/")
	normalizedURL := fmt.Sprintf("https://github.com/%s.git", cleanURL)

	// Validate normalized URL
	for _, source := range sm.Sources {
		if err := source.ValidateURLHost(normalizedURL); err != nil {
			return "", fmt.Errorf("URL host not allowed: %w", err)
		}
	}

	return normalizedURL, nil
}

// FindRepo finds a repository by name
func (s *Source) FindRepo(name string) []RepoMatch {
	var matches []RepoMatch
	for _, repo := range s.data.Repos {
		if strings.EqualFold(repo.Name, name) {
			matches = append(matches, RepoMatch{
				Repo:   repo,
				Source: *s,
			})
		}
	}
	return matches
}

// GetName returns the name of the source
func (s *Source) GetName() string {
	return s.data.Name
}

// GetOrigin returns the origin URL of the source
func (s *Source) GetOrigin() string {
	return s.data.Origin
}

// GetPermissions returns the permissions of the source
func (s *Source) GetPermissions() []Permission {
	return s.data.Permissions
}

// GetRepos returns the repositories of the source
func (s *Source) GetRepos() []Repository {
	return s.data.Repos
}

// GetFilePath returns the file path of the source
func (s *Source) GetFilePath() string {
	return s.filePath
}

// SetFilePath sets the file path of the source
func (s *Source) SetFilePath(path string) {
	s.filePath = path
}

// ListSourceDetails returns a formatted string containing all source and repository details
func (sm *SourceManager) ListSourceDetails() string {
	var sb strings.Builder
	sb.WriteString("\nAvailable sources and tools:\n")

	for _, source := range sm.Sources {
		sb.WriteString(fmt.Sprintf("\n[%s] Origin: %s\n", source.GetName(), source.GetOrigin()))
		repos := source.GetRepos()
		if len(repos) == 0 {
			sb.WriteString("  No tools configured\n")
			continue
		}
		for _, repo := range repos {
			sb.WriteString(fmt.Sprintf("  - %s\n", repo.Name))
			sb.WriteString(fmt.Sprintf("    URL: %s\n", repo.URL))
			if repo.Build != "" {
				sb.WriteString(fmt.Sprintf("    Build command: %s\n", repo.Build))
			}
			if repo.Executable != "" {
				sb.WriteString(fmt.Sprintf("    Executable: %s\n", repo.Executable))
			}
			if repo.Load != "" {
				sb.WriteString(fmt.Sprintf("    Load command: %s\n", repo.Load))
			}
		}
	}
	return sb.String()
}

// UpdateSourceWithPrompt handles updating a single source with user interaction
func (sm *SourceManager) UpdateSourceWithPrompt(source SourceInterface, forceUpdate, dryRun bool) error {
	if source.GetOrigin() == "" {
		fmt.Printf("✓ Source '%s' has no origin, skipping\n", source.GetName())
		return nil
	}

	hasChanges, changes, err := sm.UpdateSource(source)
	if err != nil {
		return fmt.Errorf("failed to update source %s: %w", source.GetName(), err)
	}

	if !hasChanges {
		fmt.Printf("✓ No changes in source '%s'\n", source.GetName())
		return nil
	}

	// Print changes
	fmt.Printf("✓ Changes in source '%s':\n", source.GetName())
	if len(changes.IdentityChanges) > 0 {
		for _, change := range changes.IdentityChanges {
			fmt.Printf("  - %s\n", change)
		}
	}

	if len(changes.PermissionChanges) > 0 {
		for _, change := range changes.PermissionChanges {
			fmt.Printf("  - %s\n", change)
		}
	}

	if len(changes.RepositoryChanges) > 0 {
		for _, change := range changes.RepositoryChanges {
			fmt.Printf("  - %s\n", change)
		}
	}

	if len(changes.RequiredPermissions) > 0 {
		for _, perm := range changes.RequiredPermissions {
			fmt.Printf("  - New permission required: %s\n", perm)
		}
	}

	// If dry run, stop here
	if dryRun {
		fmt.Printf("✓ Changes would be applied to source '%s'\n", source.GetName())
		return nil
	}

	// If force is not set and there are changes that need approval, ask for confirmation
	if !forceUpdate && (len(changes.IdentityChanges) > 0 || len(changes.RequiredPermissions) > 0) {
		approved, err := promptUser("Do you want to apply these changes?")
		if err != nil {
			return fmt.Errorf("failed to get user input: %w", err)
		}
		if !approved {
			fmt.Printf("✓ Changes to source '%s' skipped\n", source.GetName())
			return nil
		}
	}

	// Apply changes
	if s, ok := source.(*Source); ok {
		if err := sm.ApplySourceUpdate(s); err != nil {
			return fmt.Errorf("failed to apply changes to source %s: %w", source.GetName(), err)
		}
		fmt.Printf("✓ Source '%s' updated\n", source.GetName())
	}
	return nil
}

// promptUser asks the user for confirmation
func promptUser(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}

// GetSourceCount returns the number of loaded sources
func (sm *SourceManager) GetSourceCount() int {
	return len(sm.Sources)
}

// GetSources returns a copy of the sources slice to prevent external modification
func (sm *SourceManager) GetSources() []SourceInterface {
	sources := make([]SourceInterface, len(sm.Sources))
	copy(sources, sm.Sources)
	return sources
}

// PrintRepoInfo prints repository information to the given writer
func (sm *SourceManager) PrintRepoInfo(w *tabwriter.Writer, repo RepoInfo, verbose, veryVerbose bool) {
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

	// Additional info with -v
	if verbose || veryVerbose {
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
	}
}

// GetUniqueRepos returns a map of unique repositories based on installation status
func (sm *SourceManager) GetUniqueRepos(repos []RepoInfo, workDir string, installedOnly bool) map[string]RepoInfo {
	uniqueTools := make(map[string]RepoInfo)

	for _, repo := range repos {
		// If tool is installed and there's a conflict
		if _, exists := uniqueTools[repo.Name]; exists {
			// Keep existing if it matches .getgit file
			continue
		} else {
			uniqueTools[repo.Name] = repo
		}
	}

	return uniqueTools
}
