# Agent Development Guidelines

This document contains guidelines for AI coding agents working on this codebase.

## Pull Request Requirements

When creating a pull request, always ensure the following are updated as part of the PR:

- **Screenshots**: If UI changes were made, capture new screenshots and update `docs/screenshots/`. The README references `welcome.png`, `details.png`, and `graph.png`.
- **README.md**: Update feature lists, API endpoint docs, project structure, and any affected sections.
- **docs/api.md**: Update endpoint documentation, request/response examples, and field tables for any API changes.
- **docs/openapi.yaml**: Update the OpenAPI spec for any new or modified endpoints, schemas, or parameters.
- **BLOG.md**: If the change is significant enough to warrant a blog-style write-up, update or note it.

Do not consider a PR complete until documentation and screenshots reflect the current state of the code.

## Standards and Conventions

Follow established standards and conventions. When in doubt, look up the relevant specification or industry practice rather than guessing.

### API Design

- REST API endpoints live under `/api/`
- OpenAPI spec is served at `/api/openapi.yaml` (standard location for API consumers)
- The spec source file lives in `docs/openapi.yaml` and is embedded into the binary
- When adding or modifying API endpoints, update both the handler code and `docs/openapi.yaml`
- Response format: `{"success": bool, "data": ..., "error": "..."}` (except raw content endpoints like SBOM download)

### File and Directory Conventions

- Embedded static assets use Go's `//go:embed` directives
- `web/` — frontend files embedded and served at `/`
- `docs/` — documentation files embedded and served at `/docs/`
- API spec served at its standard path `/api/openapi.yaml`, not under `/docs/`
- Keep documentation in sync with code: if you change an endpoint, update the spec

### Commit Messages

- Use [Conventional Commits](https://www.conventionalcommits.org/) format: `type(scope): description`
- Common types: `feat`, `fix`, `docs`, `ci`, `build`, `refactor`, `test`, `chore`
- Scope is optional but encouraged (e.g., `ci(release)`, `build(docker)`)
- Keep the subject line under 72 characters
- Use the body for additional context when needed

### Go Conventions

- Module path: `github.com/hkolvenbach/oci-explorer`
- Follow standard Go project layout conventions
- Use `go vet` and fix all warnings before committing

## Build Verification

**CRITICAL**: After making any code changes, verify that the code compiles successfully.

### Required Build Checks

1. **Standard Build** (current platform):
   ```bash
   make build
   ```

2. **All Platforms** (before releases):
   ```bash
   make build-all
   ```

3. **Tests** (should always pass):
   ```bash
   make test
   ```

### Pre-Commit Checklist

- [ ] `make build` succeeds
- [ ] No unused imports or variables
- [ ] `make test` succeeds
- [ ] `go vet ./...` passes
- [ ] OpenAPI spec updated (if API was changed)

## Common Build Errors

1. **Unused Imports** — `"package" imported and not used` → Remove the import
2. **Unused Variables** — `declared and not used` → Use the variable or replace with `_`
3. **Type Mismatches** — `cannot use X (type Y) as type Z` → Correct the type or add conversion
4. **Missing Dependencies** — `package not found` → Run `go mod tidy`

## Code Quality

### Simplicity

- Write code that is as simple and readable as possible
- Only introduce abstractions (helpers, utilities, shared functions) when at least two occurrences of similar code exist
- Prefer inline, straightforward code over premature generalization

### Imports

- Remove unused imports immediately
- Group imports: stdlib, third-party, local

### Error Handling

- Always handle errors explicitly
- Use `_` for intentionally ignored errors only with a comment
- Return errors from functions, don't just log them

### Testing

- Write tests for new functionality
- Use table-driven tests for multiple cases
- Test both success and error paths

## Directory Guidelines

### `registry/`

- Core OCI registry client logic
- Must work in both server and WASM contexts
- Test with real registries (Docker Hub, GHCR, Quay)

### `web/`

- Frontend HTML/JS (embedded into the Go binary)
- Should work with both server and WASM backends

### `docs/`

- Human-readable documentation (Markdown, API reference)
- `openapi.yaml` source file lives here but is served at `/api/openapi.yaml`
- Embedded into the binary via `//go:embed docs/*`

### `tools/`

- Standalone helper utilities (each has its own `go.mod`)

## Troubleshooting

### Go Installation Issues

If you see `package encoding/pem is not in std`, reinstall Go or run `go clean -cache -modcache`.

### Dependency Issues

Run `go mod tidy` then `go mod download`. Ensure `go.mod` and `go.sum` are committed.
