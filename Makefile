BINARY_NAME=stress-go
BUILD_DIR=.
GO_FILES=$(shell find . -name "*.go" -type f)

.PHONY: all build clean test fmt help build-linux build-windows build-all

all: build

build: build-linux build-windows

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux -ldflags "-s -w" .

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows.exe -ldflags "-s -w" .

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)-linux
	rm -f $(BUILD_DIR)/$(BINARY_NAME)-windows.exe
