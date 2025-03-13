package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/repository"
	"github.com/traberph/getgit/pkg/sources"
)

const (
	colorGreen  = "\033[32m"
	colorOrange = "\033[31m"
	colorReset  = "\033[0m"
)

var (
	installedOnly bool
	veryVerbose   bool
)

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
  --installed, -i      Show only installed tools
  --verbose, -v       Show all fields (build commands, executables, etc.) instead of just name and URL
  --very-verbose, -V  Show all fields including load command`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInfo,
}

func init() {
	infoCmd.Flags().BoolVarP(&installedOnly, "installed", "i", false, "Show only installed tools")
	infoCmd.Flags().BoolVarP(&veryVerbose, "very-verbose", "V", false, "Show all fields including load command")

	// Add completion support
	infoCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Create a new source manager
		sm, err := sources.NewSourceManager()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		defer sm.Close()

		// Get work directory for checking installed tools
		workDir, err := config.GetWorkDir()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Create a new repository manager
		rm, err := repository.NewManager(workDir, false)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		defer rm.Close()

		// Get all available tools
		repos, err := sm.ListRepositories()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Build list of tools based on --installed flag
		var suggestions []string
		for _, repo := range repos {
			status := rm.GetRepoStatus(repo)
			if installedOnly && status.Installed {
				suggestions = append(suggestions, repo.Name)
			} else if !installedOnly {
				suggestions = append(suggestions, repo.Name)
			}
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	rootCmd.AddCommand(infoCmd)
}

func runInfo(cmd *cobra.Command, args []string) error {
	// Create a new source manager
	sm, err := sources.NewSourceManager()
	if err != nil {
		return fmt.Errorf("failed to create source manager: %w", err)
	}
	defer sm.Close()

	// Get work directory for checking installed tools
	workDir, err := config.GetWorkDir()
	if err != nil {
		return fmt.Errorf("failed to get work directory: %w", err)
	}

	// Create a new repository manager
	rm, err := repository.NewManager(workDir, false)
	if err != nil {
		return fmt.Errorf("failed to create repository manager: %w", err)
	}
	defer rm.Close()

	// Use tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.StripEscape)
	defer w.Flush()

	if len(args) == 0 {
		// List all repositories
		repos, err := sm.ListRepositories()
		if err != nil {
			return fmt.Errorf("failed to list tools: %w", err)
		}

		if len(repos) == 0 {
			return fmt.Errorf("no tools found in the index")
		}

		// Get unique repositories based on installation status
		uniqueTools := rm.GetUniqueRepos(repos, installedOnly)

		// Convert map to slice for output
		var statusList []repository.RepoStatus
		for _, status := range uniqueTools {
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
			rm.PrintRepoInfo(w, status, verbose, veryVerbose)
			if i < len(statusList)-1 {
				if veryVerbose {
					fmt.Fprintf(w, "\n\n")
				} else {
					fmt.Fprintf(w, "\n")
				}
			}
		}
		return nil
	}

	// Find specific repository
	toolName := args[0]
	repos, err := sm.FindRepository(toolName)
	if err != nil {
		return fmt.Errorf("failed to find tool information: %w", err)
	}

	if len(repos) == 0 {
		return fmt.Errorf("no information found for tool '%s'", toolName)
	}

	// Get unique repositories based on installation status
	uniqueTools := rm.GetUniqueRepos(repos, installedOnly)

	// Convert map to slice for output
	var statusList []repository.RepoStatus
	for _, status := range uniqueTools {
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
		rm.PrintRepoInfo(w, status, verbose, veryVerbose)
		if i < len(statusList)-1 {
			if veryVerbose {
				fmt.Fprintf(w, "\n\n")
			} else {
				fmt.Fprintf(w, "\n")
			}
		}
	}

	return nil
}
