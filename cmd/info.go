package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/getgitfile"
	"github.com/traberph/getgit/pkg/index"
)

const (
	colorGreen  = "\033[32m"
	colorOrange = "\033[31m"
	colorReset  = "\033[0m"
)

var (
	installedOnly bool
	veryVerbose   bool
	correlation   bool
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
  --very-verbose, -V  Show all fields including load command
  --correlation       Show correlation between tools`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInfo,
}

func init() {
	infoCmd.Flags().BoolVarP(&installedOnly, "installed", "i", false, "Show only installed tools")
	infoCmd.Flags().BoolVarP(&veryVerbose, "very-verbose", "V", false, "Show all fields including load command")
	infoCmd.Flags().BoolVar(&correlation, "correlation", false, "Show correlation between tools")

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
	// Helper function to format multi-line values
	formatMultiLine := func(value string) string {
		if strings.Contains(value, "\n") {
			lines := strings.Split(strings.TrimSpace(value), "\n")
			// First line goes after the tab, subsequent lines are indented
			for i := range lines {
				lines[i] = strings.TrimSpace(lines[i])
			}
			return lines[0] + "\n\t" + strings.Join(lines[1:], "\n\t")
		}
		return strings.TrimSpace(value)
	}

	// Basic info always shown
	fmt.Fprintf(w, "name:\t%s\n", repo.Name)
	fmt.Fprintf(w, "repository url:\t%s\n", repo.URL)
	if repo.Installed {
		fmt.Fprintf(w, "status:\t%sinstalled%s\n", colorGreen, colorReset)
	} else {
		fmt.Fprintf(w, "status:\tnot installed\n")
	}

	// Additional info with -v
	if verbose || veryVerbose {
		if repo.Installed {
			if repo.UpdateTrain == "edge" {
				fmt.Fprintf(w, "update train:\t%sedge%s\n", colorOrange, colorReset)
			} else {
				fmt.Fprintf(w, "update train:\t%s\n", repo.UpdateTrain)
			}
		}
		fmt.Fprintf(w, "source name:\t%s\n", repo.SourceName)
	}

	// Full info with -V
	if veryVerbose {
		if repo.Build != "" {
			fmt.Fprintf(w, "build command:\t%s\n", formatMultiLine(repo.Build))
		}
		if repo.Executable != "" {
			fmt.Fprintf(w, "executable:\t%s\n", formatMultiLine(repo.Executable))
		}
		fmt.Fprintf(w, "source file:\t%s\n", repo.SourceFile)
		if repo.Installed {
			fmt.Fprintf(w, "install path:\t%s\n", formatMultiLine(repo.InstallPath))
		}
		if repo.Load != "" {
			fmt.Fprintf(w, "load command:\t%s\n", formatMultiLine(repo.Load))
		}
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.StripEscape)
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

		// Map to track unique tool names and their actual installation status
		uniqueTools := make(map[string]repoStatus)

		// Convert to repo status and filter if needed
		for _, repo := range repos {
			status := getRepoStatus(repo, workDir)

			// If tool is installed and there's a conflict (tool already exists in map)
			if status.Installed {
				if existing, exists := uniqueTools[repo.Name]; exists {
					// Check .getgit file to determine which one is actually installed
					repoPath := filepath.Join(workDir, repo.Name)
					if getgitFile, err := getgitfile.ReadFromRepo(repoPath); err == nil && getgitFile != nil {
						// If .getgit file exists, use the source specified in it
						if getgitFile.SourceName == repo.SourceName {
							uniqueTools[repo.Name] = status
						} else if getgitFile.SourceName == existing.SourceName {
							// Keep existing if it matches .getgit file
							continue
						}
					}
				} else {
					uniqueTools[repo.Name] = status
				}
			} else if !installedOnly {
				// For non-installed tools, only add if not showing installed only
				if _, exists := uniqueTools[repo.Name]; !exists {
					uniqueTools[repo.Name] = status
				}
			}
		}

		// Convert map to slice for output
		var statusList []repoStatus
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
			printRepoInfo(w, status)
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
	repos, err := indexManager.FindRepository(toolName)
	if err != nil {
		return fmt.Errorf("failed to find tool information: %w", err)
	}

	if len(repos) == 0 {
		return fmt.Errorf("no information found for tool '%s'", toolName)
	}

	// Map to track unique tool matches and their actual installation status
	uniqueTools := make(map[string]repoStatus)

	// Convert to repo status and filter if needed
	for _, repo := range repos {
		status := getRepoStatus(repo, workDir)

		// If tool is installed and there's a conflict
		if status.Installed {
			if existing, exists := uniqueTools[repo.Name]; exists {
				// Check .getgit file to determine which one is actually installed
				repoPath := filepath.Join(workDir, repo.Name)
				if getgitFile, err := getgitfile.ReadFromRepo(repoPath); err == nil && getgitFile != nil {
					// If .getgit file exists, use the source specified in it
					if getgitFile.SourceName == repo.SourceName {
						uniqueTools[repo.Name] = status
					} else if getgitFile.SourceName == existing.SourceName {
						// Keep existing if it matches .getgit file
						continue
					}
				}
			} else {
				uniqueTools[repo.Name] = status
			}
		} else if !installedOnly {
			// For non-installed tools, only add if not showing installed only
			if _, exists := uniqueTools[repo.Name]; !exists {
				uniqueTools[repo.Name] = status
			}
		}
	}

	// Convert map to slice for output
	var statusList []repoStatus
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
		printRepoInfo(w, status)
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
