package sqlstore

import (
	"context"
	"database/sql"
	"sort"

	"github.com/amarin/genodex/internal/models"
)

// Граф внешних ключей схемы. Удаление сущности и поиск ссылающихся строятся по
// нему, а не по ручному списку таблиц: новая связная таблица или новая строгая
// ссылка учитываются автоматически, как и в storage.createFKIndexes.

// entityTables — таблица главной строки каждой сущности порта.
var entityTables = map[models.Type]string{
	models.TypePerson:                 "persons",
	models.TypeRelation:               "relations",
	models.TypeResidence:              "residences",
	models.TypeFamily:                 "families",
	models.TypeSurname:                "surnames",
	models.TypeGivenName:              "given_names",
	models.TypePatronymic:             "patronymics",
	models.TypeEstate:                 "estates",
	models.TypeTitle:                  "titles",
	models.TypeAdministrativeDivision: "administrative_divisions",
	models.TypeChurch:                 "churches",
	models.TypeParish:                 "parishes",
	models.TypeEvent:                  "events",
	models.TypeSource:                 "sources",
	models.TypeArchive:                "archives",
	models.TypeArchiveNode:            "archive_nodes",
	models.TypeArchiveDocument:        "archive_documents",
	models.TypeAttachment:             "attachments",
	models.TypeCitation:               "citations",
	models.TypeNote:                   "notes",
	models.TypeRepository:             "repositories",
}

// typeOfTable — обратное отображение: таблица главной строки → тип сущности.
var typeOfTable = func() map[string]models.Type {
	m := make(map[string]models.Type, len(entityTables))
	for t, table := range entityTables {
		m[table] = t
	}

	return m
}()

// valueTables — общие таблицы значений: строки принадлежат одной строке-владельцу
// и удаляются вместе с ней (см. очистку сирот в helpers.go).
var valueTables = []string{"text_refs", "dates", "anchors"}

// fkEdge — один внешний ключ: колонка child.col ссылается на parent.
type fkEdge struct {
	child, col, parent, onDelete string
}

// schemaGraph — все внешние ключи схемы в стабильном порядке.
type schemaGraph struct {
	edges []fkEdge
}

// loadSchemaGraph читает внешние ключи всех таблиц одним запросом.
func loadSchemaGraph(ctx context.Context, s *Store) (*schemaGraph, error) {
	edges, err := scanRows(s.run(ctx), func(r *sql.Rows) (fkEdge, error) {
		var e fkEdge
		err := r.Scan(&e.child, &e.col, &e.parent, &e.onDelete)

		return e, err
	}, `SELECT m.name, f."from", f."table", f.on_delete
		FROM sqlite_master AS m, pragma_foreign_key_list(m.name) AS f
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'
		ORDER BY m.name, f.id, f.seq`)
	if err != nil {
		return nil, err
	}

	return &schemaGraph{edges: edges}, nil
}

// graph возвращает граф схемы; результат кэшируется, ошибка — нет (отменённый
// контекст не должен «отравить» хранилище). Внутри InTx запрос идёт в той же
// транзакции, поэтому не блокируется на единственном соединении.
func (s *Store) graph(ctx context.Context) (*schemaGraph, error) {
	if g := s.cache.g.Load(); g != nil {
		return g, nil
	}

	g, err := loadSchemaGraph(ctx, s)
	if err != nil {
		return nil, err
	}

	// параллельная загрузка даёт тот же граф: побеждает первый записанный
	if !s.cache.g.CompareAndSwap(nil, g) {
		g = s.cache.g.Load()
	}

	return g, nil
}

// cascadeChildren — связные таблицы, каскадно удаляемые вместе с родителем.
func (g *schemaGraph) cascadeChildren(parent string) []fkEdge {
	return g.filter(func(e fkEdge) bool { return e.parent == parent && e.onDelete == "CASCADE" })
}

// restrictInto — строгие ссылки на таблицу (запрещают удаление строки-цели).
func (g *schemaGraph) restrictInto(parent string) []fkEdge {
	return g.filter(func(e fkEdge) bool { return e.parent == parent && e.onDelete == "RESTRICT" })
}

// owner возвращает каскадную ссылку связной таблицы на её владельца.
func (g *schemaGraph) owner(child string) (fkEdge, bool) {
	for _, e := range g.edges {
		if e.child == child && e.onDelete == "CASCADE" {
			return e, true
		}
	}

	return fkEdge{}, false
}

// valueCols группирует колонки таблицы, ссылающиеся на общие value-таблицы:
// value-таблица → колонки (порядок колонок — как в схеме).
func (g *schemaGraph) valueCols(table string) map[string][]string {
	out := map[string][]string{}

	for _, e := range g.edges {
		if e.child != table {
			continue
		}

		for _, v := range valueTables {
			if e.parent == v {
				out[v] = append(out[v], e.col)
			}
		}
	}

	return out
}

func (g *schemaGraph) filter(keep func(fkEdge) bool) []fkEdge {
	var out []fkEdge

	for _, e := range g.edges {
		if keep(e) {
			out = append(out, e)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].child != out[j].child {
			return out[i].child < out[j].child
		}

		return out[i].col < out[j].col
	})

	return out
}
