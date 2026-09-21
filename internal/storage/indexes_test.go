package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// pragmaRows выполняет запрос и возвращает строки как срезы значений.
func pragmaRows(t *testing.T, db *DB, query string, args ...any) [][]any {
	t.Helper()

	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}

	var out [][]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		out = append(out, vals)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return out
}

func allTables(t *testing.T, db *DB) []string {
	t.Helper()

	var names []string
	for _, r := range pragmaRows(t, db,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`) {
		names = append(names, fmt.Sprint(r[0]))
	}

	return names
}

// fkColumns возвращает FK-колонки таблицы классическим PRAGMA (независимо от
// табличных функций, которыми пользуется реализация).
func fkColumns(t *testing.T, db *DB, table string) []string {
	t.Helper()

	var cols []string
	for _, r := range pragmaRows(t, db, fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteIdent(table))) {
		cols = append(cols, fmt.Sprint(r[3])) // колонки: id, seq, table, from, to, …
	}

	return cols
}

// leadingIndexes возвращает для каждой колонки таблицы имена индексов, у
// которых она первая (включая автоиндексы первичных и уникальных ключей).
func leadingIndexes(t *testing.T, db *DB, table string) map[string][]string {
	t.Helper()

	out := map[string][]string{}
	for _, il := range pragmaRows(t, db, fmt.Sprintf("PRAGMA index_list(%s)", quoteIdent(table))) {
		name := fmt.Sprint(il[1]) // колонки: seq, name, unique, origin, partial
		info := pragmaRows(t, db, fmt.Sprintf("PRAGMA index_info(%s)", quoteIdent(name)))
		if len(info) == 0 {
			continue
		}
		col := fmt.Sprint(info[0][2]) // колонки: seqno, cid, name; первая строка — ведущая колонка
		out[col] = append(out[col], name)
	}

	return out
}

func TestForeignKeysAreIndexed(t *testing.T) {
	db := openTestDB(t)

	checked := 0
	for _, table := range allTables(t, db) {
		leading := leadingIndexes(t, db, table)
		for _, col := range fkColumns(t, db, table) {
			checked++
			if len(leading[col]) == 0 {
				t.Errorf("%s.%s: у FK-колонки нет ведущего индекса", table, col)
			}
		}
	}

	// Защита от пустой проверки: в схеме больше сотни FK-колонок.
	if checked < 100 {
		t.Errorf("проверено FK-колонок: %d, ожидается не меньше 100", checked)
	}
}

func TestForeignKeyIndexesNotDuplicated(t *testing.T) {
	db := openTestDB(t)

	for _, table := range allTables(t, db) {
		leading := leadingIndexes(t, db, table)
		for _, col := range fkColumns(t, db, table) {
			if n := len(leading[col]); n > 1 {
				t.Errorf("%s.%s: %d ведущих индексов (%v), ожидается один", table, col, n, leading[col])
			}
		}
	}
}

func TestForeignKeyIndexNaming(t *testing.T) {
	db := openTestDB(t)

	for _, want := range []struct{ table, name string }{
		{"person_names", "idx_person_names_person_id"},
		{"person_names", "idx_person_names_surname_id"},
		{"source_links", "idx_source_links_citation_id"},
		// явный индекс, созданный DDL схемы, сохраняется
		{"source_links", "idx_source_links_target"},
	} {
		var n int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND name = ?`,
			want.table, want.name,
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("индекс %s на %s: найдено %d, ожидается 1", want.name, want.table, n)
		}
	}
}

func indexNames(t *testing.T, db *DB) []string {
	t.Helper()

	var names []string
	for _, r := range pragmaRows(t, db, `SELECT name FROM sqlite_master WHERE type = 'index' ORDER BY name`) {
		names = append(names, fmt.Sprint(r[0]))
	}
	sort.Strings(names)

	return names
}

func TestForeignKeyIndexesIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "genealogy.db")

	first, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	before := indexNames(t, first)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := OpenDB(path)
	if err != nil {
		t.Fatalf("повторное открытие: %v", err)
	}
	defer second.Close()

	after := indexNames(t, second)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("набор индексов изменился при повторном открытии:\nдо    %v\nпосле %v", before, after)
	}
	if len(before) < 100 {
		t.Errorf("индексов %d, ожидается не меньше 100 (FK-индексы + явные)", len(before))
	}
}

// Колонки, у которых уже есть ведущий индекс (первичный ключ, явный индекс,
// UNIQUE), новыми индексами не дублируются; вторая колонка составного индекса
// ведущей не считается. В боевой схеме такого случая нет, поэтому ветка
// проверяется на игрушечной БД.
func TestCreateFKIndexesSkipsCoveredColumns(t *testing.T) {
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.SetMaxOpenConns(1)

	for _, q := range []string{
		`CREATE TABLE parent (id TEXT PRIMARY KEY)`,
		// FK-колонка — первичный ключ: автоиндекс уже есть
		`CREATE TABLE child_pk (owner TEXT PRIMARY KEY REFERENCES parent(id))`,
		// FK-колонка без индекса
		`CREATE TABLE child_bare (owner TEXT REFERENCES parent(id), n INTEGER)`,
		// явный индекс, ведущий FK-колонкой
		`CREATE TABLE child_explicit (owner TEXT REFERENCES parent(id))`,
		`CREATE INDEX my_owner_idx ON child_explicit (owner)`,
		// UNIQUE, ведущий FK-колонкой
		`CREATE TABLE child_unique (x TEXT, y TEXT REFERENCES parent(id), UNIQUE (y, x))`,
		// FK-колонка вторая в UNIQUE: ведущего индекса нет
		`CREATE TABLE child_second (a TEXT, b TEXT REFERENCES parent(id), UNIQUE (a, b))`,
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	for i := 0; i < 2; i++ { // второй проход — идемпотентность
		if err := createFKIndexes(d); err != nil {
			t.Fatalf("проход %d: %v", i+1, err)
		}
	}

	want := map[string]string{
		"child_pk":       "sqlite_autoindex_child_pk_1",
		"child_bare":     "idx_child_bare_owner",
		"child_explicit": "my_owner_idx",
		"child_unique":   "sqlite_autoindex_child_unique_1",
		"child_second":   "idx_child_second_b,sqlite_autoindex_child_second_1",
	}
	for table, wantNames := range want {
		got, err := queryStrings(d,
			`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = ? ORDER BY name`, table)
		if err != nil {
			t.Fatal(err)
		}
		if joined := strings.Join(got, ","); joined != wantNames {
			t.Errorf("%s: индексы %q, ожидается %q", table, joined, wantNames)
		}
	}
}

// Выборки по владельцу и по ссылке на value-таблицу идут по индексу.
func TestLookupsUseIndexes(t *testing.T) {
	db := openTestDB(t)

	tests := []struct {
		query string
		index string
	}{
		{"SELECT id FROM person_names WHERE person_id = ?", "idx_person_names_person_id"},
		{"SELECT id FROM person_names WHERE surname_id = ?", "idx_person_names_surname_id"},
		{"SELECT id FROM source_links WHERE citation_id = ?", "idx_source_links_citation_id"},
		{"DELETE FROM person_names WHERE person_id = ?", "idx_person_names_person_id"},
	}
	for _, tt := range tests {
		var details []string
		for _, r := range pragmaRows(t, db, "EXPLAIN QUERY PLAN "+tt.query, "x") {
			details = append(details, fmt.Sprint(r[3])) // колонки: id, parent, notused, detail
		}
		plan := strings.Join(details, "; ")
		if !strings.Contains(plan, tt.index) {
			t.Errorf("%s\nплан: %s\nожидается индекс %s", tt.query, plan, tt.index)
		}
	}
}
