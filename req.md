This is a tool to install small programs which are not contained in os sources or are vastly outdated
It followes the approach to clone the git repo and building the binary local

The programm is configured in ~/.config/getgit
the sources.d folder contains files describing multiple git repos and how to install them

The root folder for this programm is specified in ~/.config/getgit/config.yaml

## the subcommands
### update
fetches a new version of the source files if they specify a origin
reviews the new file and stores it
the user has to approve changes to the name and the origin
the user has to approve changes to the permissons
the user is informed about chnmages in the repos
the --force or -f flag is to skip the user approval
the --dry-run or -d flag shows changes without applying them
`--index-only` or `-i` just check the local files and build the index

### info <tool>
outputs the infos for the specified tool
without a specified tool all tools are printed
`--installed` or `-i` will give infos on installed repos
`--verbose` or `-v` will output all fields otherwise only name and repo url is printed
therfor the root folder is checked for installed tools and getgit files in the folder


### install <tool>
clones a git repo for the provided tool
if the tool is contained in multipe source files the user is prompted which one he prefers
add a .getgit file to the repo if there was a conflict and write the name of the source file in it for later updates
if the repo url is not http assume it is hosted on github
if the repo url starts with http first check the permissions if the host is allowed
after this the repo is cloned
`--release` or `-r` for latest taged release (default)
`--edge` or `-e` for the latest commit (comment in .gitget for updates)
if the user wants to switch between edge and relase just call the install command again with the corresponding flag
if the repo is already installed due to a installed collection remove the comment in the .gitget

the specified build command is executed if it defined
an alias is created in the `.alias` file which is located in the root folder

if a load command is specified the command is put in the .getgit file
source the gitget file in the `.alias` file so the commands are executed
Replace the `{{ gitget.root }}` placeholder in the command with the gitget root defined in the config using go temlating

since the git clone and build command may produce a lot of output hide it per default
show a spinner instead and at which stage the command is (clone/build)
`--verbose` or `-v` still shows the output
`--skip-build` or `-s` skips the build 

### upgrade
check which repos are present on the system
check if there are new relaeses for the repos 
pull the new relese if ther is one
execute the build command specified in the source file for the repo
if the repo is specified multiple times check if a .getgit file exists in the repo
if not ask the user which one to use and create a .getgit file
`--skip-build` or `-s` skips the build 

### uninstall <tool>
removes a tool
removes the alias from the .alias file
remove sourcing of the .gitget file if it exists for this tool


### collect
install all tools from a collection
create a .getgit file to keep track that the tool is not manually installed


## tecnical details
- source files are located at ~/.config/getgit/sources.d
- source files are managed by the sources pkg

- index.db is located in ~/.cache/getgit/index.db
- it is maintained by the index pkg

- install and upgrade commands get the required information from the index.db
- if the db does not exist create it uising the index pkg

- after an update command the index pkg is used to update the index db

- if the build command is not specified no build process is startd
- it is assumed the programm does not neeed a build like bash/python
- still create the alias

.gitget example

```bash
: <<'EOF'
sourcefile: "traberph.yaml"
updates: "release"
collection:
   - 'k8s'
EOF
export NVM=/home/philipp/...
```