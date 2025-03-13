package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/getgitfile"
	"github.com/traberph/getgit/pkg/repository"
	"github.com/traberph/getgit/pkg/shell"
	"github.com/traberph/getgit/pkg/sources"
)

var (
	release bool
	edge    bool // Use edge update train
)

// promptSourceSelection prompts the user to select a source from multiple matches
func promptSourceSelection(matches []sources.RepoMatch) (*sources.RepoMatch, error) {
	fmt.Printf("\nTool found in multiple sources. Please select one:\n")
	for i, match := range matches {
		fmt.Printf("%d) %s (from source: %s)\n", i+1, match.Repo.Name, match.Source.GetName())
		fmt.Printf("   URL: %s\n", match.Repo.URL)
		fmt.Printf("   Build command: %s\n", match.Repo.Build)
		fmt.Printf("   Executable: %s\n\n", match.Repo.Executable)
	}

	var selection int
	fmt.Print("Enter number (1-" + fmt.Sprint(len(matches)) + "): ")
	_, err := fmt.Scanf("%d", &selection)
	if err != nil || selection < 1 || selection > len(matches) {
		return nil, fmt.Errorf("invalid selection")
	}

	return &matches[selection-1], nil
}

// installTool handles the installation of a tool
func installTool(sm *sources.SourceManager, toolName string, cmd *cobra.Command) error {
	// Get work directory
	workDir, err := config.GetWorkDir()
	if err != nil {
		return fmt.Errorf("failed to get work directory: %w", err)
	}

	// Create repository manager
	rm, err := repository.NewManager(workDir, verbose)
	if err != nil {
		return fmt.Errorf("failed to create repository manager: %w", err)
	}
	defer rm.Close()

	rm.Output.PrintInfo(fmt.Sprintf("Starting installation of '%s'...\n", toolName))

	// Find tool in sources
	matches := sm.FindRepo(toolName)
	if len(matches) == 0 {
		return fmt.Errorf("tool '%s' not found in any source", toolName)
	}

	// Print matches
	fmt.Printf("Found %d matches for '%s':\n", len(matches), toolName)
	for i, match := range matches {
		fmt.Printf("%d. %s (from %s)\n", i+1, match.Repo.Name, match.Source.GetName())
	}

	isExistingInstall := false
	var getgitFile *getgitfile.GetGitFile
	var selectedMatch *sources.RepoMatch

	// Check for existing installation using manager
	isExistingInstall, err = rm.IsToolInstalled(toolName)
	if err != nil {
		return fmt.Errorf("failed to check existing installation: %w", err)
	}

	if isExistingInstall {
		getgitFile, err = rm.Getgit.Read(toolName)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read tool configuration: %w", err)
		}

		// Find matching source if .getgit file exists
		if getgitFile != nil {
			for _, match := range matches {
				if match.Source.GetName() == getgitFile.SourceName {
					selectedMatch = &match
					break
				}
			}
			if selectedMatch == nil {
				return fmt.Errorf("source '%s' specified in .getgit file no longer contains this tool", getgitFile.SourceName)
			}
		}
	}

	// Select source if not already determined
	if selectedMatch == nil {
		if len(matches) == 1 {
			selectedMatch = &matches[0]
		} else {
			var err error
			selectedMatch, err = promptSourceSelection(matches)
			if err != nil {
				return fmt.Errorf("source selection failed: %w", err)
			}
		}
	}

	// Validate and normalize URL
	repoURL, err := sm.NormalizeAndValidateURL(selectedMatch.Repo.URL)
	if err != nil {
		return fmt.Errorf("failed to validate URL: %w", err)
	}
	selectedMatch.Repo.URL = repoURL

	// Determine update train
	newUpdateTrain, _ := rm.Getgit.GetUpdateTrain(toolName, edge, release)
	useEdgeTrain := newUpdateTrain == getgitfile.UpdateTrainEdge

	// For existing installations, check if we need to update
	if isExistingInstall {
		// Determine if update train has changed
		updateTrainChanged := false
		if getgitFile != nil {
			updateTrainChanged = getgitFile.UpdateTrain != newUpdateTrain
		} else {
			// If no .getgit file exists, treat it as a change if we're switching to edge
			updateTrainChanged = useEdgeTrain
		}

		// Show update train change message before any other operations
		if updateTrainChanged {
			rm.Output.StopStage() // Stop any existing spinner
			if useEdgeTrain {
				rm.Output.PrintInfo(fmt.Sprintf("Switching '%s' to edge (latest commit)", toolName))
			} else {
				rm.Output.PrintInfo(fmt.Sprintf("Switching '%s' to release (latest tag)", toolName))
			}
			rm.Output.PrintInfo("Checking for updates...")

			// Write .getgit file first
			if err := rm.Getgit.Write(toolName, selectedMatch.Source.GetName(), newUpdateTrain, selectedMatch.Repo.Load); err != nil {
				return fmt.Errorf("failed to write tool configuration: %w", err)
			}
			rm.Output.PrintStatus("Updated tool configuration")

			// Now update the package
			if err := rm.UpdatePackage(repository.Repository{
				Name:       selectedMatch.Repo.Name,
				URL:        repoURL,
				Build:      selectedMatch.Repo.Build,
				Executable: selectedMatch.Repo.Executable,
				Load:       selectedMatch.Repo.Load,
				UseEdge:    useEdgeTrain,
				SkipBuild:  skipBuild,
				SourceName: selectedMatch.Source.GetName(),
			}); err != nil {
				return fmt.Errorf("failed to install tool: %w", err)
			}
			rm.Output.PrintInfo(fmt.Sprintf("Tool '%s' configuration updated successfully!", toolName))
			return nil
		}

		// Check for updates based on the update train
		hasUpdates := false
		var latestTag string
		var err error

		// Only check for updates if the repository exists and has a .git directory
		if _, gitErr := os.Stat(filepath.Join(workDir, toolName, ".git")); gitErr == nil {
			hasUpdates, latestTag, err = checkForUpdates(rm, toolName, useEdgeTrain)
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			// Handle case when no updates are needed
			if !hasUpdates {
				rm.Output.PrintInfo(fmt.Sprintf("Tool '%s' is already up to date!", toolName))
				return nil
			}

			if !useEdgeTrain && latestTag != "" {
				rm.Output.PrintInfo(fmt.Sprintf("New version available: %s", latestTag))
			}
		}
	}

	// Update or install the package
	if !isExistingInstall {
		// For new installations, first clone the repository
		if _, err := rm.CloneOrUpdate(repoURL, toolName); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		// Write .getgit file
		if err := rm.Getgit.Write(toolName, selectedMatch.Source.GetName(), newUpdateTrain, selectedMatch.Repo.Load); err != nil {
			return fmt.Errorf("failed to write tool configuration: %w", err)
		}
	}

	// Now update the package (this will handle building and setting up the tool)
	if err := rm.UpdatePackage(repository.Repository{
		Name:       selectedMatch.Repo.Name,
		URL:        repoURL,
		Build:      selectedMatch.Repo.Build,
		Executable: selectedMatch.Repo.Executable,
		Load:       selectedMatch.Repo.Load,
		UseEdge:    useEdgeTrain,
		SkipBuild:  skipBuild,
		SourceName: selectedMatch.Source.GetName(),
	}); err != nil {
		return fmt.Errorf("failed to install tool: %w", err)
	}

	// Only update completion script for new installations
	if !isExistingInstall {
		if err := shell.UpdateCompletionScript(cmd); err != nil {
			rm.Output.PrintError(fmt.Sprintf("Warning: Failed to update completion script: %v", err))
		} else {
			rm.Output.PrintStatus("Updated shell completion")
		}
	}

	rm.Output.PrintInfo(fmt.Sprintf("\nInstallation of '%s' completed successfully!", toolName))
	return nil
}

var installCmd = &cobra.Command{
	Use:   "install <tool>",
	Short: "Install a tool",
	Long: `Installs a tool from a Git repository.

Clones the repository and sets up the tool according to its configuration.
If a tool exists in multiple sources, prompts for selection.

Examples:
  getgit install toolname        # Install from configured sources
  getgit install username/repo   # Install directly from GitHub

Flags:
  --release, -r    Install the latest tagged release (default)
  --edge, -e       Install the latest commit from the main branch
  --verbose, -v    Show detailed output during installation
  --skip-build, -s Skip the build step`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if edge && release {
			return fmt.Errorf("cannot specify both --release and --edge")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("please specify a tool to install")
		}

		sm, err := sources.NewSourceManager()
		if err != nil {
			return fmt.Errorf("failed to initialize source manager: %w", err)
		}
		defer sm.Close()

		if err := sm.LoadSources(); err != nil {
			return fmt.Errorf("failed to load sources: %w", err)
		}

		if len(sm.Sources) == 0 {
			sourcesDir, err := config.GetSourcesDir()
			if err != nil {
				return fmt.Errorf("failed to get sources directory: %w", err)
			}
			return fmt.Errorf("no sources configured. Add source files to %s", sourcesDir)
		}

		return installTool(sm, args[0], cmd)
	},
}

func init() {
	installCmd.Flags().BoolVarP(&release, "release", "r", false, "Install the latest tagged release")
	installCmd.Flags().BoolVarP(&edge, "edge", "e", false, "Use edge update train")

	// Add completion support
	installCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		sm, err := sources.NewSourceManager()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		defer sm.Close()

		if err := sm.LoadSources(); err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		repos, err := sm.ListRepositories()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var suggestions []string
		for _, repo := range repos {
			suggestions = append(suggestions, repo.Name)
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	rootCmd.AddCommand(installCmd)
}
