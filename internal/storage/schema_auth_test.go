package storage

import (
	"path/filepath"
	"testing"
)

func TestAuthSchemaTablesExist(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, table := range []string{"owners", "sessions", "api_tokens", "invites"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Errorf("таблица %s не создана: %v", table, err)
		}
	}
}

// TestAuthSchemaLoginUnique: owners.login — UNIQUE (auth.md §3, решение
// финального ревью этапа A).
func TestAuthSchemaLoginUnique(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const insert = `INSERT INTO owners(id, login, password_hash, created_at) VALUES (?, ?, ?, ?)`

	if _, err := db.Exec(insert, "OW-1", "vladelec", "h1", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("первая вставка: %v", err)
	}

	if _, err := db.Exec(insert, "OW-2", "vladelec", "h2", "2026-01-01T00:00:00Z"); err == nil {
		t.Fatal("ожидалась ошибка уникальности login")
	}
}

// TestAuthSchemaOwnerRestrictsDelete: owner_id REFERENCES ... RESTRICT —
// нельзя удалить владельца, пока есть его сессия.
func TestAuthSchemaOwnerRestrictsDelete(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(
		`INSERT INTO owners(id, login, password_hash, created_at) VALUES ('OW-1','x','h','2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("owner: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO sessions(id, owner_id, access_token_hash, access_expires_at,
		 refresh_token_hash, refresh_expires_at, created_at)
		 VALUES ('SS-1','OW-1','ah','2026-01-01T00:00:00Z','rh','2026-02-01T00:00:00Z','2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("session: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM owners WHERE id = 'OW-1'`); err == nil {
		t.Fatal("ожидалась ошибка RESTRICT при удалении владельца с сессией")
	}
}

// TestAuthSchemaIdempotentOpen: повторное открытие той же базы не падает
// (CREATE TABLE IF NOT EXISTS) и не плодит индексы.
func TestAuthSchemaIdempotentOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	db1, err := OpenDB(path)
	if err != nil {
		t.Fatalf("первое открытие: %v", err)
	}
	_ = db1.Close()

	db2, err := OpenDB(path)
	if err != nil {
		t.Fatalf("второе открытие: %v", err)
	}
	t.Cleanup(func() { _ = db2.Close() })
}
