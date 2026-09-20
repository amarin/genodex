package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

// entityRow — строка чтения сущности.
type entityRow struct {
	ID   string
	Data []byte
}

// DB оборачивает одно SQLite-соединение. Текущая схема — переходная,
// модель-агностичная (JSON в колонке data, поисковый индекс — в search);
// нормализация — по плану docs/todo.md.
type DB struct {
	d *sql.DB
}

// OpenDB открывает БД, включает WAL, foreign keys и создаёт схему.
func OpenDB(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := d.Exec(p); err != nil {
			d.Close()
			return nil, fmt.Errorf("open: %w", err)
		}
	}

	ddl := []string{
		`CREATE TABLE IF NOT EXISTS meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS entity (
			entity_type TEXT NOT NULL,
			id          TEXT NOT NULL,
			data        TEXT NOT NULL,
			search      TEXT NOT NULL DEFAULT '',
			updated_at  TEXT NOT NULL,
			PRIMARY KEY (entity_type, id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entity_search
			ON entity (entity_type, search)`,
	}
	for _, q := range ddl {
		if _, err := d.Exec(q); err != nil {
			d.Close()
			return nil, fmt.Errorf("schema: %w", err)
		}
	}
	if _, err := d.Exec(
		`INSERT OR IGNORE INTO meta(key, value) VALUES ('schema_version', ?)`, schemaVersion,
	); err != nil {
		d.Close()
		return nil, err
	}
	return &DB{d: d}, nil
}

// Upsert сохраняет сущность (INSERT OR REPLACE). search — уже нормализованный
// конкатенат поисковых строк сущности.
func (d *DB) Upsert(entityType, id string, data, search []byte) error {
	_, err := d.d.Exec(
		`INSERT OR REPLACE INTO entity(entity_type, id, data, search, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		entityType, id, string(data), string(search), time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (d *DB) Delete(entityType, id string) error {
	_, err := d.d.Exec(`DELETE FROM entity WHERE entity_type = ? AND id = ?`, entityType, id)
	return err
}

func (d *DB) Get(entityType, id string) ([]byte, bool, error) {
	var data string
	err := d.d.QueryRow(
		`SELECT data FROM entity WHERE entity_type = ? AND id = ?`, entityType, id,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(data), true, nil
}

// List возвращает все сущности типа в недетерминированном порядке.
func (d *DB) List(entityType string) ([]entityRow, error) {
	rows, err := d.d.Query(
		`SELECT id, data FROM entity WHERE entity_type = ?`, entityType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entityRow
	for rows.Next() {
		var r entityRow
		var data string
		if err := rows.Scan(&r.ID, &data); err != nil {
			return nil, err
		}
		r.Data = []byte(data)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Search возвращает id сущностей, чей search-индекс содержит query (полное
// или частичное совпадение). query обязан быть уже нормализованным.
func (d *DB) Search(entityType, query string) ([]string, error) {
	rows, err := d.d.Query(
		`SELECT id FROM entity
		 WHERE entity_type = ? AND search LIKE '%' || ? || '%'
		 ORDER BY id`,
		entityType, query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (d *DB) Count(entityType string) (int, error) {
	var n int
	err := d.d.QueryRow(
		`SELECT COUNT(*) FROM entity WHERE entity_type = ?`, entityType,
	).Scan(&n)
	return n, err
}

// IntegrityCheck выполняет PRAGMA integrity_check (максимум 4 строки ошибок).
func (d *DB) IntegrityCheck() error {
	rows, err := d.d.Query("PRAGMA integrity_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		if line != "ok" {
			return fmt.Errorf("integrity_check: %s", line)
		}
	}
	return rows.Err()
}

// VacuumInto создаёт консистентный снапшот БД без остановки (SQLite сам
// согласует состояние с WAL на момент выполнения).
func (d *DB) VacuumInto(path string) error {
	esc := strings.ReplaceAll(path, "'", "''")
	_, err := d.d.Exec(fmt.Sprintf("VACUUM INTO '%s'", esc))
	return err
}

func (d *DB) Close() error { return d.d.Close() }
