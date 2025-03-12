# GetGit

Context:  
I totally don't like outdated programs or having to manually download programs on all machines over and over again. So, since I wanted to make a project with Go anyway, here we go...  
(Haha; Go and here we go—did you get it? Nah, anyway...)


GetGit is a command-line tool for managing tools not present in os package managers.  
It allows you to install, update, and remove tools directly from Git repositories.  

## Installation

Reqirements: `make` and  `golang`  
Installation: 
```
mkdir tools & cd tools
wget -qO- https://raw.githubusercontent.com/traberph/getgit/refs/heads/main/install.sh | bash
```
This will create a tools folder and installs getgit into it.  
Getgit itself will also install its tools into this folder.  
Make sure to source your `.bashrc` again since getgit uses aliases

## Commands

### update
Updates the tool sources and index database.

Usage: `getgit update [tool]`

Without arguments, updates all source files and rebuilds the tool index. With a tool name, updates that specific tool.

Flags:
- `--force, -f`: Skip user approval for changes
- `--dry-run, -d`: Show changes without applying them
- `--index-only, -i`: Only rebuild the tool index without fetching updates (can be used if sourcefiles are localy maintained and updated)

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


## Configuration

### Source Files
Source files are located in `~/.config/getgit/sources.d` and contain tool definitions including:
- Repository URL
- Build commands
- Executable paths
- Load commands
For more details check out the default source files.

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

