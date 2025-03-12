package sources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePermissions(t *testing.T) {
	tests := []struct {
		name    string
		source  Source
		repo    Repository
		wantErr bool
	}{
		{
			name: "valid permission",
			source: Source{
				Permissions: []Permission{
					{Origins: []string{"github.com"}},
				},
			},
			repo: Repository{
				URL: "https://github.com/user/repo",
			},
			wantErr: false,
		},
		{
			name: "invalid permission",
			source: Source{
				Permissions: []Permission{
					{Origins: []string{"github.com"}},
				},
			},
			repo: Repository{
				URL: "https://gitlab.com/user/repo",
			},
			wantErr: true,
		},
		{
			name: "no permissions - allow all",
			source: Source{
				Permissions: []Permission{},
			},
			repo: Repository{
				URL: "https://github.com/user/repo",
			},
			wantErr: false,
		},
		{
			name: "empty origins - allow all",
			source: Source{
				Permissions: []Permission{
					{Origins: []string{}},
				},
			},
			repo: Repository{
				URL: "https://github.com/user/repo",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.source.ValidatePermissions(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePermissions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSourceChanges(t *testing.T) {
	tests := []struct {
		name        string
		oldSource   Source
		newSource   Source
		wantValid   bool
		wantChanges SourceChanges
	}{
		{
			name: "no changes",
			oldSource: Source{
				Name:   "test",
				Origin: "https://example.com/source",
				Permissions: []Permission{
					{Origins: []string{"github.com"}},
				},
				Repos: []Repository{
					{Name: "repo1", URL: "https://github.com/user/repo1"},
				},
			},
			newSource: Source{
				Name:   "test",
				Origin: "https://example.com/source",
				Permissions: []Permission{
					{Origins: []string{"github.com"}},
				},
				Repos: []Repository{
					{Name: "repo1", URL: "https://github.com/user/repo1"},
				},
			},
			wantValid:   false,
			wantChanges: SourceChanges{},
		},
		{
			name: "identity change",
			oldSource: Source{
				Name:   "test",
				Origin: "https://example.com/source",
			},
			newSource: Source{
				Name:   "test2",
				Origin: "https://example.com/source2",
			},
			wantValid: true,
			wantChanges: SourceChanges{
				IdentityChanges: []string{
					"Name changed from 'test' to 'test2'",
					"Origin changed from 'https://example.com/source' to 'https://example.com/source2'",
				},
			},
		},
		{
			name: "permission change",
			oldSource: Source{
				Name: "test",
				Permissions: []Permission{
					{Origins: []string{"github.com"}},
				},
			},
			newSource: Source{
				Name: "test",
				Permissions: []Permission{
					{Origins: []string{"github.com", "gitlab.com"}},
				},
			},
			wantValid: true,
			wantChanges: SourceChanges{
				RequiredPermissions: []string{"New origin permission requested: 'gitlab.com'"},
			},
		},
		{
			name: "remove permission",
			oldSource: Source{
				Name: "test",
				Permissions: []Permission{
					{Origins: []string{"github.com", "gitlab.com"}},
				},
			},
			newSource: Source{
				Name: "test",
				Permissions: []Permission{
					{Origins: []string{"github.com"}},
				},
			},
			wantValid: true,
			wantChanges: SourceChanges{
				PermissionChanges: []string{"Origin permission removed: 'gitlab.com'"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, changes := ValidateSourceChanges(tt.oldSource, tt.newSource)
			if valid != tt.wantValid {
				t.Errorf("ValidateSourceChanges() valid = %v, want %v", valid, tt.wantValid)
			}
			if !compareStringSlices(changes.IdentityChanges, tt.wantChanges.IdentityChanges) {
				t.Errorf("ValidateSourceChanges() identity changes = %v, want %v", changes.IdentityChanges, tt.wantChanges.IdentityChanges)
			}
			if !compareStringSlices(changes.PermissionChanges, tt.wantChanges.PermissionChanges) {
				t.Errorf("ValidateSourceChanges() permission changes = %v, want %v", changes.PermissionChanges, tt.wantChanges.PermissionChanges)
			}
			if !compareStringSlices(changes.RequiredPermissions, tt.wantChanges.RequiredPermissions) {
				t.Errorf("ValidateSourceChanges() required permissions = %v, want %v", changes.RequiredPermissions, tt.wantChanges.RequiredPermissions)
			}
		})
	}
}

// Helper function to compare string slices regardless of order
func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int)
	for _, str := range a {
		seen[str]++
	}
	for _, str := range b {
		seen[str]--
		if seen[str] < 0 {
			return false
		}
	}
	return true
}

func TestSourceManager(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "getgit-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test source file
	sourceContent := `
name: test-source
origin: https://example.com/source
permissions:
  - origins: ["github.com"]
repos:
  - name: repo1
    url: https://github.com/user/repo1
    build: make build
    executable: bin/repo1
    load: make install
collections:
  - name: collection1
    repos: ["repo1"]
`
	sourcePath := filepath.Join(tmpDir, "test-source.yaml")
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Create SourceManager
	sm := &SourceManager{
		configDir: tmpDir,
	}

	// Test LoadSources
	if err := sm.LoadSources(); err != nil {
		t.Errorf("LoadSources() error = %v", err)
	}
	if len(sm.Sources) != 1 {
		t.Errorf("LoadSources() sources = %v, want 1", len(sm.Sources))
	}

	// Test FindRepo
	matches := sm.FindRepo("repo1")
	if len(matches) != 1 {
		t.Errorf("FindRepo() matches = %v, want 1", len(matches))
	}
	if matches[0].Repo.Name != "repo1" {
		t.Errorf("FindRepo() repo name = %v, want repo1", matches[0].Repo.Name)
	}
}
