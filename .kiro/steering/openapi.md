# OpenAPI Specification

## Location
The OpenAPI spec lives at:

```
api/openapi.yaml
```

This follows the [golang-standards/project-layout](https://github.com/golang-standards/project-layout) convention where the `api/` directory holds:
> OpenAPI/Swagger specs, JSON schema files, protocol definition files.

The `api/` directory is the correct and conventional home for protocol contracts in a standard Go project. Do not place the spec under `docs/`, `internal/`, or the repo root.

## Spec Standard
- OpenAPI **3.1**
- Single file: `api/openapi.yaml`
- External `$ref` files (e.g. shared schemas) may live under `api/schemas/` if the spec grows large, but the entrypoint is always `api/openapi.yaml`

## Code Generation
- Tool: [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen) v2
- Generated output goes to `internal/gen/api.gen.go` — never edited by hand
- Regenerate with: `make generate`
- The `Makefile` target runs: `oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml`
- Generation config (`api/oapi-codegen.yaml`) emits strict chi server stubs + models into package `gen`
- A `//go:generate` directive in `internal/gen/generate.go` means `go generate ./...` also works

## Workflow Rule
The spec is the **single source of truth**. The correct order of changes is always:

```
1. Update api/openapi.yaml
2. make generate        — regenerates internal/gen/api.gen.go
3. make build          — verify the codebase compiles against the new generated types
4. Implement/update the handler against the generated interface
5. make build          — verify again after handler changes
6. Commit api/openapi.yaml, internal/gen/api.gen.go, and handler changes together
```

Never write a handler first and retrofit the spec. Never modify generated files.

**Before every commit that touches `api/openapi.yaml`:**
- `make generate` must have been run and `internal/gen/api.gen.go` must be up to date
- `make build` must pass with zero errors
- Both files must be staged in the same commit — a spec change without the regenerated file, or a regenerated file without a passing build, must not be committed

## Versioning
- Version is the third path segment: `/{domain}/{context}/{version}/{id}` — see `.kiro/steering/url-structure.md`
- Breaking changes require a new version segment (`v2`) on the affected context only; other contexts are unaffected
- Non-breaking additions (new optional fields, new endpoints) are made in-place on the current version
