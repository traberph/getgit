package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/getgitfile"
	"github.com/traberph/getgit/pkg/repository"
	"github.com/traberph/getgit/pkg/sources"
	"github.com/traberph/getgit/pkg/utils"
)

// verbose is a persistent flag defined in root.go
var upgradeSkipBuild bool // Skip building the tool after upgrade

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [tool]",
	Short: "Upgrade installed tools",
	Long: `Upgrades installed tools to their latest versions.

Without arguments, upgrades all installed tools.
With a tool name, upgrades only that specific tool.

Examples:
  getgit upgrade         # Upgrade all installed tools
  getgit upgrade k9s    # Upgrade only k9s`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get work directory
		workDir, err := config.GetWorkDir()
		if err != nil {
			return fmt.Errorf("failed to get work directory: %w", err)
		}

		// Initialize source manager
		sm, err := sources.NewSourceManager()
		if err != nil {
			return fmt.Errorf("failed to initialize source manager: %w", err)
		}

		if err := sm.LoadSources(); err != nil {
			return fmt.Errorf("failed to load sources: %w", err)
		}

		// Initialize repository manager
		rm, err := repository.NewManager(workDir, verbose)
		if err != nil {
			return fmt.Errorf("failed to create repository manager: %w", err)
		}

		// If a specific tool is specified, only upgrade that one
		if len(args) > 0 {
			toolName := args[0]
			return upgradeSpecificTool(sm, rm, toolName, workDir)
		}

		// Otherwise, upgrade all installed tools
		return upgradeAllTools(sm, rm, workDir)
	},
}

func init() {
	upgradeCmd.Flags().BoolVarP(&upgradeSkipBuild, "skip-build", "s", false, "Skip building the tool after upgrade")
	rootCmd.AddCommand(upgradeCmd)
}

// checkForUpdates checks if there are updates available for a repository
func checkForUpdates(rm *repository.Manager, repoPath string, useEdge bool) (bool, string, error) {
	// Fetch updates from remote
	if err := rm.FetchUpdates(repoPath); err != nil {
		return false, "", fmt.Errorf("failed to fetch updates: %w", err)
	}

	if useEdge {
		// For edge, check if the remote HEAD is different from local HEAD
		hasUpdates, err := rm.HasEdgeUpdates(repoPath)
		if err != nil {
			return false, "", fmt.Errorf("failed to check for edge updates: %w", err)
		}
		return hasUpdates, "", nil
	}

	// For release, check if there's a newer tag available
	currentTag, err := rm.GetCurrentTag(repoPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to get current tag: %w", err)
	}

	latestTag, err := rm.GetLatestTag(repoPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to get latest tag: %w", err)
	}

	// If no tags exist and we're in release mode, consider it as needing update
	if latestTag == "" {
		return true, "", nil
	}

	// If we have no current tag, we need an update
	if currentTag == "" {
		return true, latestTag, nil
	}

	// Compare versions
	hasUpdate, err := rm.IsTagNewer(repoPath, currentTag, latestTag)
	if err != nil {
		return false, "", fmt.Errorf("failed to compare versions: %w", err)
	}
	return hasUpdate, latestTag, nil
}

func upgradeSpecificTool(sm *sources.SourceManager, rm *repository.Manager, toolName, workDir string) error {
	toolPath := filepath.Join(workDir, toolName)
	if _, err := os.Stat(toolPath); os.IsNotExist(err) {
		return fmt.Errorf("tool '%s' is not installed", toolName)
	}

	// Find the tool in sources
	matches := sm.FindRepo(toolName)
	if len(matches) == 0 {
		return fmt.Errorf("tool '%s' not found in any source", toolName)
	}

	// Check for .getgit file
	getgitFile, err := getgitfile.ReadFromRepo(toolPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read .getgit file: %w", err)
	}

	var selectedMatch *sources.RepoMatch
	if getgitFile != nil {
		// Find the match corresponding to the source in .getgit file
		for _, match := range matches {
			if match.Source.GetName() == getgitFile.SourceName {
				selectedMatch = &match
				break
			}
		}
		if selectedMatch == nil {
			return fmt.Errorf("source '%s' specified in .getgit file no longer contains this tool", getgitFile.SourceName)
		}
	} else if len(matches) == 1 {
		selectedMatch = &matches[0]
	} else {
		// If multiple matches and no .getgit file, prompt user to select one
		var err error
		selectedMatch, err = utils.PromptSourceSelection(matches)
		if err != nil {
			return fmt.Errorf("source selection failed: %w", err)
		}

		// Create .getgit file for future reference
		updateTrain := "release"
		if err := getgitfile.WriteToRepo(toolPath, selectedMatch.Source.GetName(), updateTrain, selectedMatch.Repo.Load); err != nil {
			return fmt.Errorf("failed to write .getgit file: %w", err)
		}
	}

	// Determine update train
	useEdge := getgitFile != nil && getgitFile.UpdateTrain == "edge"

	// Check for updates
	hasUpdates, _, err := checkForUpdates(rm, toolPath, useEdge)
	if err != nil {
		if strings.Contains(err.Error(), "failed to fetch updates") {
			return fmt.Errorf("network error while checking for updates: %w", err)
		}
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !hasUpdates {
		return fmt.Errorf("tool '%s' is already up to date", toolName)
	}

	// Update the tool
	if err := rm.UpdatePackage(repository.Repository{
		Name:       selectedMatch.Repo.Name,
		URL:        selectedMatch.Repo.URL,
		Build:      selectedMatch.Repo.Build,
		Executable: selectedMatch.Repo.Executable,
		Load:       selectedMatch.Repo.Load,
		UseEdge:    useEdge,
		SkipBuild:  upgradeSkipBuild,
		SourceName: selectedMatch.Source.GetName(),
	}); err != nil {
		if strings.Contains(err.Error(), "build failed:") {
			return fmt.Errorf("build failed for '%s': %w", toolName, err)
		} else if strings.Contains(err.Error(), "failed to checkout") {
			return fmt.Errorf("failed to switch version for '%s': %w", toolName, err)
		} else if strings.Contains(err.Error(), "failed to write .getgit file") {
			return fmt.Errorf("failed to update configuration for '%s': %w", toolName, err)
		}
		return fmt.Errorf("failed to update '%s': %w", toolName, err)
	}

	// Update tool configuration
	if err := rm.WriteToolConfig(toolName, selectedMatch.Source.GetName(), "release", selectedMatch.Repo.Load); err != nil {
		return fmt.Errorf("failed to write tool configuration: %w", err)
	}

	return nil
}

func upgradeAllTools(sm *sources.SourceManager, rm *repository.Manager, workDir string) error {
	// Create output manager for spinner
	om := repository.NewOutputManager(verbose)

	// Get list of installed tools
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return fmt.Errorf("failed to read work directory: %w", err)
	}

	var errors []string
	skipped := 0
	updated := 0
	total := 0

	// First count total tools to check
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".git" {
			continue
		}
		toolPath := filepath.Join(workDir, entry.Name())
		if _, err := os.Stat(filepath.Join(toolPath, ".git")); err != nil {
			continue
		}
		total++
	}

	if total == 0 {
		om.PrintInfo("No tools found to upgrade.")
		return nil
	}

	om.PrintInfo(fmt.Sprintf("Found %d tools to check", total))

	// Now process each tool
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".git" {
			continue
		}

		toolPath := filepath.Join(workDir, entry.Name())
		if _, err := os.Stat(filepath.Join(toolPath, ".git")); err != nil {
			continue
		}

		// Check if tool uses edge updates
		getgitFile, err := getgitfile.ReadFromRepo(toolPath)
		if err != nil && !os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("%s: failed to read .getgit file - %v", entry.Name(), err))
			om.PrintError(fmt.Sprintf("%s: failed to read .getgit file - %v", entry.Name(), err))
			continue
		}

		useEdge := getgitFile != nil && getgitFile.UpdateTrain == "edge"

		// Start processing with spinner
		om.StartStage(fmt.Sprintf("Checking %s (%d/%d)", entry.Name(), updated+skipped+1, total))

		// Try to upgrade the tool
		err = upgradeSpecificTool(sm, rm, entry.Name(), workDir)

		// Stop spinner and clear line before showing any status
		om.StopStage()

		// Print appropriate status message
		if err != nil {
			if err.Error() == fmt.Sprintf("tool '%s' is already up to date", entry.Name()) {
				skipped++
				om.PrintStatus(fmt.Sprintf("%s: already up to date", entry.Name()))
			} else {
				errors = append(errors, fmt.Sprintf("%s: %v", entry.Name(), err))
				om.PrintError(fmt.Sprintf("%s: upgrade failed - %v", entry.Name(), err))
			}
		} else {
			updated++
			if useEdge {
				om.PrintStatus(fmt.Sprintf("%s: updated to latest commit", entry.Name()))
			} else {
				om.PrintStatus(fmt.Sprintf("%s: updated successfully", entry.Name()))
			}
		}
	}

	// Print summary with a blank line before it
	if len(errors) > 0 {
		om.PrintInfo("\nErrors occurred during upgrade:")
		for _, err := range errors {
			om.PrintError(err)
		}
		om.PrintInfo("") // Add blank line before summary
	}

	om.PrintInfo(fmt.Sprintf("Summary: %d updated, %d skipped, %d failed", updated, skipped, len(errors)))

	if len(errors) > 0 {
		return fmt.Errorf("%d tools failed to upgrade", len(errors))
	}
	return nil
}
