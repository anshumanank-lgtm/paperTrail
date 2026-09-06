APP := papertrail
BUILD_DIR := bin
VENV := .venv

.PHONY: venv build run scan sca test clean

venv:
	python3 -m venv $(VENV)
	$(VENV)/bin/pip install -r internal/intelligence/requirements.txt

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP) ./cmd/papertrail

run:build
	./$(BUILD_DIR)/$(APP) $(ARGS)

scan: run

sca:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run ./...

test:
	go test ./...

clean:
	rm -f $(BUILD_DIR)/$(APP)
	rm index.db