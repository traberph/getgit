package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/getgitfile"
	"github.com/traberph/getgit/pkg/index"
)

var installedOnly bool

var infoCmd = &cobra.Command{
	Use:   "info [tool]",
	Short: "Display information about tools",
	Long: `Displays information about available or installed tools.

Without arguments, lists all available tools.
With a tool name, shows detailed information about that specific tool.

Examples:
  getgit info           # List all available tools
  getgit info toolname # Show details about a specific tool

Flags:
  --installed, -i  Show only installed tools
  --verbose, -v   Show all fields (build commands, executables, etc.) instead of just name and URL`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInfo,
}

func init() {
	infoCmd.Flags().BoolVarP(&installedOnly, "installed", "i", false, "Show only installed tools")
	infoCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show all fields instead of just name and URL")

	// Add completion support
	infoCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

		// Build list of tools based on --installed flag
		var suggestions []string
		for _, repo := range repos {
			// Check if tool is installed
			toolPath := filepath.Join(workDir, repo.Name)
			isInstalled := false
			if _, err := os.Stat(toolPath); err == nil {
				isInstalled = true
			}

			// Add to suggestions based on --installed flag
			if installedOnly && isInstalled {
				suggestions = append(suggestions, repo.Name)
			} else if !installedOnly {
				suggestions = append(suggestions, repo.Name)
			}
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	rootCmd.AddCommand(infoCmd)
}

type repoStatus struct {
	index.RepoInfo
	Installed   bool
	UpdateTrain string
	InstallPath string
	Load        string // Command to be executed when loading the tool
}

func printRepoInfo(w *tabwriter.Writer, repo repoStatus) {
	fmt.Fprintf(w, "Name:\t%s\n", repo.Name)
	fmt.Fprintf(w, "Repository URL:\t%s\n", repo.URL)
	if verbose {
		fmt.Fprintf(w, "Build Command:\t%s\n", repo.Build)
		fmt.Fprintf(w, "Executable:\t%s\n", repo.Executable)
		if repo.Load != "" {
			fmt.Fprintf(w, "Load Command:\t%s\n", repo.Load)
		}
		fmt.Fprintf(w, "Source Name:\t%s\n", repo.SourceName)
		fmt.Fprintf(w, "Source File:\t%s\n", repo.SourceFile)
		if repo.Installed {
			fmt.Fprintf(w, "Status:\tInstalled\n")
			fmt.Fprintf(w, "Install Path:\t%s\n", repo.InstallPath)
			fmt.Fprintf(w, "Update Train:\t%s\n", repo.UpdateTrain)
		} else {
			fmt.Fprintf(w, "Status:\tNot installed\n")
		}
	} else if repo.Installed {
		fmt.Fprintf(w, "Status:\tInstalled\n")
	}
}

func getRepoStatus(repo index.RepoInfo, workDir string) repoStatus {
	status := repoStatus{
		RepoInfo: repo,
		Load:     repo.Load, // Explicitly copy the Load field
	}

	// Check if tool is installed
	repoPath := filepath.Join(workDir, repo.Name)
	if _, err := os.Stat(repoPath); err == nil {
		status.Installed = true
		status.InstallPath = repoPath

		// Check for .getgit file
		if getgitFile, err := getgitfile.ReadFromRepo(repoPath); err == nil && getgitFile != nil {
			status.UpdateTrain = getgitFile.UpdateTrain
		} else {
			status.UpdateTrain = "release" // Default to release if no .getgit file
		}
	}

	return status
}

func runInfo(cmd *cobra.Command, args []string) error {
	// Create a new index manager
	indexManager, err := index.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create index manager: %w", err)
	}
	defer indexManager.Close()

	// Get work directory for checking installed tools
	workDir, err := config.GetWorkDir()
	if err != nil {
		return fmt.Errorf("failed to get work directory: %w", err)
	}

	// Use tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	if len(args) == 0 {
		// List all repositories
		repos, err := indexManager.ListRepositories()
		if err != nil {
			return fmt.Errorf("failed to list tools: %w", err)
		}

		if len(repos) == 0 {
			return fmt.Errorf("no tools found in the index")
		}

		// Convert to repo status and filter if needed
		var statusList []repoStatus
		for _, repo := range repos {
			status := getRepoStatus(repo, workDir)
			if !installedOnly || status.Installed {
				statusList = append(statusList, status)
			}
		}

		if len(statusList) == 0 {
			if installedOnly {
				return fmt.Errorf("no installed tools found")
			}
			return fmt.Errorf("no tools found in the index")
		}

		fmt.Printf("Found %d tools:\n\n", len(statusList))

		for i, status := range statusList {
			printRepoInfo(w, status)
			if i < len(statusList)-1 {
				fmt.Fprintf(w, "\n")
			}
		}
		return nil
	}

	// Find specific repository
	toolName := args[0]
	repos, err := indexManager.FindRepository(toolName)
	if err != nil {
		return fmt.Errorf("failed to find tool information: %w", err)
	}

	if len(repos) == 0 {
		return fmt.Errorf("no information found for tool '%s'", toolName)
	}

	// Convert to repo status and filter if needed
	var statusList []repoStatus
	for _, repo := range repos {
		status := getRepoStatus(repo, workDir)
		if !installedOnly || status.Installed {
			statusList = append(statusList, status)
		}
	}

	if len(statusList) == 0 {
		if installedOnly {
			return fmt.Errorf("tool '%s' is not installed", toolName)
		}
		return fmt.Errorf("no information found for tool '%s'", toolName)
	}

	if len(statusList) > 1 {
		fmt.Printf("Found %d entries for tool '%s':\n\n", len(statusList), toolName)
	}

	for i, status := range statusList {
		printRepoInfo(w, status)
		if i < len(statusList)-1 {
			fmt.Fprintf(w, "\n")
		}
	}

	return nil
}
