# GetGit

GetGit is a command-line tool for managing Git repositories. It allows you to install, update, and remove tools directly from Git repositories, with support for both release and edge versions.

## Installation

GetGit is configured in `~/.config/getgit`. The `sources.d` folder contains files describing Git repositories and their installation instructions.

The root folder for installed tools is specified in `~/.config/getgit/config.yaml`.

## Commands

### update
Updates the tool sources and index database.

Usage: `getgit update [tool]`

Without arguments, updates all source files and rebuilds the tool index. With a tool name, updates that specific tool.

Flags:
- `--force, -f`: Skip user approval for changes
- `--dry-run, -d`: Show changes without applying them
- `--index-only, -i`: Only rebuild the tool index without fetching updates

### info
Displays information about available or installed tools.

Usage: `getgit info [tool]`

Without arguments, lists all available tools. With a tool name, shows detailed information about that specific tool.

Flags:
- `--installed, -i`: Show only installed tools
- `--verbose, -v`: Show all fields (build commands, executables, etc.) instead of just name and URL

### install
Installs a tool from a Git repository.

Usage: `getgit install <tool>`

Clones the repository and sets up the tool according to its configuration. If a tool exists in multiple sources, prompts for selection.

Flags:
- `--release, -r`: Install the latest tagged release (default)
- `--edge, -e`: Install the latest commit from the main branch
- `--verbose, -v`: Show detailed output during installation
- `--skip-build, -s`: Skip the build step

### upgrade
Upgrades installed tools to their latest versions.

Usage: `getgit upgrade [tool]`

Without arguments, upgrades all installed tools. With a tool name, upgrades only that specific tool.

Flags:
- `--skip-build, -s`: Skip the build step after updating
- `--verbose, -v`: Show detailed output during upgrade

### uninstall
Removes an installed tool.

Usage: `getgit uninstall <tool>`

Removes the tool's files, aliases, and configuration.

### completion
Generates shell completion scripts.

Usage: `getgit completion [bash|zsh|fish|powershell]`

Generates completion scripts for the specified shell.

## Configuration

### Source Files
Source files are located in `~/.config/getgit/sources.d` and contain tool definitions including:
- Repository URL
- Build commands
- Executable paths
- Load commands

### Tool Installation
When installing a tool, GetGit:
1. Clones the repository
2. Checks out the appropriate version (release tag or latest commit)
3. Runs the build command if specified
4. Creates necessary aliases
5. Sets up load commands if required

### Aliases
GetGit maintains an `.alias` file in the root directory that contains:
- Command aliases for installed tools
- Source commands for tools that require environment setup

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details. 