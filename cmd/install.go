package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/getgitfile"
	"github.com/traberph/getgit/pkg/index"
	"github.com/traberph/getgit/pkg/repository"
	"github.com/traberph/getgit/pkg/sources"
)

var (
	force     bool
	edge      bool
	release   bool
	skipBuild bool
)

// promptSourceSelection prompts the user to select a source from multiple matches
func promptSourceSelection(matches []sources.RepoMatch) (*sources.RepoMatch, error) {
	fmt.Printf("\nTool found in multiple sources. Please select one:\n")
	for i, match := range matches {
		fmt.Printf("%d) %s (from source: %s)\n", i+1, match.Repo.Name, match.Source.Name)
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

// getUpdateTrain determines which update train to use based on flags and existing .getgit file
func getUpdateTrain(getgitFile *getgitfile.GetGitFile) string {
	// If flags are specified, they take precedence
	if edge {
		return "edge"
	}
	if release {
		return "release"
	}

	// If .getgit file exists, use its preference
	if getgitFile != nil {
		return getgitFile.UpdateTrain
	}

	// Default to release
	return "release"
}

// installTool handles the installation of a tool
func installTool(sm *sources.SourceManager, toolName string) error {
	matches := sm.FindRepo(toolName)
	if len(matches) == 0 {
		return fmt.Errorf("tool '%s' not found in any source", toolName)
	}

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

	rm.Output.PrintInfo(fmt.Sprintf("Starting installation of '%s'...\n", toolName))

	var selectedMatch *sources.RepoMatch
	repoPath := filepath.Join(workDir, toolName)

	// Check for existing installation and .getgit file
	var getgitFile *getgitfile.GetGitFile
	if _, err := os.Stat(repoPath); err == nil {
		getgitFile, err = getgitfile.ReadFromRepo(repoPath)
		if err != nil {
			return fmt.Errorf("failed to read .getgit file: %w", err)
		}

		if getgitFile != nil {
			// Find the match corresponding to the source in .getgit file
			for _, match := range matches {
				if match.Source.Name == getgitFile.SourceName {
					selectedMatch = &match
					break
				}
			}
			if selectedMatch == nil {
				return fmt.Errorf("source '%s' specified in .getgit file no longer contains this tool", getgitFile.SourceName)
			}
		}
	}

	// If no .getgit file or matching source found, handle source selection
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

	// Determine collections this tool belongs to
	var collections []string
	for _, collection := range selectedMatch.Source.Collections {
		for _, repoName := range collection.Repos {
			if repoName == selectedMatch.Repo.Name {
				collections = append(collections, collection.Name)
			}
		}
	}

	// Validate URL and permissions
	repoURL := selectedMatch.Repo.URL
	if strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://") {
		// For HTTP(S) URLs, validate the host is allowed
		if err := selectedMatch.Source.ValidateURLHost(repoURL); err != nil {
			return fmt.Errorf("URL host not allowed: %w", err)
		}
	} else {
		// Assume GitHub repository if not HTTP(S)
		// Clean the URL by trimming any prefixes
		cleanURL := strings.TrimPrefix(repoURL, "github.com/")
		cleanURL = strings.TrimPrefix(cleanURL, "https://github.com/")
		cleanURL = strings.TrimPrefix(cleanURL, "http://github.com/")
		repoURL = fmt.Sprintf("https://github.com/%s.git", cleanURL)
		selectedMatch.Repo.URL = repoURL
	}

	// Validate permissions for the repository
	if err := selectedMatch.Source.ValidatePermissions(selectedMatch.Repo); err != nil {
		return fmt.Errorf("permission validation failed: %w", err)
	}

	// Determine update train
	updateTrain := getUpdateTrain(getgitFile)
	useEdge := updateTrain == "edge"

	// Install the tool
	if err := rm.UpdatePackage(repository.Repository{
		Name:       selectedMatch.Repo.Name,
		URL:        repoURL,
		Build:      selectedMatch.Repo.Build,
		Executable: selectedMatch.Repo.Executable,
		Load:       selectedMatch.Repo.Load,
		UseEdge:    useEdge,
		SkipBuild:  skipBuild,
		SourceName: selectedMatch.Source.Name,
	}); err != nil {
		return fmt.Errorf("failed to install tool: %w", err)
	}
	rm.Output.PrintStatus("Repository cloned successfully")

	if selectedMatch.Repo.Build != "" && !skipBuild {
		rm.Output.PrintStatus("Build completed")
	}

	if selectedMatch.Repo.Executable != "" {
		rm.Output.PrintStatus("Created alias")
	}

	// Write or update .getgit file
	if len(matches) > 1 || getgitFile != nil || len(collections) > 0 {
		if err := getgitfile.WriteToRepo(repoPath, selectedMatch.Source.Name, updateTrain, collections, selectedMatch.Repo.Load); err != nil {
			return fmt.Errorf("failed to write .getgit file: %w", err)
		}
		rm.Output.PrintStatus("Created .getgit file")
		if len(collections) > 0 {
			rm.Output.PrintInfo(fmt.Sprintf("Tool belongs to collections: %v", collections))
		}
	}

	// Update completion script
	if err := updateCompletionScript(); err != nil {
		rm.Output.PrintError(fmt.Sprintf("Warning: Failed to update completion script: %v", err))
	} else {
		rm.Output.PrintStatus("Updated shell completion")
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
		// Validate that only one of --release or --edge is specified
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

		return installTool(sm, args[0])
	},
}

func init() {
	installCmd.Flags().BoolVarP(&force, "force", "f", false, "Force installation without confirmation")
	installCmd.Flags().BoolVarP(&release, "release", "r", false, "Install the latest tagged release (default)")
	installCmd.Flags().BoolVarP(&edge, "edge", "e", false, "Install the latest commit from the main branch")
	installCmd.Flags().BoolVarP(&skipBuild, "skip-build", "s", false, "Skip the build step")

	// Add completion support
	installCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

		// Build list of available tools that are not installed
		var suggestions []string
		for _, repo := range repos {
			// Check if tool is already installed
			toolPath := filepath.Join(workDir, repo.Name)
			if _, err := os.Stat(toolPath); os.IsNotExist(err) {
				suggestions = append(suggestions, repo.Name)
			}
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	rootCmd.AddCommand(installCmd)
}
