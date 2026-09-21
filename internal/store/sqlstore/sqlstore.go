package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

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
//
// Store внутри InTx — копия с exec = транзакция и scoped = true: все методы
// порта работают в ней, вложенный InTx не открывает новую транзакцию.
type Store struct {
	st *storage.Storage
	db *storage.DB

	exec   sqlExecutor  // соединение или (внутри InTx) транзакция
	scoped bool         // true — Store привязан к транзакции InTx
	cache  *schemaCache // общий для копий Store кэш графа внешних ключей
}

// schemaCache — граф внешних ключей схемы, строится при первом удалении.
type schemaCache struct {
	mu sync.Mutex
	g  *schemaGraph
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
	db := st.DB()

	return &Store{st: st, db: db, exec: db, cache: &schemaCache{}}
}

// Close закрывает хранилище.
func (s *Store) Close() error {
	return s.st.Close()
}

// queryer — то, что нужно помощникам чтения (scanRows, loadX, listIDs);
// реализует runner.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// sqlExecutor — то, что умеют и соединение (*storage.DB), и транзакция (*sql.Tx).
type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// runner выполняет запросы с привязанным контекстом: помощники вызывают
// Exec/Query/QueryRow, а отмена ctx доходит до драйвера.
type runner struct {
	ctx context.Context
	x   sqlExecutor
}

// Exec — ExecContext с привязанным контекстом.
func (r runner) Exec(query string, args ...any) (sql.Result, error) {
	return r.x.ExecContext(r.ctx, query, args...)
}

// Query — QueryContext с привязанным контекстом.
func (r runner) Query(query string, args ...any) (*sql.Rows, error) {
	return r.x.QueryContext(r.ctx, query, args...)
}

// QueryRow — QueryRowContext с привязанным контекстом.
func (r runner) QueryRow(query string, args ...any) *sql.Row {
	return r.x.QueryRowContext(r.ctx, query, args...)
}

// run возвращает runner поверх соединения или, внутри InTx, транзакции.
func (s *Store) run(ctx context.Context) runner { return runner{ctx: ctx, x: s.exec} }

// inTx выполняет fn в транзакции с привязанным контекстом; ошибка — откат.
// Внутри InTx транзакция уже открыта: fn выполняется в ней без новой.
func (s *Store) inTx(ctx context.Context, fn func(tx runner) error) error {
	if s.scoped {
		return fn(s.run(ctx))
	}

	return s.db.TxContext(ctx, func(tx *sql.Tx) error {
		return fn(runner{ctx: ctx, x: tx})
	})
}

// notFound сообщает, что строки нет: Get превращает это в models.ErrNotFound,
// остальные ошибки хранилища возвращаются как есть.
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
func listEntities[T any](ctx context.Context, s *Store, table string, get func(context.Context, models.ID) (*T, error)) ([]*T, error) {
	ids, err := listIDs(s.run(ctx), table)
	if err != nil {
		return nil, err
	}

	out := make([]*T, 0, len(ids))

	for _, id := range ids {
		v, err := get(ctx, id)
		if errors.Is(err, models.ErrNotFound) {
			continue // строку удалили между чтением id и Get
		}

		if err != nil {
			return nil, err
		}

		out = append(out, v)
	}

	return out, nil
}
