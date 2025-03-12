package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/index"
	"github.com/traberph/getgit/pkg/repository"
	"github.com/traberph/getgit/pkg/sources"
)

var (
	forceUpdate bool
	dryRun      bool
	indexOnly   bool
)

// promptUser asks the user for confirmation
func promptUser(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}

// listSources displays all available sources and their repositories
func listSources(sm *sources.SourceManager) {
	fmt.Println("\nAvailable sources and tools:")
	for _, source := range sm.Sources {
		fmt.Printf("\n[%s] Origin: %s\n", source.Name, source.Origin)
		if len(source.Repos) == 0 {
			fmt.Println("  No tools configured")
			continue
		}
		for _, repo := range source.Repos {
			fmt.Printf("  - %s\n", repo.Name)
			fmt.Printf("    URL: %s\n", repo.URL)
			if repo.Build != "" {
				fmt.Printf("    Build command: %s\n", repo.Build)
			}
			if repo.Executable != "" {
				fmt.Printf("    Executable: %s\n", repo.Executable)
			}
			if repo.Load != "" {
				fmt.Printf("    Load command: %s\n", repo.Load)
			}
		}
	}
}

// updateSource handles updating a single source
func updateSource(sm *sources.SourceManager, source *sources.Source) error {
	if source.Origin == "" {
		fmt.Printf("✓ Source '%s' has no origin, skipping\n", source.Name)
		return nil
	}

	// Create output manager for spinner
	om := repository.NewOutputManager(verbose)
	om.StartStage(fmt.Sprintf("Checking source '%s'", source.Name))

	hasChanges, changes, err := sm.UpdateSource(source)
	if err != nil {
		om.StopStage()
		return fmt.Errorf("failed to update source %s: %w", source.Name, err)
	}

	om.StopStage()

	if !hasChanges {
		fmt.Printf("✓ No changes in source '%s'\n", source.Name)
		return nil
	}

	// Print changes
	fmt.Printf("✓ Changes in source '%s':\n", source.Name)
	if len(changes.IdentityChanges) > 0 {
		for _, change := range changes.IdentityChanges {
			fmt.Printf("  - %s\n", change)
		}
	}

	if len(changes.PermissionChanges) > 0 {
		for _, change := range changes.PermissionChanges {
			fmt.Printf("  - %s\n", change)
		}
	}

	if len(changes.RepositoryChanges) > 0 {
		for _, change := range changes.RepositoryChanges {
			fmt.Printf("  - %s\n", change)
		}
	}

	if len(changes.RequiredPermissions) > 0 {
		for _, perm := range changes.RequiredPermissions {
			fmt.Printf("  - New permission required: %s\n", perm)
		}
	}

	// If dry run, stop here
	if dryRun {
		fmt.Printf("✓ Changes would be applied to source '%s'\n", source.Name)
		return nil
	}

	// If force is not set and there are changes that need approval, ask for confirmation
	if !forceUpdate && (len(changes.IdentityChanges) > 0 || len(changes.RequiredPermissions) > 0) {
		approved, err := promptUser("Do you want to apply these changes?")
		if err != nil {
			return fmt.Errorf("failed to get user input: %w", err)
		}
		if !approved {
			fmt.Printf("✓ Changes to source '%s' skipped\n", source.Name)
			return nil
		}
	}

	// Apply changes
	if err := sm.ApplySourceUpdate(source); err != nil {
		return fmt.Errorf("failed to apply changes to source %s: %w", source.Name, err)
	}
	fmt.Printf("✓ Source '%s' updated\n", source.Name)
	return nil
}

// updateTool handles updating a single tool
func updateTool(sm *sources.SourceManager, toolName string) error {
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

	match := matches[0]
	fmt.Printf("Found tool '%s' in source '%s'\n", match.Repo.Name, match.Source.Name)

	if dryRun {
		fmt.Printf("Dry run: would update tool '%s' from source '%s'\n", match.Repo.Name, match.Source.Name)
		fmt.Printf("  URL: %s\n", match.Repo.URL)
		fmt.Printf("  Build command: %s\n", match.Repo.Build)
		fmt.Printf("  Executable: %s\n", match.Repo.Executable)
		return nil
	}

	// Update the tool
	return rm.UpdatePackage(repository.Repository{
		Name:       match.Repo.Name,
		URL:        match.Repo.URL,
		Build:      match.Repo.Build,
		Executable: match.Repo.Executable,
		Load:       match.Repo.Load,
		SkipBuild:  skipBuild,
	})
}

// updateCompletionScript updates the bash completion script
func updateCompletionScript() error {
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

var updateCmd = &cobra.Command{
	Use:   "update [tool]",
	Short: "Update tools from configured sources",
	Long: `Updates the tool sources and index database.

Without arguments, updates all source files and rebuilds the tool index.
With a tool name, updates that specific tool.

Examples:
  getgit update              # Update all sources and rebuild index
  getgit update toolname    # Update specific tool

Flags:
  --force, -f       Skip user approval for changes
  --dry-run, -d     Show changes without applying them
  --index-only, -i  Only rebuild the tool index without fetching updates`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sourcesDir, err := config.GetSourcesDir()
		if err != nil {
			return fmt.Errorf("failed to get sources directory: %w", err)
		}

		sm, err := sources.NewSourceManager()
		if err != nil {
			return fmt.Errorf("failed to initialize source manager: %w", err)
		}

		if err := sm.LoadSources(); err != nil {
			return fmt.Errorf("failed to load sources: %w", err)
		}

		if len(sm.Sources) == 0 {
			return fmt.Errorf("no sources configured. Add source files to %s", sourcesDir)
		}

		// If index-only flag is set, just update the index and return
		if indexOnly {
			fmt.Println("Starting index update...")
			indexManager, err := index.NewManager()
			if err != nil {
				return fmt.Errorf("failed to initialize index manager: %w", err)
			}
			defer indexManager.Close()

			if err := indexManager.UpdateIndex(sm); err != nil {
				return fmt.Errorf("failed to update index: %w", err)
			}
			fmt.Printf("✓ Tool index updated\n")

			// Update completion script
			if err := updateCompletionScript(); err != nil {
				fmt.Printf("Warning: Failed to update completion script: %v\n", err)
			} else {
				fmt.Printf("✓ Shell completion updated\n")
			}

			return nil
		}

		// If no specific tool is specified, update all sources and list them
		if len(args) == 0 {
			fmt.Println("Starting source updates...")
			for i := range sm.Sources {
				if err := updateSource(sm, &sm.Sources[i]); err != nil {
					fmt.Printf("Error updating source '%s': %v\n", sm.Sources[i].Name, err)
				}
			}

			// Update the index after source updates
			if !dryRun {
				fmt.Println("\nUpdating tool index...")
				indexManager, err := index.NewManager()
				if err != nil {
					return fmt.Errorf("failed to initialize index manager: %w", err)
				}
				defer indexManager.Close()

				if err := indexManager.UpdateIndex(sm); err != nil {
					return fmt.Errorf("failed to update index: %w", err)
				}
				fmt.Printf("✓ Tool index updated\n")

				// Update completion script
				if err := updateCompletionScript(); err != nil {
					fmt.Printf("Warning: Failed to update completion script: %v\n", err)
				} else {
					fmt.Printf("✓ Shell completion updated\n")
				}
			}

			return nil
		}

		// Handle specific tool update
		return updateTool(sm, args[0])
	},
}

func init() {
	updateCmd.Flags().BoolVarP(&forceUpdate, "force", "f", false, "Skip user approval for changes")
	updateCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Show changes without applying them")
	updateCmd.Flags().BoolVarP(&indexOnly, "index-only", "i", false, "Just check local files and build the index")
	rootCmd.AddCommand(updateCmd)
}
