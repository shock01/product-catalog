.PHONY: tools generate lint build check docker-build


GOPATH := $(shell go env GOPATH)

PLATFORMS := linux/amd64,linux/arm64
REGISTRY  ?= ghcr.io/shock01/product-catalog

# ---------------------------------------------------------------------------
# Tool installation
# ---------------------------------------------------------------------------

tools:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# ---------------------------------------------------------------------------
# Code generation
# ---------------------------------------------------------------------------

generate: tools
	$(GOPATH)/bin/oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml

# ---------------------------------------------------------------------------
# Lint
# ---------------------------------------------------------------------------

lint:
	$(GOPATH)/bin/golangci-lint run ./...

# ---------------------------------------------------------------------------
# Pre-commit check (required when api/openapi.yaml changes)
# ---------------------------------------------------------------------------

check: generate build lint

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

build:
	CGO_ENABLED=0 go build -o bin/api                ./cmd/api
	CGO_ENABLED=0 go build -o bin/indexer            ./cmd/indexer
	CGO_ENABLED=0 go build -o bin/importer           ./cmd/importer
	CGO_ENABLED=0 go build -o bin/price-consumer     ./cmd/price-consumer
	CGO_ENABLED=0 go build -o bin/price-importer     ./cmd/price-importer
	CGO_ENABLED=0 go build -o bin/inventory-consumer ./cmd/inventory-consumer

# ---------------------------------------------------------------------------
# Docker — multi-platform
# ---------------------------------------------------------------------------

docker-build:
	docker buildx build --platform $(PLATFORMS) \
		-f Dockerfile.api            -t $(REGISTRY)/api:$(TAG)            --push .
	docker buildx build --platform $(PLATFORMS) \
		-f Dockerfile.indexer        -t $(REGISTRY)/indexer:$(TAG)        --push .
	docker buildx build --platform $(PLATFORMS) \
		-f Dockerfile.importer       -t $(REGISTRY)/importer:$(TAG)       --push .
	docker buildx build --platform $(PLATFORMS) \
		-f Dockerfile.price-consumer -t $(REGISTRY)/price-consumer:$(TAG) --push .
	docker buildx build --platform $(PLATFORMS) \
		-f Dockerfile.price-importer     -t $(REGISTRY)/price-importer:$(TAG)     --push .
	docker buildx build --platform $(PLATFORMS) \
		-f Dockerfile.inventory-consumer -t $(REGISTRY)/inventory-consumer:$(TAG) --push .
