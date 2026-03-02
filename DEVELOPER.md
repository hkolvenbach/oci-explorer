# Developer Guide

## Dev Container Setup

The project includes a [Dev Container](https://containers.dev/) configuration that provides a fully configured development environment with Go, Node.js, and Trivy pre-installed.

### Using with VS Code

1. Install the [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
2. Open the project folder in VS Code
3. Press `Ctrl+Shift+P` (or `Cmd+Shift+P` on macOS) and select **Dev Containers: Reopen in Container**
4. Wait for the container to build and the post-create script to finish

The container automatically installs:
- Go 1.25 with toolchain
- Node.js 22
- Trivy (pinned version — see `TRIVY_VERSION` in `.devcontainer/post-create.sh`)
- Go and frontend dependencies

### Using with GitHub Codespaces

Click **Code > Codespaces > New codespace** on the repository page. The devcontainer configuration is picked up automatically.

### Forwarded Ports

| Port | Service |
|------|---------|
| 8080 | Go backend (OCI Explorer) |
| 5173 | Vite dev server (frontend hot reload) |
| 2345 | Delve debugger (remote attach) |

## Running Locally

### Backend only (Go)

```bash
# Build frontend first (required — Go embeds web/dist/*)
cd web && npm ci && npm run build && cd ..

# Run with verbose logging
go run . -v
```

### Frontend hot reload (Vite dev server)

In a separate terminal:

```bash
cd web
npm run dev
```

Vite proxies `/api` and `/docs` requests to `localhost:8080` (configured in `web/vite.config.ts`), so you need the Go backend running simultaneously.

### Docker (full stack with Trivy)

```bash
make run
```

This builds the Docker image (frontend + backend + Trivy) and runs it on port 8080.

## Debugging with VS Code

The project ships with `.vscode/launch.json` configurations for debugging both the Go backend and tests.

### Prerequisites

- The [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.go) for VS Code
- [Delve](https://github.com/go-delve/delve) debugger (installed automatically by the Go extension, or `go install github.com/go-delve/delve/cmd/dlv@latest`)

### Launch Configurations

Open the **Run and Debug** panel (`Ctrl+Shift+D` / `Cmd+Shift+D`) and select a configuration:

| Configuration | Description |
|--------------|-------------|
| **Launch Server** | Build and run the server under the debugger with `-v` (verbose). Breakpoints work in all Go files. |
| **Launch Server (with Trivy)** | Same as above, but ensures `/usr/local/bin` is in PATH so Trivy is discoverable. Use this in the devcontainer. |
| **Attach to Server** | Attach to a running Delve headless server on port 2345 (see below). |
| **Test Current Package** | Debug the test file you have open. Set breakpoints, then press F5. |

### Step-by-step: Debug the Go backend

1. Build the frontend (the Go binary embeds `web/dist/*`):
   ```bash
   cd web && npm ci && npm run build && cd ..
   ```
2. Set a breakpoint in any handler (e.g., `main.go:handleInspect`)
3. Select **Launch Server** from the Run and Debug dropdown
4. Press **F5** to start debugging
5. Open `http://localhost:8080` in your browser and trigger the endpoint
6. VS Code pauses at your breakpoint with full variable inspection

### Step-by-step: Remote attach with Delve

Useful when you want to start the server manually (e.g., with specific flags or environment variables):

1. Build with debug symbols:
   ```bash
   go build -gcflags="all=-N -l" -o build/oci-explorer .
   ```
2. Start the Delve headless server:
   ```bash
   dlv exec ./build/oci-explorer --headless --listen=:2345 --api-version=2 --accept-multiclient -- -v
   ```
3. In VS Code, select **Attach to Server** and press **F5**
4. Set breakpoints and interact with the app normally

### Debugging tests

1. Open a test file (e.g., `scanner/scanner_test.go`)
2. Set breakpoints in the test function you want to debug
3. Select **Test Current Package** and press **F5**
4. Or: click the **Debug Test** code lens above any `func Test*` function

### Debugging the Svelte frontend

The Vite dev server includes source maps. Use Chrome/Firefox DevTools:

1. Start the backend: `go run . -v`
2. Start the frontend: `cd web && npm run dev`
3. Open `http://localhost:5173` in your browser
4. Open DevTools (`F12`) > Sources tab
5. Find your `.svelte` files under `src/` and set breakpoints

## Upgrading Dependencies

Run `make upgrade` to update all dependencies in one shot:

```bash
make upgrade
```

This does three things:

1. **Go modules** — runs `go get -u ./...` then `go mod tidy`
2. **npm packages** — runs `npm update` in `web/`
3. **Trivy** — queries the GitHub API for the latest release and updates the pinned version in `Makefile` and `.devcontainer/post-create.sh`

The Trivy version is pinned in three places (kept in sync by `make upgrade`):

| File | Variable |
|------|----------|
| `Makefile` | `TRIVY_VERSION ?= x.y.z` — passed as `--build-arg` to Docker |
| `.devcontainer/post-create.sh` | `TRIVY_VERSION=x.y.z` — used during devcontainer setup |
| `Dockerfile` | `ARG TRIVY_VERSION=x.y.z` — default, overridden by the Makefile build-arg |

After running `make upgrade`, review the changes with `git diff`, run `make test`, and commit.

## Testing

```bash
# Run all tests (short mode — skips integration tests)
go test -short ./...

# Run all tests including integration (requires network + Trivy)
go test -v ./...

# Run a specific test
go test -v -run TestProcessReport ./scanner/

# Frontend type checking
cd web && npx svelte-check --tsconfig ./tsconfig.json
```

## Project Architecture

```
Browser ──→ Vite (dev) or Go embed (prod) ──→ Svelte 5 SPA
                                                   │
                                                   ▼
                                              /api/* routes
                                                   │
                                    ┌──────────────┼──────────────┐
                                    ▼              ▼              ▼
                              registry/       scanner/       docshandler/
                           (OCI client)    (Trivy subprocess)  (markdown → HTML)
```

- **`main.go`** — HTTP server, route registration, CORS middleware, handlers
- **`registry/`** — OCI registry client (inspect, referrers, SBOM, VEX, matching tags)
- **`scanner/`** — Trivy subprocess wrapper, JSON parsing, severity grouping
- **`docshandler/`** — Serves `/docs/` markdown files as HTML
- **`web/src/`** — Svelte 5 frontend with TypeScript and Tailwind CSS
