package index

import (
	"os"
	"testing"

	"github.com/traberph/getgit/pkg/sources"
)

func TestIndexManager(t *testing.T) {
	// Create a temporary directory for the test database
	tmpDir, err := os.MkdirTemp("", "getgit-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up a test environment
	os.Setenv("XDG_CACHE_HOME", tmpDir)

	// Create a new index manager
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create index manager: %v", err)
	}
	defer manager.Close()

	// Create test source data
	sourceManager := &sources.SourceManager{
		Sources: []sources.Source{
			{
				Name:     "test-source",
				FilePath: "/test/source1.yaml",
				Repos: []sources.Repository{
					{
						Name:       "repo1",
						URL:        "https://github.com/test/repo1",
						Build:      "make install",
						Executable: "/usr/local/bin/repo1",
					},
					{
						Name:       "repo2",
						URL:        "https://github.com/test/repo2",
						Build:      "go install",
						Executable: "/usr/local/bin/repo2",
					},
				},
			},
			{
				Name:     "test-source2",
				FilePath: "/test/source2.yaml",
				Repos: []sources.Repository{
					{
						Name:       "repo1",
						URL:        "https://github.com/other/repo1",
						Build:      "make",
						Executable: "/usr/local/bin/repo1-alt",
					},
				},
			},
		},
	}

	// Test UpdateIndex
	if err := manager.UpdateIndex(sourceManager); err != nil {
		t.Fatalf("Failed to update index: %v", err)
	}

	// Test FindRepository
	repos, err := manager.FindRepository("repo1")
	if err != nil {
		t.Fatalf("Failed to find repository: %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("Expected 2 repositories named repo1, got %d", len(repos))
	}

	// Verify repository details
	found := false
	for _, repo := range repos {
		if repo.URL == "https://github.com/test/repo1" {
			found = true
			if repo.Build != "make install" {
				t.Errorf("Expected build command 'make install', got '%s'", repo.Build)
			}
			if repo.SourceName != "test-source" {
				t.Errorf("Expected source name 'test-source', got '%s'", repo.SourceName)
			}
		}
	}
	if !found {
		t.Error("Did not find expected repository")
	}

	// Test ListRepositories
	allRepos, err := manager.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}
	if len(allRepos) != 3 {
		t.Errorf("Expected 3 total repositories, got %d", len(allRepos))
	}
}
