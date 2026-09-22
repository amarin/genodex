package storage

// Схема internal/auth: четыре таблицы, не входящие в generic-систему
// сущностей (models.Type/internal/store — см. docs/data-model/auth.md §3–4).
// owner_id/created_by/used_by — настоящие внешние ключи на owners(id), но
// RESTRICT/SET NULL, не CASCADE: schemaGraph.owner (fkgraph.go) считает
// «дочерней» только CASCADE-связь, поэтому auth-таблицы не становятся частью
// generic-графа Delete*/InUseError, а fkgraph-тесты просто перечисляют их в
// serviceTables (docs/plans/2026-09-22-auth-a2-storage.md, предпосылка).
// Владельца в этом проходе удалить нельзя (нет DeleteOwner) — RESTRICT сейчас
// не сработает ни разу, но выражает верное намерение схемы на будущее.
var authSchemaDDL = []string{
	`CREATE TABLE IF NOT EXISTS owners (
		id            TEXT PRIMARY KEY,
		login         TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at    TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS sessions (
		id                 TEXT PRIMARY KEY,
		owner_id           TEXT NOT NULL REFERENCES owners(id) ON DELETE RESTRICT,
		access_token_hash  TEXT NOT NULL UNIQUE,
		access_expires_at  TEXT NOT NULL,
		refresh_token_hash TEXT NOT NULL UNIQUE,
		refresh_expires_at TEXT NOT NULL,
		created_at         TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS api_tokens (
		id           TEXT PRIMARY KEY,
		owner_id     TEXT NOT NULL REFERENCES owners(id) ON DELETE RESTRICT,
		label        TEXT NOT NULL DEFAULT '',
		token_hash   TEXT NOT NULL UNIQUE,
		created_at   TEXT NOT NULL,
		last_used_at TEXT,
		revoked_at   TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS invites (
		id         TEXT PRIMARY KEY,
		created_by TEXT NOT NULL REFERENCES owners(id) ON DELETE RESTRICT,
		token_hash TEXT NOT NULL UNIQUE,
		expires_at TEXT NOT NULL,
		used_at    TEXT,
		used_by    TEXT REFERENCES owners(id) ON DELETE SET NULL,
		created_at TEXT NOT NULL
	)`,
}
