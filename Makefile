setup:
	make build
	make install

build:
	go build -o bin/getgit
	chmod +x bin/getgit

install:
	mkdir -p ~/.config/getgit
	cp -r config/* ~/.config/getgit/
	echo "root: $$(dirname $$(pwd))" > ~/.config/getgit/config.yaml
	# Create or update the alias file
	TOOLS_DIR=$$(dirname $$(pwd)); \
	echo "# This file is managed by getgit. Do not edit manually." > $$TOOLS_DIR/.alias; \
	echo "# It contains aliases for installed tools." >> $$TOOLS_DIR/.alias; \
	echo "alias getgit=\"$$(pwd)/bin/getgit\"" >> $$TOOLS_DIR/.alias; \
	bin/getgit completion bash > $$TOOLS_DIR/.bash_completion; \
	grep -q "source $$TOOLS_DIR/.alias" ~/.bashrc || echo "source $$TOOLS_DIR/.alias" >> ~/.bashrc; \
	grep -q "source $$TOOLS_DIR/.bash_completion" ~/.bashrc || echo "source $$TOOLS_DIR/.bash_completion" >> ~/.bashrc; \
	
	bin/getgit update --index-only

uninstall:
	TOOLS_DIR=$$(dirname $$(pwd)); \
	rm -rf ~/.config/getgit; \
	rm -rf ~/.cache/getgit; \
	sed -i "\#source $$TOOLS_DIR/.alias#d" ~/.bashrc; \
	rm -f $$TOOLS_DIR/.alias

test:
	go test -v -race -cover ./...

.PHONY: setup build install uninstall test

	

	
