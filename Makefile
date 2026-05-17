# lvm cross-compilation build script
# Builds executables for all platforms into ./dist/

.PHONY: all clean dist windows linux macos test

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

DIST_DIR = dist

all: dist

# Build for current platform
build:
	go build -o lvm .

# Cross-compile for all platforms and copy to dist/
dist: windows linux macos

windows:
	@echo "Building Windows/amd64..."
	GOOS=windows GOARCH=amd64 go build -o $(DIST_DIR)/lvm-windows-amd64.exe .
	GOOS=windows GOARCH=386 go build -o $(DIST_DIR)/lvm-windows-386.exe .

linux:
	@echo "Building Linux/amd64..."
	GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/lvm-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o $(DIST_DIR)/lvm-linux-arm64 .
	GOOS=linux GOARCH=386 go build -o $(DIST_DIR)/lvm-linux-386 .

macos:
	@echo "Building macOS/amd64..."
	GOOS=darwin GOARCH=amd64 go build -o $(DIST_DIR)/lvm-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o $(DIST_DIR)/lvm-darwin-arm64 .

# Build for current platform only (default)
$(DIST_DIR)/%:
	@mkdir -p $(DIST_DIR)
	go build -o $@ .

clean:
	rm -rf $(DIST_DIR) lvm

test:
	go test ./...

help:
	@echo "Available targets:"
	@echo "  all       - Build for all platforms (default)"
	@echo "  windows   - Build Windows executables"
	@echo "  linux     - Build Linux executables"
	@echo "  macos     - Build macOS executables"
	@echo "  build     - Build current platform only"
	@echo "  clean     - Remove dist/ and lvm binary"
	@echo "  test      - Run tests"
	@echo "  help      - Show this help message"
