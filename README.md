# Genealogy MCP

Single-binary service for a personal genealogy data store: keeps genealogy entities 
(people, administrative divisions, churches, parishes, archives, events, sources, families, ...) 
and exposes them through two interfaces:

- **MCP** (under `/mcp`, Streamable HTTP) — for AI assistants and MCP clients;
- **HTTP API + web SPA** (under `/api`, `/static`, `/`) — for humans.

Backend in Go; frontend is a Vite + React 18 + antd SPA built into `web/dist` and embedded into the binary via `//go:embed`.

## Quick start

Requires Go 1.26+ and Node.js + npm (for building the frontend).

```bash
# 1. Build the frontend (only needed if web/dist is missing or stale)
cd web && npm install && npm run build && cd ..

# 2. Build and run
go build ./...
go run ./cmd/genodex -p 9000
```

Flags:

| Flag | Default | Purpose |
|------|---------|---------|
| `-p`  | `9000`  | HTTP port |
| `-web`| `prod`  | Web assets mode: `prod` (embedded in binary) or `dev` (read `web/dist` from disk) |

## Endpoints

| Path | Purpose |
|------|---------|
| `/mcp` | MCP server, Streamable HTTP transport |
| `/api/health` | Liveness: `{"status":"ok"}` |
| `/api/settlements` | List of settlements (JSON) |
| `/static/` | Built SPA assets |
| `/` | Web SPA (`index.html`, fallback for client-side routes) |

Smoke test:

```bash
curl -s http://localhost:9000/api/health      # {"status":"ok"}
curl -s http://localhost:9000/api/settlements
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:9000/  # 200
```

## Documentation

Detailed docs (in Russian) live in [`/docs`](docs/index.md):

- [Architecture and project structure](docs/architecture.md)
- [Usage and endpoints](docs/usage.md)
- [Development: commands and build rules](docs/development.md)

AI agents: see [`AGENTS.md`](AGENTS.md).