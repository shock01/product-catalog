# Dockerfile Standards

## Multi-Stage Builds
All Dockerfiles must use multi-stage builds with exactly two stages:

1. `builder` — compiles the Go binary
2. `runtime` — minimal final image

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /bin/service ./cmd/<service>

FROM gcr.io/distroless/static-nonroot:nonroot AS runtime
COPY --from=builder /bin/service /service
ENTRYPOINT ["/service"]
```

## Multi-Platform Targets
- Always declare `ARG TARGETOS` and `ARG TARGETARCH` in the builder stage
- Pass them to `go build` via `GOOS`/`GOARCH` — never hard-code the target platform
- Build and push with `docker buildx build --platform linux/amd64,linux/arm64`
- CI must produce a multi-arch manifest (not separate tags per arch)

## Base Images
- Builder: `golang:1.24-alpine` — use `--platform=$BUILDPLATFORM` so the build toolchain always runs on the host platform
- Runtime: `gcr.io/distroless/static-nonroot` — no shell, no package manager, runs as non-root by default
- Pin images by digest in CI; use tag-only in local dev for convenience

## One Dockerfile Per Binary
Each binary under `cmd/` gets its own Dockerfile at the repo root:

```
Dockerfile.api
Dockerfile.indexer
Dockerfile.importer
```

## Build Constraints
- `CGO_ENABLED=0` — required for distroless; ensures a fully static binary
- `-trimpath` — removes local file paths from the binary
- `-ldflags="-s -w"` — strips debug info and symbol table

## Layer Caching
- Copy `go.mod` and `go.sum` and run `go mod download` before copying source — preserves the module cache layer across source-only changes
