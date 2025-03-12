package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// Common flags used across commands
var verbose bool

var rootCmd = &cobra.Command{
	Use:   "getgit",
	Short: "A Git package manager",
	Long: `

 ██████╗ ███████╗████████╗ ██████╗ ██╗████████╗
██╔════╝ ██╔════╝╚══██╔══╝██╔════╝ ██║╚══██╔══╝
██║  ███╗█████╗     ██║   ██║  ███╗██║   ██║   
██║   ██║██╔══╝     ██║   ██║   ██║██║   ██║   
╚██████╔╝███████╗   ██║   ╚██████╔╝██║   ██║   
 ╚═════╝ ╚══════╝   ╚═╝    ╚═════╝ ╚═╝   ╚═╝   


GetGit is a command-line tool for managing Git packages. It allows you to install, 
update, and remove tools directly from Git repositories, with support for both 
release and edge versions.

Configuration is stored in ~/.config/getgit with tool sources in the sources.d/ directory.
The root folder for installed tools is specified in ~/.config/getgit/config.yaml.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
