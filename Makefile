setup:
	make build
	make install

build:
	go build -o bin/getgit
	chmod +x bin/getgit
	rm -rf ~/.cache/getgit;

install:
	mkdir -p ~/.config/getgit
	cp -r config/* ~/.config/getgit/
	echo "root: $$(dirname $$(pwd))" > ~/.config/getgit/config.yaml
	# Create or update the load file
	TOOLS_DIR=$$(dirname $$(pwd)); \
	echo "# This file is managed by getgit. Do not edit manually." > $$TOOLS_DIR/.load; \
	echo "# It contains aliases for installed tools and source commands for other tools." >> $$TOOLS_DIR/.load; \
	echo "alias getgit=\"$$(pwd)/bin/getgit\"" >> $$TOOLS_DIR/.load; \
	bin/getgit completion bash > $$TOOLS_DIR/.bash_completion; \
	grep -q "source $$TOOLS_DIR/.load" ~/.bashrc || echo "source $$TOOLS_DIR/.load" >> ~/.bashrc; \
	grep -q "source $$TOOLS_DIR/.bash_completion" ~/.bashrc || echo "source $$TOOLS_DIR/.bash_completion" >> ~/.bashrc; \
	
	bin/getgit update --index-only

uninstall:
	TOOLS_DIR=$$(dirname $$(pwd)); \
	rm -rf ~/.config/getgit; \
	rm -rf ~/.cache/getgit; \
	sed -i "\#source $$TOOLS_DIR/.load#d" ~/.bashrc; \
	rm -f $$TOOLS_DIR/.load; \
	rm -f $$TOOLS_DIR/.bash_completion

test:
	go test -v -race -cover ./...


.PHONY: setup build install uninstall test clean


	

	
