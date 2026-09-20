package sqlstore

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/storage"
	"github.com/amarin/genodex/internal/store"
)

// Store — реализация порта store.Store поверх internal/storage: каждая
// сущность — набор строк колоночной схемы (сущность = таблица, коллекция =
// связная таблица, value-тип = общая таблица dates/text_refs/anchors).
// Дизайн: docs/data-model/normalization-s1s2.md §5–§6. JSON-блобов нет.
//
// Соединение у storage.DB одно (SetMaxOpenConns(1)): вложенный запрос при
// открытом курсоре встаёт в дедлок, поэтому все помощники читают строки
// целиком и закрывают курсор до следующего запроса.
type Store struct {
	st *storage.Storage
	db *storage.DB
}

var _ store.Store = (*Store)(nil)

// Open открывает адаптер в каталоге данных.
func Open(dataDir string) (*Store, error) {
	st, err := storage.Open(dataDir)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}

	return New(st), nil
}

// New оборачивает уже открытое хранилище.
func New(st *storage.Storage) *Store {
	return &Store{st: st, db: st.DB()}
}

// Close закрывает хранилище.
func (s *Store) Close() error {
	return s.st.Close()
}

// queryer — общий интерфейс чтения для *storage.DB и *sql.Tx.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// notFound сообщает, что строки нет. Контракт Get по всему порту: сущность
// не найдена — (nil, nil), ошибки хранилища возвращаются как есть.
func notFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// boolInt переводит признак приватности в целочисленную колонку SQLite.
func boolInt(v bool) int {
	if v {
		return 1
	}

	return 0
}

// nullID отдаёт SQL NULL вместо пустого идентификатора: необязательный FK
// с пустой строкой нарушил бы RESTRICT (такой строки-цели нет).
func nullID(id models.ID) any {
	if id == "" {
		return nil
	}

	return string(id)
}

// nullIDPtr — то же для необязательной ссылки-указателя.
func nullIDPtr(p *models.ID) any {
	if p == nil {
		return nil
	}

	return nullID(*p)
}

// nullInt64 отдаёт SQL NULL вместо нулевого id value-строки.
func nullInt64(v int64) any {
	if v == 0 {
		return nil
	}

	return v
}

// idFrom читает необязательный идентификатор: NULL → пустой ID.
func idFrom(ns sql.NullString) models.ID {
	if !ns.Valid {
		return ""
	}

	return models.ID(ns.String)
}

// idPtrFrom читает необязательную ссылку-указатель: NULL → nil.
func idPtrFrom(ns sql.NullString) *models.ID {
	if !ns.Valid || ns.String == "" {
		return nil
	}

	id := models.ID(ns.String)

	return &id
}

// int64From читает необязательный id value-строки: NULL → 0.
func int64From(n sql.NullInt64) int64 {
	if !n.Valid {
		return 0
	}

	return n.Int64
}

// scanRows читает все строки запроса целиком и закрывает курсор: одно
// соединение не допускает вложенных запросов при открытом *sql.Rows.
func scanRows[T any](q queryer, scan func(*sql.Rows) (T, error), query string, args ...any) ([]T, error) {
	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []T

	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, v)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, rows.Close()
}

// collectIDs собирает значения колонок-ссылок на value-таблицы (NULL и нули
// пропускаются). Курсор закрывается до возврата.
func collectIDs(q queryer, query string, args ...any) ([]int64, error) {
	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []int64

	for rows.Next() {
		vals := make([]sql.NullInt64, len(cols))
		ptrs := make([]any, len(cols))

		for i := range vals {
			ptrs[i] = &vals[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		for _, v := range vals {
			if v.Valid && v.Int64 != 0 {
				out = append(out, v.Int64)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, rows.Close()
}

// listIDs возвращает идентификаторы всех строк таблицы в порядке вставки.
func listIDs(q queryer, table string) ([]models.ID, error) {
	return scanRows(q, func(r *sql.Rows) (models.ID, error) {
		var id string
		err := r.Scan(&id)

		return models.ID(id), err
	}, `SELECT id FROM `+table+` ORDER BY rowid`)
}

// listEntities читает список сущностей таблицы: сначала все id (курсор
// закрыт), затем Get по каждому. Масштаб личной генеалогии это позволяет.
func listEntities[T any](s *Store, table string, get func(models.ID) (*T, error)) ([]*T, error) {
	ids, err := listIDs(s.db, table)
	if err != nil {
		return nil, err
	}

	out := make([]*T, 0, len(ids))

	for _, id := range ids {
		v, err := get(id)
		if err != nil {
			return nil, err
		}

		if v == nil {
			continue
		}

		out = append(out, v)
	}

	return out, nil
}
