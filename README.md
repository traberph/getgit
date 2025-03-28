# GetGit

Context:  
I totally don't like outdated programs or having to manually download programs on all machines over and over again. So, since I wanted to make a project with Go anyway, here we go...  
(Haha; Go and here we go—did you get it? Nah, anyway...)


GetGit is a command-line tool for managing tools not present in os package managers.  
It allows you to install, update, and remove tools directly from Git repositories.  

## Installation

Requirements: `make` and  `golang`  
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

Usage: `getgit update`

Updates all source files and rebuilds the tool index. This command does not update individual tools - use 'getgit upgrade' for that purpose.

Flags:
- `--force, -f`: Skip user approval for changes
- `--dry-run, -d`: Show changes without applying them
- `--index-only, -i`: Only rebuild the tool index without fetching updates (can be used if source files are locally maintained and updated)

### info
Displays information about available or installed tools.

Usage: `getgit info [tool]`

Without arguments, lists all available tools. With a tool name, shows detailed information about that specific tool.

Flags:
- `--installed, -i`: Show only installed tools
- `--verbose, -v`: Show all fields (build commands, executables, etc.) instead of just name and URL
- `--very-verbose, -V`: Show all fields including load command

### install
Installs a tool from a Git repository.

Usage: `getgit install <tool>`

Clones the repository and sets up the tool according to its configuration. If a tool exists in multiple sources, prompts for selection.
The install command can also be used to change between --release and --edge

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

## Technical Background

### Tool Installation
When installing a tool, GetGit:
1. Clones the repository into the tools directory (parent of getgits own location)
2. Checks out the appropriate version (release tag or latest commit)
3. Runs the build command if specified
4. Creates necessary aliases
5. Sets up load commands if required
6. Creates a `.getgit` file to track installation metadata

### .getgit Files
The `.getgit` file serves two important purposes:
1. Storing metadata about the tool installation in a YAML format within a heredoc section:
   - `sourcefile`: The name of the source file that defined this tool
   - `updates`: The update train ("release" or "edge")
2. Containing any shell commands needed to load the tool environment

A `.getgit` file looks like:
```bash
#!/bin/bash

: <<'EOF'
sourcefile: default
updates: edge
EOF

export SOME_VARIABLE=""
source some_script.sh
```

### Load Commands
GetGit maintains a `.load` file in the root directory that contains:
- Command aliases for installed tools
- Source commands for tools that require environment setup

This file is automatically sourced by your shell when you start a new session, making all installed tools immediately available.

### Name Conflict Resolution
When a tool exists in multiple sources:
1. During installation, you'll be prompted to select which source to use
2. During upgrade, the system uses the source recorded in the tool's `.getgit` file
3. If the `.getgit` file is missing during upgrade, you'll be prompted to select a source again

### Tool Dependencies
GetGit focuses on standalone tools, but if a tool has dependencies:
- System dependencies should be installed separately using your OS package manager
- Dependencies between GetGit-managed tools can be handled by installing the required tools first

