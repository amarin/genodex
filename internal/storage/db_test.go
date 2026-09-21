package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenDB(filepath.Join(t.TempDir(), "genealogy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// wantTables — полный ожидаемый состав схемы (сущности + связные таблицы +
// общие таблицы value-типов). Фиксирует колоночную схему из
// docs/data-model/normalization-s1s2.md §5.
var wantTables = []string{
	// служебные и общие
	"meta", "dates", "text_refs", "anchors", "source_links", "search_index",
	// персоны
	"persons", "person_names", "person_estates", "person_titles",
	"person_nicknames", "person_notes",
	// связи и семьи
	"relations", "relation_notes",
	"families", "family_members", "family_notes",
	// словари
	"surnames", "surname_variants", "surname_items", "surname_notes",
	"given_names", "given_name_variants", "given_name_items", "given_name_notes",
	"patronymics", "patronymic_variants", "patronymic_items", "patronymic_notes",
	"estates", "estate_variants", "estate_items", "estate_notes",
	"titles", "title_variants", "title_items", "title_notes",
	// места
	"administrative_divisions", "ad_items", "ad_variants", "ad_renames",
	"ad_successors", "ad_notes",
	"churches", "church_settlements", "church_variants", "church_notes",
	"parishes", "parish_settlements", "parish_notes",
	// факты
	"events", "event_participants", "event_notes",
	"residences",
	// доказательства
	"sources", "source_notes",
	"citations",
	"notes",
	"repositories", "repository_urls", "repository_notes",
	"archives", "archive_notes",
	"archive_nodes", "node_settlements", "node_notes",
	"archive_documents", "doc_settlements", "doc_notes",
	"attachments",
}

func TestSchemaCreated(t *testing.T) {
	db := openTestDB(t)
	for _, table := range wantTables {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatalf("table %s: %v", table, err)
		}
	}

	// в схеме не должно быть лишних таблиц (в частности, старой entity)
	rows, err := db.Query(
		`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := append([]string(nil), wantTables...)
	sort.Strings(want)
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("таблиц в схеме %d, ожидается %d:\ngot  %v\nwant %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("таблица #%d = %q, ожидается %q", i, got[i], want[i])
		}
	}
}

func TestSchemaVersionIsZero(t *testing.T) {
	db := openTestDB(t)
	var v string
	if err := db.QueryRow("SELECT value FROM meta WHERE key='schema_version'").Scan(&v); err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	if v != "0" {
		t.Fatalf("schema_version = %q, want 0", v)
	}
}

func TestCountRejectsUnknownTable(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Count("persons"); err != nil {
		t.Fatalf("Count(persons): %v", err)
	}
	for _, bad := range []string{"person", "sqlite_master", "persons; DROP TABLE persons", ""} {
		if _, err := db.Count(bad); err == nil {
			t.Fatalf("Count(%q) должен быть отклонён реестром", bad)
		}
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	db := openTestDB(t)
	var on int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&on); err != nil {
		t.Fatal(err)
	}
	if on != 1 {
		t.Fatalf("PRAGMA foreign_keys = %d, want 1", on)
	}
}

func TestFKRestrict(t *testing.T) {
	db := openTestDB(t)
	// строгая ссылка не даёт удалить персону, на которую ссылается Relation
	if _, err := db.Exec("INSERT INTO persons(id) VALUES ('p-1'), ('p-2')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		"INSERT INTO relations(id, kind, person_a, person_b) VALUES ('r-1','blood','p-1','p-2')",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM persons WHERE id='p-1'"); err == nil {
		t.Fatal("FK RESTRICT не сработал")
	}
}

func TestFKRestrictOnMissingTarget(t *testing.T) {
	db := openTestDB(t)
	// вставка связи на несуществующую персону отвергается
	if _, err := db.Exec(
		"INSERT INTO relations(id, kind, person_a, person_b) VALUES ('r-1','blood','nope','nope')",
	); err == nil {
		t.Fatal("вставка с висячей ссылкой должна быть отвергнута")
	}
}

func TestFKCascade(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec("INSERT INTO persons(id) VALUES ('p-1')"); err != nil {
		t.Fatal(err)
	}
	ref := insertTextRef(t, db, "Блохин")
	if _, err := db.Exec(
		"INSERT INTO person_names(person_id, surname_id, given_id, patronymic_id) VALUES ('p-1', ?, ?, ?)",
		ref, ref, ref,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO person_notes(person_id, position, text_ref_id) VALUES ('p-1', 0, ?)", ref); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM persons WHERE id='p-1'"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"person_names", "person_notes"} {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("%s count=%d после каскада, want 0", table, n)
		}
	}
}

// TestSourceLinksReferenceCitations фиксирует: source_links ссылается на
// citations через citation_id (не source_id) и цитату нельзя удалить, пока
// на неё есть ссылка.
func TestSourceLinksReferenceCitations(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec(
		"INSERT INTO sources(id, kind, title) VALUES ('s-1','document','Ревизская сказка')",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO citations(id, source_id) VALUES ('c-1','s-1')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		"INSERT INTO source_links(citation_id, target_type, target_id) VALUES ('c-1','person','p-1')",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM citations WHERE id='c-1'"); err == nil {
		t.Fatal("citations нельзя удалять, пока на них ссылается source_links")
	}
	if _, err := db.Exec("DELETE FROM sources WHERE id='s-1'"); err == nil {
		t.Fatal("sources нельзя удалять, пока на них ссылается citations")
	}
}

// TestSearchIndex проверяет, что поисковый индекс принимает нормализованные
// термины и ищется через LIKE по уже опущенной в нижний регистр строке.
func TestSearchIndex(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec("INSERT INTO persons(id) VALUES ('p-1'), ('p-2')"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct{ id, term string }{
		{"p-1", "Семёнов"},
		{"p-2", "Дорожкин"},
	} {
		if _, err := db.Exec(
			`INSERT INTO search_index(entity_table, entity_id, field, term)
			 VALUES ('persons', ?, 'surname', ?)`, r.id, Normalize(r.term),
		); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.Query(
		`SELECT entity_id FROM search_index
		 WHERE entity_table = 'persons' AND term LIKE ? ORDER BY entity_id`,
		Normalize("Семен")+"%",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if len(ids) != 1 || ids[0] != "p-1" {
		t.Fatalf("search_index = %v, want [p-1]", ids)
	}

	// повторная запись того же термина не дублируется (составной PK)
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO search_index(entity_table, entity_id, field, term)
		 VALUES ('persons', 'p-1', 'surname', ?)`, Normalize("Семёнов"),
	); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM search_index").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("search_index count = %d, want 2", n)
	}
}

func TestDBIntegrityAndVacuumInto(t *testing.T) {
	db := openTestDB(t)
	if err := db.IntegrityCheck(); err != nil {
		t.Fatalf("IntegrityCheck: %v", err)
	}
	out := filepath.Join(t.TempDir(), "snapshot.db")
	if err := db.VacuumInto(out); err != nil {
		t.Fatalf("VacuumInto: %v", err)
	}
}

// insertTextRef добавляет строку в text_refs и возвращает её id.
func insertTextRef(t *testing.T, db *DB, text string) int64 {
	t.Helper()
	res, err := db.Exec("INSERT INTO text_refs(text) VALUES (?)", text)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestContextMethodsHonourCancellation(t *testing.T) {
	db := openTestDB(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := db.ExecContext(ctx, "INSERT INTO persons(id) VALUES ('p-1')"); !errors.Is(err, context.Canceled) {
		t.Errorf("ExecContext = %v, want context.Canceled", err)
	}

	if rows, err := db.QueryContext(ctx, "SELECT id FROM persons"); !errors.Is(err, context.Canceled) {
		if rows != nil {
			_ = rows.Close()
		}

		t.Errorf("QueryContext = %v, want context.Canceled", err)
	}

	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(new(int)); !errors.Is(err, context.Canceled) {
		t.Errorf("QueryRowContext = %v, want context.Canceled", err)
	}

	err := db.TxContext(ctx, func(*sql.Tx) error {
		t.Error("fn не должна вызываться при отменённом контексте")

		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("TxContext = %v, want context.Canceled", err)
	}

	if n, _ := db.Count("persons"); n != 0 {
		t.Fatalf("Count persons=%d после отменённых запросов, want 0", n)
	}
}

func TestTxContextCommitAndRollback(t *testing.T) {
	db := openTestDB(t)

	if err := db.TxContext(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO persons(id) VALUES ('p-1')")

		return err
	}); err != nil {
		t.Fatal(err)
	}

	err := db.TxContext(t.Context(), func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT INTO persons(id) VALUES ('p-2')"); err != nil {
			return err
		}

		return fmt.Errorf("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if n, _ := db.Count("persons"); n != 1 {
		t.Fatalf("Count persons=%d, want 1 (p-1 закоммичена, p-2 откатана)", n)
	}
}

// waitTxDone ждёт, пока database/sql откатит транзакцию после отмены ctx: откат
// асинхронный, и без ожидания результат теста зависел бы от планировщика.
func waitTxDone(t *testing.T, tx *sql.Tx) {
	t.Helper()

	for range 1000 {
		if _, err := tx.Exec("SELECT 1"); errors.Is(err, sql.ErrTxDone) {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("транзакция не откатилась после отмены контекста")
}

// TestTxContextCancelInsideKeepsContextError: отмена ctx внутри транзакции
// откатывает её и возвращает ошибку контекста, а не sql.ErrTxDone.
func TestTxContextCancelInsideKeepsContextError(t *testing.T) {
	cases := map[string]func(ctx context.Context, tx *sql.Tx) error{
		"после отмены fn продолжает писать": func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.Exec("INSERT INTO persons(id) VALUES ('p-2')")

			return err
		},
		"fn завершается без ошибки, падает Commit": func(context.Context, *sql.Tx) error { return nil },
	}

	for name, after := range cases {
		t.Run(name, func(t *testing.T) {
			db := openTestDB(t)
			ctx, cancel := context.WithCancel(t.Context())

			err := db.TxContext(ctx, func(tx *sql.Tx) error {
				if _, err := tx.Exec("INSERT INTO persons(id) VALUES ('p-1')"); err != nil {
					return err
				}

				cancel()
				waitTxDone(t, tx)

				return after(ctx, tx)
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("TxContext = %v, want context.Canceled", err)
			}

			if n, _ := db.Count("persons"); n != 0 {
				t.Fatalf("Count persons=%d после отмены внутри транзакции, want 0", n)
			}
		})
	}
}

// TestTxContextPanicRollsBack: паника в fn откатывает транзакцию, повторно
// поднимается и не оставляет единственное соединение занятым.
func TestTxContextPanicRollsBack(t *testing.T) {
	db := openTestDB(t)

	func() {
		defer func() {
			if p := recover(); p != "boom" {
				t.Fatalf("паника = %v, want boom", p)
			}
		}()

		_ = db.TxContext(t.Context(), func(tx *sql.Tx) error {
			if _, err := tx.Exec("INSERT INTO persons(id) VALUES ('p-1')"); err != nil {
				return err
			}

			panic("boom")
		})
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM persons").Scan(&n); err != nil {
		t.Fatalf("соединение занято после паники: %v", err)
	}

	if n != 0 {
		t.Fatalf("Count persons=%d после паники, want 0", n)
	}
}

// TestPragmasApplyToEveryConnection: прагмы заданы в DSN и действуют на любое
// новое соединение пула, а не только на первое (без этого пересозданное
// соединение работало бы без внешних ключей).
func TestPragmasApplyToEveryConnection(t *testing.T) {
	db := openTestDB(t)
	db.d.SetMaxIdleConns(0) // каждый запрос — новое соединение

	for i := 0; i < 3; i++ {
		var fk, busy int
		var journal string

		if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatal(err)
		}

		if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
			t.Fatal(err)
		}

		if err := db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
			t.Fatal(err)
		}

		if fk != 1 || busy != 5000 || journal != "wal" {
			t.Fatalf("соединение %d: foreign_keys=%d busy_timeout=%d journal_mode=%s, want 1/5000/wal", i, fk, busy, journal)
		}
	}
}

// TestOpenDBPathWithSpecialCharacters: путь с пробелом и символами, значимыми
// для URI (?, #, %), открывается ровно по этому пути.
func TestOpenDBPathWithSpecialCharacters(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a b#c?d%e")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "genealogy.db")

	db, err := OpenDB(path)
	if err != nil {
		t.Fatalf("OpenDB(%q): %v", path, err)
	}

	if _, err := db.Exec("INSERT INTO persons(id) VALUES ('p-1')"); err != nil {
		t.Fatal(err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("файл БД не по запрошенному пути: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "genealogy.db") {
			t.Errorf("лишний файл рядом с БД: %s", e.Name())
		}
	}
}
