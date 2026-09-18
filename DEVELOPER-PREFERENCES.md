# Developer Preferences

Project-specific preferences for code style, architecture, and development workflow. Always follow.

## File Structure

One struct — one file. Exception: a struct with many long methods. Splitting criterion: a method + its helper methods used only by it take no less than one screen (25–30 lines). When splitting — use snake_case filename reflecting the struct or main function placed in that file.

## Architecture: Layers

### Handlers

Incoming handlers receive a request, extract all data (headers, cookies, URL parameters; for MCP — data from the request and interaction channel), translate it to internal format (`internal/models`), then invoke a scenario. The scenario is encapsulated behind an interface defining the call contract — never a direct call.

### Scenarios

Each scenario is a separate package. Accepts arguments and returns results only via `models.<Type>` types. Only exceptions: `context.Context` on input and `error` on output. Encapsulates the minimal set of objects needed to interact with subsystems (storage, config, etc.).

### models vs entity

- `internal/entity` — external types defining API/MCP JSON contracts.
- `internal/models` — internal application types used by scenarios and handlers.

## App and Initialization

The main app entity lives in `internal/app`. All startup logic, subsystem initialization, start/stop pipelines are there.

## Packages

Every internal entity gets its own package. The package name reflects the entity's purpose so that an import like `import "github.com/.../internal/config"` clearly indicates the entity (`config.Config`).

## Dependencies and Interfaces

All inter-package dependencies are described as interfaces in `deps.go` to avoid hard coupling and circular dependencies.

```go
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}.
```

Generate mocks with `mockgen` (uber). All interface names are PascalCase so generated mock structs and constructors form valid CamelCase.

## Learning rules

During development, if the owner says:

- **"did X — always do it this way"** — cement the behavior.
- **"I liked X"** — cement the pattern.
- **"wrong to do X that way"** — exclude the behavior.
- **"don't do X that way anymore"** — prohibit it.

Action: extract the essence from "X", determine context (positive → cement, negative → exclude), propose adding the rule to this file. After agreement — add it here.

## Superpowers workflow

The superpowers experience (brainstorming → planning → implementation → artifacts) is documented in `/Users/asmarin/dev/mine/gomorphy/docs/en/superpowers/`. Reproduce here with modifications:

1. Concise style — references to package names and struct/method names (`models.SomeModel`, `usecases/some_use_case/`), no line-number references. Line numbers go stale on refactoring.
2. Artifacts (research, plans, implementation descriptions) go in first-level subdirectories of `docs/`, no deeper than one level. E.g. `docs/plans/`, `docs/research/`, `docs/implementation/`.

## Workflow

Every change goes through these stages:

1. **Grooming/planning/spec** — plan (`docs/plans/`) + spec (`docs/plans/`).
2. **Research** (if needed) — motivation and rationale (`docs/research/`).
3. **Implementation** — code + tests.
4. **Artifact consolidation** — after implementation, plan and spec merge into one file `docs/implementation/<name>.md` describing how it was done. Research (if any) stays separate — so the "why" is preserved.
5. **Changelog** — after the owner confirms the stage is complete:
   - Append to `CHANGELOG.md` in the repo root.
   - Propose a short commit message.
   - Choose: "review and commit myself" / "push to git".
   - If "push to git" — choose: "push now" / "I'll push myself".
