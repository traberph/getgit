package repository

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestOutputManager(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
	}{
		{
			name:    "verbose mode",
			verbose: true,
		},
		{
			name:    "non-verbose mode",
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			om := NewOutputManager(tt.verbose)
			if om == nil {
				t.Error("NewOutputManager() returned nil")
			}
			if om.verbose != tt.verbose {
				t.Errorf("NewOutputManager().verbose = %v, want %v", om.verbose, tt.verbose)
			}

			// Test stage management
			om.StartStage("test stage")
			om.CompleteStage()
			om.StopStage()

			// Test output methods
			om.AddOutput("test output")
			om.PrintStatus("test status")
			om.PrintError("test error")
			om.PrintInfo("test info")
		})
	}
}

func TestRepositoryManager(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "getgit-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test repository manager
	rm, err := NewManager(tmpDir, true)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Test repository operations
	repo := Repository{
		Name:       "k9s",
		URL:        "https://github.com/derailed/k9s",
		Build:      "make build",
		Executable: "execs/k9s",
		UseEdge:    false,
		SkipBuild:  false,
	}

	// Test CloneOrUpdate
	repoPath, err := rm.CloneOrUpdate(repo.URL, repo.Name)
	if err != nil {
		t.Errorf("CloneOrUpdate() error = %v", err)
	}
	if repoPath == "" {
		t.Error("CloneOrUpdate() returned empty path")
	}

	// Ensure the repository path is cleaned up after the test
	defer os.RemoveAll(repoPath)

	// Test version operations
	tag, err := rm.GetLatestTag(repoPath)
	if err != nil {
		t.Errorf("GetLatestTag() error = %v", err)
	}
	if tag == "" {
		t.Error("GetLatestTag() returned empty tag")
	}

	if err := rm.CheckoutTag(repoPath, tag); err != nil {
		t.Errorf("CheckoutTag() error = %v", err)
	}

	currentTag, err := rm.GetCurrentTag(repoPath)
	if err != nil {
		t.Errorf("GetCurrentTag() error = %v", err)
	}
	if currentTag != tag {
		t.Errorf("GetCurrentTag() = %v, want %v", currentTag, tag)
	}

	hasUpdates, err := rm.HasEdgeUpdates(repoPath)
	if err != nil {
		t.Errorf("HasEdgeUpdates() error = %v", err)
	}
	// We can't reliably predict if there will be updates or not
	// Just ensure the call succeeds
	t.Logf("HasEdgeUpdates() = %v", hasUpdates)
}

// Helper function to initialize a test Git repository
func initTestRepo(path string) error {
	cmds := []struct {
		name string
		args []string
	}{
		{"git", []string{"init"}},
		{"git", []string{"config", "user.name", "Test User"}},
		{"git", []string{"config", "user.email", "test@example.com"}},
		{"git", []string{"commit", "--allow-empty", "-m", "Initial commit"}},
		{"git", []string{"tag", "v1.0.0"}},
	}

	for _, cmd := range cmds {
		c := exec.Command(cmd.name, cmd.args...)
		c.Dir = path
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run %s: %w", cmd.name, err)
		}
	}

	return nil
}
