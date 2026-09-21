package sqlstore

import (
	"context"
	"database/sql"
	"reflect"
	"sort"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// serviceTables — таблицы, не являющиеся ни сущностью, ни связной таблицей.
var serviceTables = map[string]bool{
	"meta": true, "dates": true, "text_refs": true, "anchors": true,
	"source_links": true, "search_index": true,
}

// TestEntityTablesMatchSchema: entityTables описывает ровно корневые таблицы
// схемы (не связные и не служебные), по одной на каждый тип модели.
func TestEntityTablesMatchSchema(t *testing.T) {
	s := newStore(t)

	g, err := s.graph(t.Context())
	if err != nil {
		t.Fatalf("graph: %v", err)
	}

	tables, err := scanRowsStrings(s, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("tables: %v", err)
	}

	var roots []string

	for _, table := range tables {
		if _, isChild := g.owner(table); !isChild && !serviceTables[table] {
			roots = append(roots, table)
		}
	}

	var mapped []string
	for _, table := range entityTables {
		mapped = append(mapped, table)
	}

	sort.Strings(roots)
	sort.Strings(mapped)

	if len(roots) != len(mapped) {
		t.Fatalf("корневые таблицы схемы %v, entityTables %v", roots, mapped)
	}

	for i := range roots {
		if roots[i] != mapped[i] {
			t.Fatalf("корневые таблицы схемы %v, entityTables %v", roots, mapped)
		}
	}

	for _, typ := range models.AllTypes() {
		if _, ok := entityTables[typ]; !ok {
			t.Errorf("тип %q не отображён на таблицу", typ)
		}
	}

	if len(entityTables) != len(models.AllTypes()) {
		t.Errorf("entityTables: %d записей, типов модели: %d", len(entityTables), len(models.AllTypes()))
	}
}

// TestSchemaGraphShapeIsSupported: deleteEntity рассчитан на схему, где связные
// таблицы лежат на один уровень под сущностью, а строгие ссылки идут только из
// сущностей, связных таблиц и source_links. Тест падает, если схема вышла за
// эти рамки, — тогда deleteEntity/referrers надо расширить.
func TestSchemaGraphShapeIsSupported(t *testing.T) {
	s := newStore(t)

	g, err := s.graph(t.Context())
	if err != nil {
		t.Fatalf("graph: %v", err)
	}

	for _, e := range g.edges {
		if e.onDelete == "CASCADE" {
			if _, nested := g.owner(e.parent); nested {
				t.Errorf("%s.%s каскадом зависит от %s, у которой сам есть владелец", e.child, e.col, e.parent)
			}

			if _, ok := typeOfTable[e.parent]; !ok {
				t.Errorf("%s.%s каскадом зависит от %s — это не таблица сущности", e.child, e.col, e.parent)
			}
		}

		if e.onDelete != "RESTRICT" {
			continue
		}

		if _, isEntityTarget := typeOfTable[e.parent]; !isEntityTarget {
			continue // ссылки на value-таблицы не блокируют удаление сущностей
		}

		_, isChild := g.owner(e.child)
		_, isEntity := typeOfTable[e.child]

		if !isChild && !isEntity && e.child != "source_links" {
			t.Errorf("строгая ссылка %s.%s → %s: источник не сущность, не связная таблица и не source_links",
				e.child, e.col, e.parent)
		}
	}
}

// scanRowsStrings читает однострочные строковые результаты запроса.
func scanRowsStrings(s *Store, query string) ([]string, error) {
	return scanRows(s.run(context.Background()), func(r *sql.Rows) (string, error) {
		var v string
		err := r.Scan(&v)

		return v, err
	}, query)
}

// TestPrivateColumnTablesAreTheEntitiesWithFlag: фильтр AccessPublic опирается
// на граф схемы; набор таблиц с колонкой private — ровно сущности с флагом
// приватности в моделях.
func TestPrivateColumnTablesAreTheEntitiesWithFlag(t *testing.T) {
	s := newStore(t)

	g, err := s.graph(t.Context())
	if err != nil {
		t.Fatalf("graph: %v", err)
	}

	want := []string{
		"archive_documents", "archive_nodes", "archives", "attachments", "citations", "events",
		"families", "notes", "persons", "relations", "repositories", "residences", "sources",
	}

	var got []string

	for _, table := range entityTables {
		if g.hasPrivate(table) {
			got = append(got, table)
		}
	}

	sort.Strings(got)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("таблицы с колонкой private %v, ожидалось %v", got, want)
	}

	for table := range g.private {
		if _, isEntity := typeOfTable[table]; !isEntity {
			t.Errorf("колонка private у таблицы %s, не являющейся сущностью", table)
		}
	}
}
