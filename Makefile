APP := papertrail
BUILD_DIR := bin

.PHONY: build run scan sca test clean

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP) .

run: build
	./$(BUILD_DIR)/$(APP) ./test

scan: run

sca:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run ./...

test:
	go test ./...

clean:
	rm -f $(BUILD_DIR)/$(APP)
