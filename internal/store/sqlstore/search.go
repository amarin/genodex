package sqlstore

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/storage"
)

// hitLabelExprs — SQL-выражение подписи сущности по таблице главной строки.
// Персоны (имя из трёх TextRef) и таблицы без поисковых терминов (relations,
// residences) обрабатываются отдельно. Пустая подпись заменяется на ID.
var hitLabelExprs = map[string]string{
	"families":                 "name",
	"surnames":                 "canonical",
	"given_names":              "canonical",
	"patronymics":              "canonical",
	"estates":                  "canonical",
	"titles":                   "canonical",
	"administrative_divisions": "name",
	"churches":                 "name",
	"parishes":                 "name",
	"events":                   "type || COALESCE(' ' || (SELECT text FROM text_refs WHERE id = events.place_id), '')",
	"sources":                  "title",
	"archives":                 "name",
	"archive_nodes":            "trim(label || ' ' || name)",
	"archive_documents":        "title",
	"attachments":              "CASE WHEN filename != '' THEN filename ELSE uri END",
	"citations":                "text",
	"notes":                    "title",
	"repositories":             "name",
}

// maxRune — верхняя граница диапазона префиксного поиска: любая строка,
// начинающаяся с префикса, меньше prefix+maxRune.
const maxRune = "\U0010FFFF"

// searchSQL строит запрос Search: диапазон по term (использует idx_search_term),
// одна строка на сущность, для публичного доступа — исключение приватных строк
// таблиц privateTables (имена берутся из схемы и заключаются в кавычки,
// значения — параметрами). Порядок — по таблице сущности и id.
func searchSQL(privateTables []string) string {
	var b strings.Builder

	b.WriteString(`SELECT si.entity_table, si.entity_id, MIN(si.field)
		FROM search_index si
		WHERE si.term >= ? AND si.term < ?`)

	for _, t := range privateTables {
		b.WriteString(` AND NOT (si.entity_table = '` + t + `' AND EXISTS (
			SELECT 1 FROM "` + t + `" e WHERE e.id = si.entity_id AND e.private = 1))`)
	}

	b.WriteString(`
		GROUP BY si.entity_table, si.entity_id
		ORDER BY si.entity_table, si.entity_id
		LIMIT ? OFFSET ?`)

	return b.String()
}

// Search ищет сущности по префиксу термина; см. store.Store.Search.
func (s *Store) Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error) {
	prefix := storage.Normalize(strings.TrimSpace(query))
	if prefix == "" {
		return []models.Hit{}, nil
	}

	g, err := s.graph(ctx)
	if err != nil {
		return nil, err
	}

	var privateTables []string

	if access != models.AccessFull {
		for table := range g.private {
			privateTables = append(privateTables, table)
		}

		sort.Strings(privateTables) // стабильный текст запроса
	}

	page = page.Normalized()
	q := s.run(ctx)

	hits, err := scanRows(q, func(r *sql.Rows) (models.Hit, error) {
		var table, id, field string
		err := r.Scan(&table, &id, &field)

		return models.Hit{Type: typeOfTable[table], ID: models.ID(id), Field: field}, err
	}, searchSQL(privateTables), prefix, prefix+maxRune, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}

	if err := fillHitLabels(q, hits); err != nil {
		return nil, err
	}

	if hits == nil {
		hits = []models.Hit{}
	}

	return hits, nil
}

// fillHitLabels проставляет подписи: по одному запросу на вид сущности окна.
func fillHitLabels(q queryer, hits []models.Hit) error {
	byType := map[models.Type][]string{}

	for _, h := range hits {
		byType[h.Type] = append(byType[h.Type], string(h.ID))
	}

	labels := map[models.Type]map[string]string{}

	for typ, ids := range byType {
		l, err := loadHitLabels(q, entityTables[typ], ids)
		if err != nil {
			return err
		}

		labels[typ] = l
	}

	for i := range hits {
		hits[i].Label = labels[hits[i].Type][string(hits[i].ID)]
		if hits[i].Label == "" {
			hits[i].Label = string(hits[i].ID)
		}
	}

	return nil
}

// loadHitLabels читает подписи сущностей одной таблицы: id → подпись.
func loadHitLabels(q queryer, table string, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))

	if table == "persons" {
		names, err := queryGrouped(q, ids, nil, func(in string) string {
			return `SELECT pn.person_id, s.text, g.text, p.text FROM person_names pn
			        JOIN text_refs s ON s.id = pn.surname_id
			        JOIN text_refs g ON g.id = pn.given_id
			        JOIN text_refs p ON p.id = pn.patronymic_id
			        WHERE pn.person_id IN (` + in + `) ORDER BY pn.id`
		}, func(r *sql.Rows) (string, string, error) {
			var owner, sur, giv, pat string
			err := r.Scan(&owner, &sur, &giv, &pat)

			return owner, joinNonEmpty(sur, giv, pat), err
		})
		if err != nil {
			return nil, err
		}

		for id, labels := range names {
			out[id] = labels[0] // первое имя персоны
		}

		return out, nil
	}

	expr, ok := hitLabelExprs[table]
	if !ok {
		return out, nil // таблица без подписи: останется ID
	}

	rows, err := queryGrouped(q, ids, nil, func(in string) string {
		return `SELECT id, ` + expr + ` FROM ` + table + ` WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (string, string, error) {
		var id, label string
		err := r.Scan(&id, &label)

		return id, label, err
	})
	if err != nil {
		return nil, err
	}

	for id, labels := range rows {
		out[id] = labels[0]
	}

	return out, nil
}

// joinNonEmpty склеивает непустые части через пробел.
func joinNonEmpty(parts ...string) string {
	var kept []string

	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}

	return strings.Join(kept, " ")
}
