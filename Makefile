APP_NAME := go-image-server

.PHONY: all build build-all

all: build

build:
	@echo "==> go run ./scripts/build.go (host)..."
	@go run ./scripts/build.go

build-all:
	@echo "==> go run ./scripts/build.go --all..."
	@go run ./scripts/build.go --all
