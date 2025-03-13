package shell

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
)

// UpdateCompletionScript updates the bash completion script
func UpdateCompletionScript(rootCmd *cobra.Command) error {
	// Get work directory
	workDir, err := config.GetWorkDir()
	if err != nil {
		return fmt.Errorf("failed to get work directory: %w", err)
	}

	// Update bash completion script
	completionFile := filepath.Join(workDir, ".bash_completion")
	f, err := os.Create(completionFile)
	if err != nil {
		return fmt.Errorf("failed to create completion file: %w", err)
	}
	defer f.Close()

	// Generate new completion script
	if err := rootCmd.GenBashCompletion(f); err != nil {
		return fmt.Errorf("failed to generate completion script: %w", err)
	}

	return nil
}
