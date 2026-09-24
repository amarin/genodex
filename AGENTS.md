# AGENTS.md

Project instructions for AI agents (Codex, Claude, LGTM). The authoritative docs for humans and agents live in `/docs` (in Russian).

## About the project
- Single-binary service for a personal genealogy data store. Dev-facing entry point: `go run ./cmd/genodex -p <port> [-web prod|dev]` (`serve` is the default subcommand).
- Layered storage: `internal/mcp`, `internal/httpapi` → `internal/usecases/<scenario>/` → `internal/store` (port) → `internal/store/sqlstore` (adapter) → `internal/storage` (SQLite; колоночная схема, `schema_version` 0). `internal/app` wires everything; data lives on disk (no in-memory store).
- Two public interfaces: MCP (under `/mcp`, Streamable HTTP) for AI assistants, and HTTP API + web SPA (under `/api`, `/static`, `/`) for humans.
- Backend in Go; frontend (Vite + React 18 + antd) is built into `web/dist` and embedded via `//go:embed all:dist`.
- `internal/models` holds the domain types for all service entities (no per-type detail here) and has no JSON tags; `internal/transport` holds the DTOs (JSON tags + converters) that define the API/MCP JSON shapes and is imported only by `internal/httpapi` and `internal/mcp`.

## Developer preferences
- `DEVELOPER-PREFERENCES.md` — project-specific code style, architecture, and development preferences. Always follow.

## One working copy per session
- Every agent session works in its own git worktree on its own branch (e.g. `git worktree add ../genodex-<topic> -b <topic>`), never in the main checkout. Several sessions may run in parallel; in a shared checkout they edit the same files and one can commit another's work.
- The main checkout belongs to the owner: the dev server (GoLand, `-web dev`) runs from it against the real `.data/`. Change it only to merge a finished branch (`git merge --ff-only`); after merging, remove the worktree and branch.
- A worktree server uses its own `-data` directory and port, not the main `.data/`. If it must work with real data, restore a copy from a backup (`./genodex restore`), don't point it at `.data/` directly.
- Stage only files you changed yourself (`git add <paths>`, not `git add -A`); leave unknown untracked or modified files alone and mention them to the owner.

## Key documents and sources of truth
- `/docs` — project documentation (architecture, usage, development).
- `internal/models` — domain model (single source of truth for entity structure). `internal/transport` — DTOs that define JSON shapes exposed via `/api` and MCP tools.
- `internal/definitions` — built-in domain data (e.g. administrative division systems in `russia/`).
- `internal/app/app.go` — wiring: storage → sqlstore → usecases → mcp/httpapi → mux; paths served (`/mcp`, `/api`, `/static`, `/`).

## Files that must not be edited
- `web/dist/**`, `web/node_modules/**` — generated; rebuild with `npm run build`.
- Root `genealogy-mcp` binary — old build artifact, not produced anymore; single binary is built as `genodex` (from `cmd/genodex`).

## Main commands
- `go build ./...` — compile. Requires `web/dist` to exist (`//go:embed all:dist` fails otherwise).
- `go vet ./...` — static analysis.
- `gofmt -w <files>` — formatting; keep the tree gofmt-clean.
- `go run ./cmd/genodex -p <port> [-web prod|dev]` — run the server (`serve` is default). `prod` (default) serves embedded web assets; `dev` reads `web/dist` from disk.
- `go build -o genodex ./cmd/genodex` — build the single binary.
- `./genodex backup [--data DIR] [--to DIR]` / `./genodex restore --from DIR --to DIR [--force]` / `./genodex verify [--data DIR] [--backup DIR]` — backup, restore, and verify the on-disk data store.
- `go test ./...` — run tests (covers `internal/storage` db/backup/restore/verify, `internal/store/sqlstore`, `internal/usecases/...`).
- `cd web && npm install` — install frontend dependencies.
- `cd web && npm run build` — build frontend into `web/dist` (required before `go build` if `web/dist` is missing or stale).
- `cd web && npm run typecheck` — TypeScript check without building.

## When to ask the user
- Changing public contracts: MCP tools (`internal/mcp`) or HTTP endpoints (`internal/httpapi`).
- Changing the storage mechanism (`internal/store`) — the port stays, but swapping the adapter has wide impact on `internal/usecases` and interfaces.
- Changing how web assets are served or the two modes in `web/embed.go`.
- Any change to `internal/storage`'s schema (`schema.go` and friends) that alters an EXISTING table in a way that isn't purely additive (new nullable column, new table) — see "Database schema changes" below. There is currently no real migration runner, so this needs explicit sign-off, not just a code review.

## Database schema changes
- **The database now holds real user data — schema changes must preserve it.** `schemaVersion` (`internal/storage/db.go`) is currently just a marker, not a migration mechanism: `OpenDB` applies the schema via `CREATE TABLE IF NOT EXISTS` (a no-op against an existing table, even if its columns no longer match the Go-side DDL) and `restore` (`internal/storage/restore.go`) flatly rejects a backup whose `schema_version` doesn't match the running binary's — there is no upgrade path from an old on-disk schema to a new one. A backward-incompatible DDL change (renamed/removed/retyped column, a new `NOT NULL` column without a default, a dropped table) will silently desync the code from an existing database rather than erroring loudly, and there is nothing that migrates old rows into the new shape.
- **Before any schema-affecting change**: take a backup of the real data store first (`./genodex backup --data <real data dir> --to <backup dir>`) — do this even in a throwaway/live-verification worktree if it points at real data, and always before touching the primary `.data/` directory in the main checkout.
- **The change itself must preserve existing data**, not just work against a freshly created empty database. If the DDL change is additive (new table, new nullable column with a sane default), `CREATE TABLE IF NOT EXISTS`/`ALTER TABLE ... ADD COLUMN` is enough. If it's not additive (a rename, a type change, a new required column, restructuring a relation), write an explicit, one-off data-preserving migration step (read old rows, transform, write new rows) and test it against a COPY of the real backup, not just a fresh empty DB — the existing test suite's `sqlstore` tests all use fresh temp databases and would not catch a migration bug.
- **Verify with `./genodex verify --data <dir> --backup <backup dir>`** after applying a schema change against real data, before considering the change done.
- This is a process gap, not yet a built migration runner — if schema changes become frequent enough to need one (tracked `schema_version` + ordered migration steps applied on `OpenDB`), that is itself a "when to ask the user" architectural change, not something to build silently as a side effect of an unrelated feature.

## Typical change scenarios and checks
- If you change a MCP tool or `/api` endpoint: update the contract and the frontend types if needed, run `go build ./...`, start the server and verify the route.
- If you change frontend code (`web/src`): run `npm run build` to refresh `web/dist`, then `go build ./...`; the new frontend only reaches the binary through `web/dist`.
- If you change `web/embed.go` or the serving modes: verify both `-web prod` and `-web dev`.
- Base checks before handoff: `gofmt -l .` clean, `go build ./...`, `go vet ./...`, and a quick smoke test of `/api/health`, `/api/admin-divisions`, and `/`.

## PR review rules
- LGTM uses this `AGENTS.md` as project context.
- `Restricted files` are excluded from AI review.
- `High attention areas for review` set the priority, but do not limit the review scope.

### Restricted files
- `web/dist/**`, `web/node_modules/**` — build output.
- Root `genealogy-mcp` binary — generated artifact.

### High attention areas for review
- `internal/store/store.go` — no longer exists; the port is `internal/store/deps.go` and the adapter is `internal/store/sqlstore` (used by all scenarios). Review those together with `internal/app`.
- Public interface contracts: `internal/mcp` tools and `internal/httpapi` endpoints.
- `web/embed.go` — `//go:embed` and web serving modes.