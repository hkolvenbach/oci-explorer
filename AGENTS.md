# Agent Development Guidelines

This document contains guidelines for AI coding agents working on this codebase.

## Project Mission

OCI Image Explorer aims to be the most comprehensive and technically correct way to inspect OCI images. This means:

- Understanding and adhering to OCI, in-toto, and OpenVEX standards
- Being compatible with ecosystem tools (Trivy, cosign, Notation, BuildKit)
- Being as complete and easy to use as possible

## Supply Chain Artifact Discovery

OCI Explorer discovers supply chain artifacts (signatures, SBOMs, attestations, VEX) through 4 mechanisms:

1. **OCI 1.1 Referrers API** (`VEXViaOCIReferrers`) — standard `GET /v2/<repo>/referrers/<digest>` endpoint
2. **OCI Referrers Tag Schema fallback** — `<alg>-<hex>.referrers` tag for registries that don't support the Referrers API
3. **Cosign tag scheme** (`VEXViaCosignTag`) — `.sig`, `.att` tag suffixes (e.g., `sha256-<hex>.att`)
4. **Docker BuildKit attestation manifests** (`VEXViaBuildKit`) — SBOM/provenance layers embedded in attestation manifests referenced via `vnd.docker.reference.type: attestation-manifest`

All mechanisms run unconditionally on every `InspectImage` call; results are deduplicated and merged.

## VEX Support

- **Format**: OpenVEX (`@context` = `https://openvex.dev/ns/v0.2.0`)
- **Envelopes**: raw OpenVEX JSON, in-toto statement wrapping OpenVEX predicate, DSSE envelope wrapping in-toto statement
- **Pipeline**: discover referrer → fetch attestation manifest or blob → unwrap envelope → parse OpenVEX document → return structured `VEXDocument`

## Testing Philosophy

- Integration tests must exercise the **full pipeline** (discover → fetch → parse), not just discovery
- Use **pinned tags** (e.g., `:3.21.1`, `:0.2.2`) so tests are deterministic
- Use **table-driven tests** with typed enums for discovery methods and image properties
- Never use free-form strings where a typed constant can avoid ambiguity
- Discovery method types are defined in `registry/client_test.go` as `VEXDiscoveryMethod`

## Coding Convention: Prefer Typed Constants Over Raw Strings

When a field has a known set of values (discovery methods, artifact types, VEX statuses), define a named type and constants:

```go
type VEXDiscoveryMethod string

const (
    VEXViaCosignTag    VEXDiscoveryMethod = "cosign-att-tag"
    VEXViaOCIReferrers VEXDiscoveryMethod = "oci-referrers-api"
    VEXViaBuildKit     VEXDiscoveryMethod = "buildkit-attestation"
    VEXNone            VEXDiscoveryMethod = "none"
)
```

Use human-readable const values (not opaque integers) so logs and test output are self-documenting.

## Coding Convention: Pin All Versions for Reproducible Builds

Always pin dependency and tool versions — never use `@latest`. This applies to:

- GitHub Actions (`uses: actions/checkout@<hash> # v4`)
- Go tool installs (`go install tool@v1.2.3`)
- Docker base images (`FROM golang:1.24.13-alpine`)
- Any external dependency in CI/CD workflows

When a commit hash is used (e.g., for GitHub Actions), add a trailing comment with the version tag:

```yaml
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4
- uses: sigstore/cosign-installer@c56c2d3e59e4281cc41dea2217323ba5694b171e # v3.8.0
```

## Key References

- [OCI Distribution Spec 1.1](https://github.com/opencontainers/distribution-spec/blob/main/spec.md) — Referrers API
- [OpenVEX Spec](https://github.com/openvex/spec) — VEX document format
- [in-toto Attestation Framework](https://github.com/in-toto/attestation/tree/main/spec) — statement/predicate envelope
- [Sigstore docs](https://docs.sigstore.dev/) — cosign signing, attestation, verification
- [Docker BuildKit attestation docs](https://docs.docker.com/build/attestations/) — SBOM and provenance in BuildKit

## Pull Request Requirements

When creating a pull request, always ensure the following are updated as part of the PR:

- **Screenshots**: If UI changes were made, capture new screenshots and update `docs/screenshots/`. The README references `welcome.png`, `details.png`, and `graph.png`.
- **README.md**: Update feature lists, API endpoint docs, project structure, and any affected sections.
- **docs/api.md**: Update endpoint documentation, request/response examples, and field tables for any API changes.
- **docs/openapi.yaml**: Update the OpenAPI spec for any new or modified endpoints, schemas, or parameters.
- **BLOG.md**: Internal notes file (git-ignored) for drafting blog-style write-ups. If the change is significant, update or note it there.

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

### DRY / Package Reuse

- Always review existing project dependencies (`go.mod`) for functionality before writing custom code
- Search public packages (e.g. Go standard library, well-maintained open-source libraries) for established solutions
- Prefer using library APIs over reimplementing functionality — even if the library API requires a slight adaptation
- Only write custom code when no suitable library exists or the library would add disproportionate complexity/dependencies
- When adding a new dependency, prefer libraries with zero/minimal transitive dependencies

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
