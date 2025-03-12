package getgitfile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadWriteGetGitFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "getgit-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test data
	sourceName := "test-tool"
	updateTrain := "edge"
	collection := []string{"dev-tools", "testing"}
	loadCommand := "source /path/to/test-tool/env.sh"

	// Write test file
	err = WriteToRepo(tempDir, sourceName, updateTrain, collection, loadCommand)
	if err != nil {
		t.Fatalf("WriteToRepo failed: %v", err)
	}

	// Verify file exists and has correct permissions
	filePath := filepath.Join(tempDir, GetGitFileName)
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("Expected file permissions 0755, got %v", info.Mode().Perm())
	}

	// Read the file back
	getgitFile, err := ReadFromRepo(tempDir)
	if err != nil {
		t.Fatalf("ReadFromRepo failed: %v", err)
	}

	// Verify contents
	if getgitFile.SourceName != sourceName {
		t.Errorf("Expected source name %q, got %q", sourceName, getgitFile.SourceName)
	}
	if getgitFile.UpdateTrain != updateTrain {
		t.Errorf("Expected update train %q, got %q", updateTrain, getgitFile.UpdateTrain)
	}
	if !reflect.DeepEqual(getgitFile.Collection, collection) {
		t.Errorf("Expected collection %v, got %v", collection, getgitFile.Collection)
	}
	if getgitFile.LoadCommand != loadCommand {
		t.Errorf("Expected load command %q, got %q", loadCommand, getgitFile.LoadCommand)
	}
}

func TestReadFromNonExistentRepo(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "getgit-test-nonexistent")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Try to read from directory with no .getgit file
	getgitFile, err := ReadFromRepo(tempDir)
	if err != nil {
		t.Errorf("Expected nil error for non-existent .getgit file, got %v", err)
	}
	if getgitFile != nil {
		t.Errorf("Expected nil GetGitFile for non-existent file, got %+v", getgitFile)
	}
}

func TestInvalidUpdateTrain(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "getgit-test-invalid")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with invalid update train
	err = WriteToRepo(tempDir, "test-tool", "invalid", nil, "")
	if err != nil {
		t.Fatalf("WriteToRepo failed: %v", err)
	}

	// Read back and verify it defaulted to "release"
	getgitFile, err := ReadFromRepo(tempDir)
	if err != nil {
		t.Fatalf("ReadFromRepo failed: %v", err)
	}
	if getgitFile.UpdateTrain != "release" {
		t.Errorf("Expected invalid update train to default to 'release', got %q", getgitFile.UpdateTrain)
	}
}
