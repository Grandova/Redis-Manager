.PHONY: all build-frontend build build-linux-amd64 build-linux-arm64 test clean

APP_NAME = redis-manager
VERSION = 1.0.4
BUILD_DIR = bin

all: build-frontend build

build-frontend:
	@echo "Building frontend with Vite..."
	cd frontend && npm install && npm run build

build:
	@echo "Building $(APP_NAME) binary..."
	go build -ldflags "-s -w -X main.Version=$(VERSION)" -o $(APP_NAME) ./cmd/redis-manager

build-linux-amd64: build-frontend
	@echo "Cross-compiling for Linux amd64 (CGO-free)..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./cmd/redis-manager

build-linux-arm64: build-frontend
	@echo "Cross-compiling for Linux arm64 (CGO-free)..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 ./cmd/redis-manager

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR) $(APP_NAME) $(APP_NAME).exe internal/web/dist
