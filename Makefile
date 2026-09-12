# Usage:
#   make build             # Set up environment and build the Papertrail binary
#   make proto             # Regenerate Go and gRPC protobuf bindings
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

PROTO_DIR := internal/intelligence/proto
PROTO_FILE := $(PROTO_DIR)/intelligence.proto

.PHONY: setup proto build start test clean deep-clean sca

setup: $(VENV_STAMP)

$(VENV_STAMP): $(REQUIREMENTS)
	python3 -m venv $(VENV)
	$(PIP) install -r $(REQUIREMENTS)
	@touch $(VENV_STAMP)

proto:
	protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)

	$(PYTHON) -m grpc_tools.protoc \
		-I$(PROTO_DIR) \
		--python_out=$(PROTO_DIR) \
		--grpc_python_out=$(PROTO_DIR) \
		$(PROTO_FILE)

build: setup proto
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP) ./cmd/papertrail

sca: setup
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run ./... --disable errcheck

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
	rm -f index.db

deep-clean: clean
	rm -rf $(VENV)

start: build
	set -a && . ./.env && set +a && $(MAKE) build && ./bin/papertrail