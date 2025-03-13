package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/repository"
	"github.com/traberph/getgit/pkg/shell"
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
		defer rm.Close()

		// Check if tool is installed
		isInstalled, err := rm.IsToolInstalled(toolName)
		if err != nil {
			return fmt.Errorf("failed to check if tool is installed: %w", err)
		}
		if !isInstalled {
			return fmt.Errorf("tool '%s' is not installed", toolName)
		}

		rm.Output.PrintInfo(fmt.Sprintf("Starting uninstallation of '%s'...\n", toolName))

		// Remove the tool's directory
		toolPath := filepath.Join(workDir, toolName)
		if err := os.RemoveAll(toolPath); err != nil {
			return fmt.Errorf("failed to remove tool directory: %w", err)
		}
		rm.Output.PrintStatus(fmt.Sprintf("Removed '%s' directory", toolName))

		// Update completion script
		if err := shell.UpdateCompletionScript(cmd); err != nil {
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

		// Get work directory for checking installed tools
		workDir, err := config.GetWorkDir()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Create repository manager to list installed tools
		rm, err := repository.NewManager(workDir, false)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		defer rm.Close()

		// Get list of installed tools by reading the work directory
		entries, err := os.ReadDir(workDir)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var tools []string
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == ".git" {
				continue
			}
			toolPath := filepath.Join(workDir, entry.Name())
			if _, err := os.Stat(filepath.Join(toolPath, ".git")); err == nil {
				tools = append(tools, entry.Name())
			}
		}

		return tools, cobra.ShellCompDirectiveNoFileComp
	}

	rootCmd.AddCommand(uninstallCmd)
}
