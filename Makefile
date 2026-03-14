APP_NAME := go-image-server
OUTPUT_DIR := bin

GOOS_LIST := windows linux darwin
GOARCH_LIST := amd64 arm64

.PHONY: all clean build build-all \
	build-windows build-linux build-darwin

all: build

clean:
	@echo "==> Cleaning $(OUTPUT_DIR)..."
	@if [ -d "$(OUTPUT_DIR)" ]; then rm -rf "$(OUTPUT_DIR)"; fi

build:
	@echo "==> go build (host)..."
	@mkdir -p "$(OUTPUT_DIR)"
	@CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$(OUTPUT_DIR)/$(APP_NAME)" .

build-all: clean build-windows build-linux build-darwin

build-windows:
	@echo "==> Building for windows..."
	@mkdir -p "$(OUTPUT_DIR)"
	@for arch in $(GOARCH_LIST); do \
	  echo "  - windows/$$arch"; \
	  CGO_ENABLED=0 GOOS=windows GOARCH=$$arch go build -trimpath -ldflags "-s -w" \
	    -o "$(OUTPUT_DIR)/$(APP_NAME)-windows-$$arch.exe" . || exit 1; \
	done

build-linux:
	@echo "==> Building for linux..."
	@mkdir -p "$(OUTPUT_DIR)"
	@for arch in $(GOARCH_LIST); do \
	  echo "  - linux/$$arch"; \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -ldflags "-s -w" \
	    -o "$(OUTPUT_DIR)/$(APP_NAME)-linux-$$arch" . || exit 1; \
	done

build-darwin:
	@echo "==> Building for darwin..."
	@mkdir -p "$(OUTPUT_DIR)"
	@for arch in $(GOARCH_LIST); do \
	  echo "  - darwin/$$arch"; \
	  CGO_ENABLED=0 GOOS=darwin GOARCH=$$arch go build -trimpath -ldflags "-s -w" \
	    -o "$(OUTPUT_DIR)/$(APP_NAME)-darwin-$$arch" . || exit 1; \
	done

