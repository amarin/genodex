package auth

import (
	"database/sql"
	"time"

	"github.com/amarin/genodex/internal/storage"
)

// SQLStore — реализация Store поверх internal/storage: то же соединение
// SQLite, что и у internal/store/sqlstore, но свои таблицы (auth.md §3).
type SQLStore struct {
	db *storage.DB
}

// NewSQLStore оборачивает уже открытое соединение (совместное с
// internal/store/sqlstore — конструирует internal/app, этап C).
func NewSQLStore(db *storage.DB) *SQLStore {
	return &SQLStore{db: db}
}

var _ Store = (*SQLStore)(nil)

// timeToSQL форматирует обязательное время как RFC3339 UTC (как
// storage/backup.go — единственный прецедент хранения time.Time текстом
// в проекте).
func timeToSQL(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// timeFromSQL разбирает обязательное время.
func timeFromSQL(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// nullTime отдаёт SQL NULL для отсутствующего времени.
func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}

	return timeToSQL(*t)
}

// timePtrFromNull читает необязательное время: NULL → nil.
func timePtrFromNull(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}

	t, err := timeFromSQL(ns.String)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// nullIDPtr отдаёт SQL NULL для отсутствующей ссылки-указателя.
func nullIDPtr(p *ID) any {
	if p == nil {
		return nil
	}

	return string(*p)
}

// idPtrFromNull читает необязательную ссылку: NULL → nil.
func idPtrFromNull(ns sql.NullString) *ID {
	if !ns.Valid {
		return nil
	}

	id := ID(ns.String)

	return &id
}

// checkRowsAffected — ErrNotFound, если операция не затронула ни одной
// строки (целевая строка не существует). Используется UPDATE/DELETE
// «точечных» методов (по id); массовые (DeleteSessionsByOwner) — нет, 0
// совпадений там законный результат.
func checkRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return ErrNotFound
	}

	return nil
}
