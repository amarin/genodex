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
| `-trust-proxy` | `false` | Trust `X-Forwarded-Proto` from a reverse proxy for the cookie `Secure` flag (enable only behind a TLS-terminating proxy you control) |

## Authentication

Writes to `/api` and all of `/mcp` require an authenticated owner — anonymous
requests get `401`. On first run, open `/register` in a browser to create the
first owner (no invite needed on a fresh database). Owners can invite others
from `/settings`, which also mints long-lived API tokens for MCP clients:
create one on `/settings`, then send it as `Authorization: Bearer gnx_...`.

## Endpoints

| Path | Purpose |
|------|---------|
| `/mcp` | MCP server, Streamable HTTP transport |
| `/api/health` | Liveness: `{"status":"ok"}` |
| `/api/admin-divisions` | Administrative divisions (JSON array `[{"id","name","type","parent_id"}]`); query: `kind=settlement`, `type=<type>`, `limit` (default 50, max 500), `offset` |
| `/static/` | Built SPA assets |
| `/` | Web SPA (`index.html`, fallback for client-side routes) |

Smoke test:

```bash
curl -s http://localhost:9000/api/health      # {"status":"ok"}
curl -s 'http://localhost:9000/api/admin-divisions?kind=settlement'
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:9000/  # 200
```

## Documentation

Detailed docs (in Russian) live in [`/docs`](docs/index.md):

- [Architecture and project structure](docs/architecture.md)
- [Usage and endpoints](docs/usage.md)
- [Development: commands and build rules](docs/development.md)

AI agents: see [`AGENTS.md`](AGENTS.md).