package storage

import (
	"database/sql"
	"fmt"
	"strings"
)

// createFKIndexes создаёт индекс на каждую FK-колонку каждой таблицы схемы,
// у которой ещё нет индекса с этой колонкой первой (автоиндексы первичных и
// уникальных ключей и явные индексы DDL учитываются; индексы по выражению и
// частичные индексы покрытием не считаются). Индекс на FK-колонке
// нужен и выборке дочерних строк по владельцу, и проверке внешних ключей при
// удалении родителя (иначе SQLite сканирует дочернюю таблицу целиком).
//
// Список генерируется по PRAGMA foreign_key_list, а не перечисляется вручную:
// новая таблица со ссылками получает индексы автоматически. Операция
// идемпотентна (CREATE INDEX IF NOT EXISTS) и не требует миграции данных.
// Сортировка (collation) индекса не проверяется, а явные индексы DDL не должны
// занимать имена вида idx_<таблица>_<колонка>: иначе IF NOT EXISTS молча
// пропустит создание.
func createFKIndexes(d *sql.DB) error {
	tables, err := queryStrings(d,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return fmt.Errorf("список таблиц: %w", err)
	}

	for _, table := range tables {
		fkCols, err := queryStrings(d,
			`SELECT "from" FROM pragma_foreign_key_list(?) ORDER BY id, seq`, table)
		if err != nil {
			return fmt.Errorf("внешние ключи %s: %w", table, err)
		}
		if len(fkCols) == 0 {
			continue
		}

		leading, err := queryStrings(d,
			`SELECT ii.name FROM pragma_index_list(?) AS il, pragma_index_info(il.name) AS ii WHERE ii.seqno = 0 AND ii.name IS NOT NULL AND il.partial = 0`,
			table)
		if err != nil {
			return fmt.Errorf("индексы %s: %w", table, err)
		}
		covered := make(map[string]bool, len(leading))
		for _, col := range leading {
			covered[col] = true
		}

		for _, col := range fkCols {
			if covered[col] {
				continue
			}
			ddl := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (%s)`,
				quoteIdent("idx_"+table+"_"+col), quoteIdent(table), quoteIdent(col))
			if _, err := d.Exec(ddl); err != nil {
				return fmt.Errorf("индекс %s.%s: %w", table, col, err)
			}
			covered[col] = true
		}
	}

	return nil
}

// queryStrings выполняет запрос и возвращает первый столбец всех строк.
// Курсор закрывается до возврата: соединение одно, вложенные запросы и DDL при
// открытом курсоре зависают.
func queryStrings(d *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}

	return out, rows.Err()
}

// quoteIdent экранирует идентификатор для подстановки в текст SQL.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
