package sources

import (
	"testing"
)

func TestValidatePermissions(t *testing.T) {
	source := Source{
		Name:   "traberph",
		Origin: "https://traberph.de/gg",
		Permissions: []Permission{
			{
				Commands: []string{"make"},
			},
			{
				Origins: []string{},
			},
		},
	}

	// Test valid repository (from example)
	validRepo := Repository{
		Name:       "k9s",
		URL:        "derailed/k9s",
		Build:      "make build",
		Executable: "execs/k9s",
	}

	if err := source.ValidatePermissions(validRepo); err != nil {
		t.Errorf("Expected no error for valid repository, got: %v", err)
	}

	// Test repository without build command
	noBuildRepo := Repository{
		Name:       "script",
		URL:        "user/script",
		Build:      "",
		Executable: "script.sh",
	}

	if err := source.ValidatePermissions(noBuildRepo); err != nil {
		t.Errorf("Expected no error for repository without build command, got: %v", err)
	}

	// Test repository with only whitespace build command
	whitespaceRepo := Repository{
		Name:       "script2",
		URL:        "user/script2",
		Build:      "   ",
		Executable: "script.sh",
	}

	if err := source.ValidatePermissions(whitespaceRepo); err != nil {
		t.Errorf("Expected no error for repository with whitespace build command, got: %v", err)
	}

	// Test invalid build command
	invalidBuildRepo := Repository{
		Name:       "test",
		URL:        "derailed/k9s",
		Build:      "cargo build",
		Executable: "test",
	}

	if err := source.ValidatePermissions(invalidBuildRepo); err == nil {
		t.Error("Expected error for invalid build command, got none")
	}

	// Test non-GitHub URL without allowed origin
	invalidURLRepo := Repository{
		Name:       "test",
		URL:        "gitlab.com/test/repo",
		Build:      "make build",
		Executable: "test",
	}

	if err := source.ValidatePermissions(invalidURLRepo); err == nil {
		t.Error("Expected error for invalid URL, got none")
	}
}
