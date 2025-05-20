# Makefile for billing-job project

# Go related settings
GO := go
GOFMT := $(GO) fmt
GOTEST := $(GO) test
GOCLEAN := $(GO) clean
GO_BUILD := $(GO) build
GO_GET := $(GO) get

# Go Binary name
BINARY_NAME := billing-job

# Directories
SRC_DIR := .
BUILD_DIR := ./bin

# Default values for GOOS and GOARCH
GOOS ?= linux
GOARCH ?= amd64

# Kubernetes client-go package
K8S_CLIENT := k8s.io/client-go/kubernetes

# Default target
.PHONY: all
all: fmt test build

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) $(SRC_DIR)

# Get dependencies
.PHONY: get
get:
	$(GO_GET) ./...

# Run tests
.PHONY: test
test:
	$(GOTEST) -v ./...

# Build the binary for specified GOOS and GOARCH
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH) .

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Run the application
.PHONY: run
run: build
	$(BUILD_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)

# Build a Docker image
.PHONY: docker-build
docker-build:
	docker build -t billing-job .

# Push the Docker image
.PHONY: docker-push
docker-push:
	docker push billing-job

# Deploy to Kubernetes (example: kubectl apply -f deployment.yaml)
.PHONY: deploy
deploy:
	kubectl apply -f k8s/deployment.yaml