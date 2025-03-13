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

	// Always show this main info message
	rm.Output.PrintInfo(fmt.Sprintf("Starting installation of '%s'...", toolName))
	fmt.Println()

	// Find tool in sources
	matches := sm.FindRepo(toolName)
	if len(matches) == 0 {
		return fmt.Errorf("tool '%s' not found in any source", toolName)
	}

	// Show search results in verbose mode
	if rm.Output.IsVerbose() {
		rm.Output.PrintStatus(fmt.Sprintf("Found '%s' in %d source(s)", toolName, len(matches)))
	}

	isExistingInstall := false
	var getgitFile *getgitfile.GetGitFile
	var selectedMatch *sources.RepoMatch
	var repoURL string

	// Check for existing installation - technical detail, verbose only
	if rm.Output.IsVerbose() {
		rm.Output.StartStage("Checking for existing installation...")
	}

	isExistingInstall, err = rm.IsToolInstalled(toolName)
	if err != nil {
		if rm.Output.IsVerbose() {
			rm.Output.StopStage()
		}
		return fmt.Errorf("failed to check existing installation: %w", err)
	}

	if rm.Output.IsVerbose() {
		if isExistingInstall {
			rm.Output.PrintStatus(fmt.Sprintf("Found existing installation of '%s'", toolName))
		} else {
			rm.Output.PrintStatus(fmt.Sprintf("No existing installation found"))
		}
	}

	if isExistingInstall {
		// Reading tool configuration - technical detail, verbose only
		if rm.Output.IsVerbose() {
			rm.Output.StartStage("Reading tool configuration...")
		}

		getgitFile, err = rm.Getgit.Read(toolName)
		if err != nil && !os.IsNotExist(err) {
			if rm.Output.IsVerbose() {
				rm.Output.StopStage()
			}
			return fmt.Errorf("failed to read tool configuration: %w", err)
		}

		if rm.Output.IsVerbose() {
			if getgitFile != nil {
				rm.Output.PrintStatus("Configuration loaded")
			} else {
				rm.Output.PrintStatus("No configuration found")
			}
		}

		// Find matching source if .getgit file exists - technical detail, verbose only
		if getgitFile != nil {
			if rm.Output.IsVerbose() {
				rm.Output.StartStage("Finding source...")
			}

			for _, match := range matches {
				if match.Source.GetName() == getgitFile.SourceName {
					selectedMatch = &match
					break
				}
			}

			if selectedMatch == nil {
				if rm.Output.IsVerbose() {
					rm.Output.StopStage()
				}
				return fmt.Errorf("source '%s' specified in configuration no longer contains this tool", getgitFile.SourceName)
			}

			if rm.Output.IsVerbose() {
				rm.Output.PrintStatus(fmt.Sprintf("Using source: %s", selectedMatch.Source.GetName()))
			}
		}
	}

	// Select source if not already determined
	if selectedMatch == nil {
		if len(matches) == 1 {
			selectedMatch = &matches[0]
			if rm.Output.IsVerbose() {
				rm.Output.PrintInfo(fmt.Sprintf("Using source: %s", selectedMatch.Source.GetName()))
			}
		} else {
			// Always show this for multiple sources as it requires user input
			rm.Output.PrintInfo("Multiple sources found, please select one:")
			var err error
			selectedMatch, err = promptSourceSelection(matches)
			if err != nil {
				return fmt.Errorf("source selection failed: %w", err)
			}
			rm.Output.PrintInfo(fmt.Sprintf("Selected source: %s", selectedMatch.Source.GetName()))
		}
	}

	// Validate URL - technical detail, verbose only
	if rm.Output.IsVerbose() {
		rm.Output.StartStage("Validating repository URL...")
	}

	var urlErr error
	repoURL, urlErr = sm.NormalizeAndValidateURL(selectedMatch.Repo.URL)
	if urlErr != nil {
		if rm.Output.IsVerbose() {
			rm.Output.StopStage()
		}
		return fmt.Errorf("failed to validate URL: %w", urlErr)
	}
	selectedMatch.Repo.URL = repoURL

	if rm.Output.IsVerbose() {
		rm.Output.PrintStatus("URL validated")
	}

	// Determine update train - technical detail, verbose only
	newUpdateTrain, _ := rm.Getgit.GetUpdateTrain(toolName, edge, release)
	useEdgeTrain := newUpdateTrain == getgitfile.UpdateTrainEdge

	// Display update train info early if it's explicitly set via flags
	if edge || release {
		if useEdgeTrain {
			rm.Output.PrintInfo(fmt.Sprintf("Switching '%s' to edge (latest commit)", toolName))
		} else {
			rm.Output.PrintInfo(fmt.Sprintf("Switching '%s' to release (latest tag)", toolName))
		}
	}

	if rm.Output.IsVerbose() {
		if useEdgeTrain {
			rm.Output.PrintInfo("Using edge update train (latest commit)")
		} else {
			rm.Output.PrintInfo("Using release update train (latest tag)")
		}
	}

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
		if updateTrainChanged && !edge && !release {
			// Show update train changes if not already shown at the top
			if useEdgeTrain {
				rm.Output.PrintInfo(fmt.Sprintf("Switching '%s' to edge (latest commit)", toolName))
			} else {
				rm.Output.PrintInfo(fmt.Sprintf("Switching '%s' to release (latest tag)", toolName))
			}
		}

		if updateTrainChanged {
			// Write .getgit file first - technical detail, verbose only
			if rm.Output.IsVerbose() {
				rm.Output.StartStage("Updating configuration...")
			}

			if err := rm.Getgit.Write(toolName, selectedMatch.Source.GetName(), newUpdateTrain, selectedMatch.Repo.Load); err != nil {
				if rm.Output.IsVerbose() {
					rm.Output.StopStage()
				}
				return fmt.Errorf("failed to write tool configuration: %w", err)
			}

			if rm.Output.IsVerbose() {
				rm.Output.PrintStatus("Configuration updated")
			}

			// Now update the package - always show this
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

			// Add empty line before final success message
			fmt.Println()
			rm.Output.PrintInfo(fmt.Sprintf("Tool '%s' updated successfully!", toolName))
			return nil
		}

		// Check for updates - always show this
		hasUpdates := false
		var latestTag string

		// Only check for updates if the repository exists and has a .git directory
		if _, gitErr := os.Stat(filepath.Join(workDir, toolName, ".git")); gitErr == nil {
			rm.Output.StartStage("Checking for updates...")

			if useEdgeTrain {
				// For edge update train, check for new commits
				hasUpdates, err = rm.HasEdgeUpdates(filepath.Join(workDir, toolName))
				if err != nil {
					rm.Output.StopStage()
					return fmt.Errorf("failed to check for edge updates: %w", err)
				}

				if hasUpdates {
					rm.Output.PrintStatus("New updates available")
				} else {
					rm.Output.PrintStatus("No updates available")
				}
			} else {
				// For release update train, check for new tags
				hasTags, err := rm.HasTags(filepath.Join(workDir, toolName))
				if err != nil {
					rm.Output.StopStage()
					return fmt.Errorf("failed to check for tags: %w", err)
				}

				if hasTags {
					currentTag, _ := rm.GetCurrentTag(filepath.Join(workDir, toolName))
					latestTag, err = rm.GetLatestTag(filepath.Join(workDir, toolName))
					if err != nil {
						rm.Output.StopStage()
						return fmt.Errorf("failed to get latest tag: %w", err)
					}

					if currentTag != latestTag {
						hasUpdates = true
						rm.Output.PrintStatus(fmt.Sprintf("New version available: %s", latestTag))
					} else {
						rm.Output.PrintStatus("Already at latest version")
					}
				} else {
					rm.Output.PrintStatus("No release tags found")
				}
			}

			// Handle case when no updates are needed
			if !hasUpdates {
				fmt.Println()
				rm.Output.PrintInfo(fmt.Sprintf("Tool '%s' is already up to date!", toolName))
				return nil
			}
		}
	} else {
		// For new installations, first clone the repository - always show this
		rm.Output.StartStage(fmt.Sprintf("Cloning repository..."))
		if _, err := rm.CloneOrUpdate(repoURL, toolName); err != nil {
			rm.Output.StopStage()
			return fmt.Errorf("failed to clone repository: %w", err)
		}
		rm.Output.PrintStatus("Repository cloned")

		// Write .getgit file - technical detail, verbose only
		if rm.Output.IsVerbose() {
			rm.Output.StartStage("Creating configuration...")
		}

		if err := rm.Getgit.Write(toolName, selectedMatch.Source.GetName(), newUpdateTrain, selectedMatch.Repo.Load); err != nil {
			if rm.Output.IsVerbose() {
				rm.Output.StopStage()
			}
			return fmt.Errorf("failed to write tool configuration: %w", err)
		}

		if rm.Output.IsVerbose() {
			rm.Output.PrintStatus("Configuration created")
		}
	}

	// Now update the package - always show this
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

	// Only update completion script for new installations - always show this
	if !isExistingInstall {
		rm.Output.StartStage("Updating shell completion...")
		if err := shell.UpdateCompletionScript(cmd); err != nil {
			rm.Output.StopStage()
			rm.Output.PrintError(fmt.Sprintf("Warning: Failed to update completion script: %v", err))
		} else {
			rm.Output.PrintStatus("Shell completion updated")
		}
	}

	// Add empty line before final success message
	fmt.Println()
	// Final success message - always show this
	rm.Output.PrintInfo(fmt.Sprintf("Installation of '%s' completed successfully!", toolName))
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
