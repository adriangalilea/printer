.PHONY: build install clean run help

BINARY_NAME=printer
XDG_BIN_HOME ?= $(HOME)/.local/bin
INSTALL_PATH=$(XDG_BIN_HOME)

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the printer binary
	go build -o $(BINARY_NAME)

install: build ## Build and install to ~/.local/bin (XDG_BIN_HOME)
	@mkdir -p $(INSTALL_PATH)
	cp $(BINARY_NAME) $(INSTALL_PATH)/
	@echo "Installed to $(INSTALL_PATH)/$(BINARY_NAME)"
	@echo "Make sure $(INSTALL_PATH) is in your PATH"

clean: ## Remove built binary
	rm -f $(BINARY_NAME)

run: build ## Build and run the printer TUI
	./$(BINARY_NAME)