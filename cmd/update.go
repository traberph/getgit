package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/shell"
	"github.com/traberph/getgit/pkg/sources"
)

var (
	forceUpdate bool
	dryRun      bool
	indexOnly   bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update source files and tool index",
	Long: `Updates the tool source files and rebuilds the tool index database.

This command fetches the latest versions of all configured source files and updates
the tool index database. It does not update individual tools - use 'getgit upgrade'
for that purpose.

Examples:
  getgit update              # Update all source files and rebuild index
  getgit update --dry-run   # Show changes without applying them
  getgit update --index-only # Only rebuild the tool index without fetching updates

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

		if sm.GetSourceCount() == 0 {
			return fmt.Errorf("no sources configured. Add source files to %s", sourcesDir)
		}

		// If index-only flag is set, just update the index and return
		if indexOnly {
			fmt.Println("Starting index update...")
			if err := sm.UpdateIndex(); err != nil {
				return fmt.Errorf("failed to update index: %w", err)
			}
			fmt.Printf("✓ Tool index updated\n")

			// Update completion script
			if err := shell.UpdateCompletionScript(rootCmd); err != nil {
				fmt.Printf("Warning: Failed to update completion script: %v\n", err)
			} else {
				fmt.Printf("✓ Shell completion updated\n")
			}

			return nil
		}

		// Update all sources
		fmt.Println("Starting source updates...")
		for _, source := range sm.GetSources() {
			if err := sm.UpdateSourceWithPrompt(source, forceUpdate, dryRun); err != nil {
				fmt.Printf("Error updating source '%s': %v\n", source.GetName(), err)
			}
		}

		// Update the index after source updates
		if !dryRun {
			fmt.Println("\nUpdating tool index...")
			if err := sm.UpdateIndex(); err != nil {
				return fmt.Errorf("failed to update index: %w", err)
			}
			fmt.Printf("✓ Tool index updated\n")

			// Update completion script
			if err := shell.UpdateCompletionScript(rootCmd); err != nil {
				fmt.Printf("Warning: Failed to update completion script: %v\n", err)
			} else {
				fmt.Printf("✓ Shell completion updated\n")
			}
		}

		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVarP(&forceUpdate, "force", "f", false, "Skip user approval for changes")
	updateCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Show changes without applying them")
	updateCmd.Flags().BoolVarP(&indexOnly, "index-only", "i", false, "Only rebuild the tool index without fetching updates")
	rootCmd.AddCommand(updateCmd)
}
