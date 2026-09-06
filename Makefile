# Usage:
#   make run ARGS=./test   # Set up environment, build, and run Papertrail
#   make build             # Set up environment and build the Papertrail binary
#   make test              # Run Go tests
#   make sca               # Run static code analysis with golangci-lint
#   make clean             # Remove build artifacts and local database
#   make deep-clean        # Clean everything, including the Python virtual environment

APP := papertrail
BUILD_DIR := bin
VENV := .venv

PYTHON := $(VENV)/bin/python
PIP := $(VENV)/bin/pip
REQUIREMENTS := internal/intelligence/requirements.txt
VENV_STAMP := $(VENV)/.installed

.PHONY: setup build run scan sca test clean deep-clean

setup: $(VENV_STAMP)

$(VENV_STAMP): $(REQUIREMENTS)
	python3 -m venv $(VENV)
	$(PIP) install -r $(REQUIREMENTS)
	@touch $(VENV_STAMP)

build: setup
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP) ./cmd/papertrail

run: build
	./$(BUILD_DIR)/$(APP) $(ARGS)

scan: run

sca: setup
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run ./...

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
	rm -f index.db

deep-clean: clean
	rm -rf $(VENV)