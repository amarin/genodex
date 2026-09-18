# AGENTS.md

Project instructions for AI agents (Codex, Claude, LGTM). The authoritative docs for humans and agents live in `/docs` (in Russian).

## About the project
- Single-binary service for a personal genealogy data store. Dev-facing entry point: `go run ./cmd/genealogy-mcp -p <port> [-web prod|dev]`.
- Two public interfaces: MCP (under `/mcp`, Streamable HTTP) for AI assistants, and HTTP API + web SPA (under `/api`, `/static`, `/`) for humans.
- Backend in Go; frontend (Vite + React 18 + antd) is built into `web/dist` and embedded via `//go:embed all:dist`.
- `internal/entity` holds the type definitions for all service entities (no per-type detail here); JSON tags on these models define the API/MCP JSON shapes.

## Developer preferences
- `DEVELOPER-PREFERENCES.md` — project-specific code style, architecture, and development preferences. Always follow.

## Key documents and sources of truth
- `/docs` — project documentation (architecture, usage, development).
- `internal/entity` — domain model, defines JSON shapes exposed via `/api` and MCP tools.
- `internal/definitions` — built-in domain data (e.g. administrative division systems in `russia/`).
- `cmd/genealogy-mcp/main.go` — routable surface: which paths (`/mcp`, `/api`, `/static`, `/`) are served.

## Files that must not be edited
- `web/dist/**`, `web/node_modules/**` — generated; rebuild with `npm run build`.
- Root `genealogy-mcp` binary — build artifact of `go build -o genealogy-mcp ./cmd/genealogy-mcp`.

## Main commands
- `go build ./...` — compile. Requires `web/dist` to exist (`//go:embed all:dist` fails otherwise).
- `go vet ./...` — static analysis.
- `gofmt -w <files>` — formatting; keep the tree gofmt-clean.
- `go run ./cmd/genealogy-mcp -p <port> [-web prod|dev]` — run the server. `prod` (default) serves embedded web assets; `dev` reads `web/dist` from disk.
- `cd web && npm install` — install frontend dependencies.
- `cd web && npm run build` — build frontend into `web/dist` (required before `go build` if `web/dist` is missing or stale).
- `cd web && npm run typecheck` — TypeScript check without building.

## When to ask the user
- Changing public contracts: MCP tools (`internal/mcp`) or HTTP endpoints (`internal/httpapi`).
- Changing the storage mechanism (`internal/store`) — currently in-memory, shared across interfaces.
- Changing how web assets are served or the two modes in `web/embed.go`.

## Typical change scenarios and checks
- If you change a MCP tool or `/api` endpoint: update the contract and the frontend types if needed, run `go build ./...`, start the server and verify the route.
- If you change frontend code (`web/src`): run `npm run build` to refresh `web/dist`, then `go build ./...`; the new frontend only reaches the binary through `web/dist`.
- If you change `web/embed.go` or the serving modes: verify both `-web prod` and `-web dev`.
- Base checks before handoff: `gofmt -l .` clean, `go build ./...`, `go vet ./...`, and a quick smoke test of `/api/health`, `/api/settlements`, and `/`.

## PR review rules
- LGTM uses this `AGENTS.md` as project context.
- `Restricted files` are excluded from AI review.
- `High attention areas for review` set the priority, but do not limit the review scope.

### Restricted files
- `web/dist/**`, `web/node_modules/**` — build output.
- Root `genealogy-mcp` binary — generated artifact.

### High attention areas for review
- `internal/store/store.go` — shared in-memory storage used by both MCP and HTTP API.
- Public interface contracts: `internal/mcp` tools and `internal/httpapi` endpoints.
- `web/embed.go` — `//go:embed` and web serving modes.