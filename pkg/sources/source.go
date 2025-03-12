package sources

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/traberph/getgit/pkg/config"
	"gopkg.in/yaml.v3"
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

// Collection represents a collection of repositories
type Collection struct {
	Name  string   `yaml:"name"`
	Repos []string `yaml:"repos"`
}

// Source represents a source configuration file
type Source struct {
	Name        string       `yaml:"name"`
	Origin      string       `yaml:"origin"`      // URL where the source file is hosted
	Permissions []Permission `yaml:"permissions"` // Security permissions
	Repos       []Repository `yaml:"repos"`
	Collections []Collection `yaml:"collections"`
	FilePath    string       `yaml:"-"` // Internal use to track source file
	newContent  []byte       `yaml:"-"` // Internal use to store new content for later use
}

// SourceChanges represents different types of changes in a source
type SourceChanges struct {
	IdentityChanges     []string // Changes to name or origin
	PermissionChanges   []string // Changes to permissions
	RepositoryChanges   []string // Changes to repositories
	RequiredPermissions []string // New permissions that need approval
}

// SourceManager handles all source-related operations
type SourceManager struct {
	configDir string
	Sources   []Source
}

// RepoMatch represents a repository match with its source
type RepoMatch struct {
	Repo   Repository
	Source Source
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

	return &SourceManager{
		configDir: sourcesDir,
	}, nil
}

// LoadSources reads all source files from the configuration directory
func (sm *SourceManager) LoadSources() error {
	entries, err := os.ReadDir(sm.configDir)
	if err != nil {
		return fmt.Errorf("failed to read sources directory: %w", err)
	}

	var sources []Source
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			sourcePath := filepath.Join(sm.configDir, entry.Name())
			data, err := os.ReadFile(sourcePath)
			if err != nil {
				return fmt.Errorf("error reading source file %s: %w", entry.Name(), err)
			}

			var source Source
			if err := yaml.Unmarshal(data, &source); err != nil {
				return fmt.Errorf("error parsing source file %s: %w", entry.Name(), err)
			}
			source.FilePath = sourcePath
			sources = append(sources, source)
		}
	}

	sm.Sources = sources
	return nil
}

// FindRepo searches for a repository by name across all sources
func (sm *SourceManager) FindRepo(name string) []RepoMatch {
	var matches []RepoMatch
	for _, source := range sm.Sources {
		for _, repo := range source.Repos {
			if strings.EqualFold(repo.Name, name) {
				matches = append(matches, RepoMatch{
					Repo:   repo,
					Source: source,
				})
			}
		}
	}
	return matches
}

// ValidatePermissions checks if the repository's URL and build command are allowed
func (s *Source) ValidatePermissions(repo Repository) error {
	// Check URL permissions
	// If no origins are specified in permissions, allow GitHub URLs by default
	hasOriginRestrictions := false
	for _, perm := range s.Permissions {
		if len(perm.Origins) > 0 {
			hasOriginRestrictions = true
			break
		}
	}

	urlAllowed := !hasOriginRestrictions // Allow if no restrictions
	if hasOriginRestrictions {
		// Check custom origins
		for _, perm := range s.Permissions {
			for _, origin := range perm.Origins {
				if strings.Contains(repo.URL, origin) {
					urlAllowed = true
					break
				}
			}
			if urlAllowed {
				break
			}
		}
	}

	if !urlAllowed {
		return fmt.Errorf("URL '%s' is not allowed in source %s - add its domain to the permissions.origins list", repo.URL, s.Name)
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
func ValidateSourceChanges(oldSource, newSource Source) (bool, SourceChanges) {
	changes := SourceChanges{}

	// Check identity changes (name and origin)
	if oldSource.Name != newSource.Name {
		changes.IdentityChanges = append(changes.IdentityChanges,
			fmt.Sprintf("Name changed from '%s' to '%s'", oldSource.Name, newSource.Name))
	}

	if oldSource.Origin != newSource.Origin {
		changes.IdentityChanges = append(changes.IdentityChanges,
			fmt.Sprintf("Origin changed from '%s' to '%s'", oldSource.Origin, newSource.Origin))
	}

	// Compare permissions
	oldOrigins := make(map[string]bool)
	for _, perm := range oldSource.Permissions {
		for _, origin := range perm.Origins {
			oldOrigins[origin] = true
		}
	}

	// Check for new permissions in the updated source
	newOrigins := make(map[string]bool)
	for _, perm := range newSource.Permissions {
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
	for _, repo := range oldSource.Repos {
		oldRepos[repo.Name] = repo
	}

	newRepos := make(map[string]Repository)
	for _, repo := range newSource.Repos {
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
func (sm *SourceManager) UpdateSource(source *Source) (bool, SourceChanges, error) {
	// Fetch new content
	newContent, err := FetchSource(source.Origin)
	if err != nil {
		return false, SourceChanges{}, fmt.Errorf("failed to fetch source: %w", err)
	}

	// Parse new content
	var newSource Source
	if err := yaml.Unmarshal(newContent, &newSource); err != nil {
		return false, SourceChanges{}, fmt.Errorf("failed to parse new source: %w", err)
	}

	// Compare with current source
	hasChanges, changes := ValidateSourceChanges(*source, newSource)
	if !hasChanges {
		return false, SourceChanges{}, nil
	}

	// Store the new content in the source for later use
	source.newContent = newContent

	// Validate all repositories in the new source
	for _, repo := range newSource.Repos {
		if err := newSource.ValidatePermissions(repo); err != nil {
			return true, changes, fmt.Errorf("permission validation failed: %w", err)
		}
	}

	return true, changes, nil
}

// ApplySourceUpdate writes the previously fetched update to disk
func (sm *SourceManager) ApplySourceUpdate(source *Source) error {
	if source.newContent == nil {
		return fmt.Errorf("no pending update for source %s", source.Name)
	}

	// Write the updated source file
	if err := os.WriteFile(source.FilePath, source.newContent, 0644); err != nil {
		return fmt.Errorf("failed to write source file: %w", err)
	}

	// Clear the pending update
	source.newContent = nil
	return nil
}

// ValidateURLHost checks if the given URL's host is allowed by the source's permissions
func (s *Source) ValidateURLHost(url string) error {
	// If no origins are specified in permissions, all are allowed
	for _, perm := range s.Permissions {
		if len(perm.Origins) == 0 {
			return nil
		}
		for _, origin := range perm.Origins {
			if strings.HasPrefix(url, origin) {
				return nil
			}
		}
	}
	return fmt.Errorf("URL host not allowed by source permissions")
}
