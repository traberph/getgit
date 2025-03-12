package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/index"
	"github.com/traberph/getgit/pkg/repository"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <tool>",
	Short: "Uninstall a tool",
	Long: `Removes an installed tool.

Removes the tool's files, aliases, and configuration.

Example:
  getgit uninstall toolname    # Remove the specified tool`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		toolName := args[0]

		// Get work directory
		workDir, err := config.GetWorkDir()
		if err != nil {
			return fmt.Errorf("failed to get work directory: %w", err)
		}

		// Create repository manager to get access to OutputManager
		rm, err := repository.NewManager(workDir, verbose)
		if err != nil {
			return fmt.Errorf("failed to create repository manager: %w", err)
		}

		// Check if tool is installed
		toolPath := filepath.Join(workDir, toolName)
		if _, err := os.Stat(toolPath); os.IsNotExist(err) {
			return fmt.Errorf("tool '%s' is not installed", toolName)
		}

		rm.Output.PrintInfo(fmt.Sprintf("Starting uninstallation of '%s'...\n", toolName))

		// Create alias manager to remove the alias
		aliasManager, err := repository.NewAliasManager()
		if err != nil {
			return fmt.Errorf("failed to initialize alias manager: %w", err)
		}

		// Check if there was an alias or source before removing
		aliasRemoved := false
		sourceRemoved := false
		for name := range aliasManager.GetAliases() {
			if name == toolName {
				aliasRemoved = true
				break
			}
		}
		for name := range aliasManager.GetSources() {
			if name == toolName {
				sourceRemoved = true
				break
			}
		}

		// Remove the tool's directory
		if err := os.RemoveAll(toolPath); err != nil {
			return fmt.Errorf("failed to remove tool directory: %w", err)
		}
		rm.Output.PrintStatus("Removed tool directory")

		// Remove the alias and source
		if err := aliasManager.RemoveAlias(toolName); err != nil {
			return fmt.Errorf("failed to remove alias: %w", err)
		}

		if aliasRemoved {
			rm.Output.PrintStatus("Removed alias")
		}
		if sourceRemoved {
			rm.Output.PrintStatus("Removed source line from .alias file")
		}

		// Update completion script
		if err := updateCompletionScript(); err != nil {
			rm.Output.PrintError(fmt.Sprintf("Warning: Failed to update completion script: %v", err))
		}

		rm.Output.PrintInfo(fmt.Sprintf("\nUninstallation of '%s' completed successfully!", toolName))
		return nil
	},
}

func init() {
	// Add completion support
	uninstallCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Create a new index manager
		indexManager, err := index.NewManager()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		defer indexManager.Close()

		// Get all available tools
		repos, err := indexManager.ListRepositories()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Get work directory for checking installed tools
		workDir, err := config.GetWorkDir()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Build list of installed tools
		var suggestions []string
		for _, repo := range repos {
			// Check if tool is installed
			toolPath := filepath.Join(workDir, repo.Name)
			if _, err := os.Stat(toolPath); err == nil {
				suggestions = append(suggestions, repo.Name)
			}
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	rootCmd.AddCommand(uninstallCmd)
}
