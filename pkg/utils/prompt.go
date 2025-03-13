package utils

import (
	"fmt"

	"github.com/traberph/getgit/pkg/sources"
)

// PromptSourceSelection prompts the user to select a source from multiple matches
// This is used by both install and upgrade commands
func PromptSourceSelection(matches []sources.RepoMatch) (*sources.RepoMatch, error) {
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
