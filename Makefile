APP_NAME := redis-top
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.version=$(VERSION)
DIST_DIR := dist
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

.PHONY: build test tidy clean release print-version

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/redis-top

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin $(DIST_DIR)

release: clean
	mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output="$(DIST_DIR)/$(APP_NAME)-$(VERSION)-$${os}-$${arch}"; \
		echo "building $$output"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o "$$output" ./cmd/redis-top; \
	done

print-version:
	@echo $(VERSION)
