package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// schemaVersion — версия схемы. 0 — первая колоночная схема; переход с
// плоской таблицы entity не обратносовместим (старые данные не мигрируются).
const schemaVersion = 0

// DB оборачивает одно SQLite-соединение. Схема — колоночная (сущность =
// таблица, коллекция = связная таблица), см. docs/data-model/normalization-s1s2.md.
// Маппинг сущностей на строки — забота internal/store/sqlstore; здесь только
// соединение, схема и обслуживание.
type DB struct {
	d *sql.DB
}

// OpenDB открывает БД, включает WAL и foreign keys и создаёт схему целиком.
func OpenDB(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// одно соединение: PRAGMA-настройки (в т.ч. foreign_keys) действуют
	// на каждое соединение отдельно, единственное соединение гарантирует,
	// что включённые здесь прагмы действуют для всех запросов
	d.SetMaxOpenConns(1)

	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := d.Exec(p); err != nil {
			d.Close()
			return nil, fmt.Errorf("open: %w", err)
		}
	}

	ddl := append(
		[]string{`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`},
		schemaDDL...,
	)
	for _, q := range ddl {
		if _, err := d.Exec(q); err != nil {
			d.Close()
			return nil, fmt.Errorf("schema: %w", err)
		}
	}
	if err := createFKIndexes(d); err != nil {
		d.Close()
		return nil, fmt.Errorf("indexes: %w", err)
	}
	if _, err := d.Exec(
		`INSERT OR IGNORE INTO meta(key, value) VALUES ('schema_version', ?)`, schemaVersion,
	); err != nil {
		d.Close()
		return nil, err
	}
	return &DB{d: d}, nil
}

// Exec выполняет запрос без результата.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) { return d.d.Exec(query, args...) }

// Query выполняет запрос с множеством строк.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) { return d.d.Query(query, args...) }

// QueryRow выполняет запрос с одной строкой.
func (d *DB) QueryRow(query string, args ...any) *sql.Row { return d.d.QueryRow(query, args...) }

// ExecContext — Exec с контекстом: отмена доходит до драйвера.
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.d.ExecContext(ctx, query, args...)
}

// QueryContext — Query с контекстом.
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.d.QueryContext(ctx, query, args...)
}

// QueryRowContext — QueryRow с контекстом.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.d.QueryRowContext(ctx, query, args...)
}

// TxContext выполняет fn в транзакции, привязанной к ctx; при ошибке — откат.
func (d *DB) TxContext(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// одно соединение: незакрытая транзакция заблокировала бы все следующие
	// запросы, поэтому откат — на любом выходе из fn (ошибка, паника, Goexit);
	// после Commit он возвращает sql.ErrTxDone и ничего не делает
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return ctxCause(ctx, err)
	}
	return ctxCause(ctx, tx.Commit())
}

// ctxCause возвращает ошибку контекста вместо sql.ErrTxDone: при отмене ctx
// database/sql откатывает транзакцию сам, и дальнейшие вызовы tx теряют причину.
func ctxCause(ctx context.Context, err error) error {
	if err != nil && errors.Is(err, sql.ErrTxDone) && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// Tx выполняет fn в транзакции; при ошибке — откат.
func (d *DB) Tx(fn func(tx *sql.Tx) error) error {
	tx, err := d.d.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Count считает строки таблицы из реестра (валидация имени защищает от
// SQL-инъекции: имя подставляется в текст запроса, а не параметром).
func (d *DB) Count(table string) (int, error) {
	if !tableNames[table] {
		return 0, fmt.Errorf("unknown table %q", table)
	}
	var n int
	err := d.d.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
	return n, err
}

// IntegrityCheck выполняет PRAGMA integrity_check.
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

// Close закрывает соединение.
func (d *DB) Close() error { return d.d.Close() }
