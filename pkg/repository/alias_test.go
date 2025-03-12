package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/traberph/getgit/pkg/config"
)

func TestAliasManager(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "getgit-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up test environment
	os.Setenv("HOME", tmpDir)
	configDir := filepath.Join(tmpDir, ".config", "getgit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create test config file
	configFile := filepath.Join(configDir, "config.yaml")
	testConfig := []byte("root: " + tmpDir)
	if err := os.WriteFile(configFile, testConfig, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create new alias manager
	am, err := NewAliasManager()
	if err != nil {
		t.Fatalf("NewAliasManager() error = %v", err)
	}

	// Test adding an alias
	toolName := "test-tool"
	binaryPath := filepath.Join(tmpDir, "test-tool", "bin", "test")
	if err := am.AddAlias(toolName, binaryPath); err != nil {
		t.Errorf("AddAlias() error = %v", err)
	}

	// Create test .getgit file
	toolDir := filepath.Join(tmpDir, "test-tool")
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		t.Fatalf("Failed to create tool dir: %v", err)
	}
	getgitFile := filepath.Join(toolDir, ".getgit")
	getgitContent := `#!/bin/bash

: <<'EOF'
sourcefile: test-source
updates: release
collection:
  - test-collection
EOF

source test-tool/env.sh`
	if err := os.WriteFile(getgitFile, []byte(getgitContent), 0644); err != nil {
		t.Fatalf("Failed to write .getgit file: %v", err)
	}

	// Test adding a source
	if err := am.AddSource(toolName, getgitFile); err != nil {
		t.Errorf("AddSource() error = %v", err)
	}

	// Test getting aliases
	aliases := am.GetAliases()
	if alias, ok := aliases[toolName]; !ok {
		t.Errorf("GetAliases() missing alias for %s", toolName)
	} else if alias != binaryPath {
		t.Errorf("GetAliases()[%s] = %v, want %v", toolName, alias, binaryPath)
	}

	// Test getting sources
	sources := am.GetSources()
	if source, ok := sources[toolName]; !ok {
		t.Errorf("GetSources() missing source for %s", toolName)
	} else if source != getgitFile {
		t.Errorf("GetSources()[%s] = %v, want %v", toolName, source, getgitFile)
	}

	// Test removing an alias
	if err := am.RemoveAlias(toolName); err != nil {
		t.Errorf("RemoveAlias() error = %v", err)
	}

	// Verify alias was removed
	aliases = am.GetAliases()
	if _, ok := aliases[toolName]; ok {
		t.Errorf("Alias %s still exists after removal", toolName)
	}

	// Test loading aliases from file
	testToolDir := filepath.Join(tmpDir, "test-path")
	if err := os.MkdirAll(testToolDir, 0755); err != nil {
		t.Fatalf("Failed to create test tool dir: %v", err)
	}
	testGetgitFile := filepath.Join(testToolDir, ".getgit")
	testGetgitContent := `#!/bin/bash

: <<'EOF'
sourcefile: test-source
updates: release
collection:
  - test-collection
EOF

source test-path/env.sh`
	if err := os.WriteFile(testGetgitFile, []byte(testGetgitContent), 0644); err != nil {
		t.Fatalf("Failed to write test .getgit file: %v", err)
	}

	// Write test alias file
	aliasFile, err := config.GetAliasFile()
	if err != nil {
		t.Fatalf("Failed to get alias file path: %v", err)
	}

	testAliasContent := fmt.Sprintf(`# This file is managed by getgit. Do not edit manually.
# It contains aliases for installed tools.
alias test-tool="/test/path/bin/test"
source "%s" # test-tool
`, testGetgitFile)

	if err := os.WriteFile(aliasFile, []byte(testAliasContent), 0644); err != nil {
		t.Fatalf("Failed to write test alias file: %v", err)
	}

	// Create new manager to test loading
	am2, err := NewAliasManager()
	if err != nil {
		t.Fatalf("NewAliasManager() error = %v", err)
	}

	// Verify loaded aliases
	aliases = am2.GetAliases()
	if alias, ok := aliases["test-tool"]; !ok {
		t.Error("Failed to load alias from file")
	} else if alias != "/test/path/bin/test" {
		t.Errorf("Loaded alias = %v, want /test/path/bin/test", alias)
	}

	// Verify loaded sources
	sources = am2.GetSources()
	if source, ok := sources["test-tool"]; !ok {
		t.Error("Failed to load source from file")
	} else if source != testGetgitFile {
		t.Errorf("Loaded source = %v, want %v", source, testGetgitFile)
	}
}
