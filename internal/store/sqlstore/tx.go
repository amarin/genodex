package sqlstore

import (
	"context"
	"database/sql"

	"github.com/amarin/genodex/internal/store"
)

// InTx выполняет fn в одной транзакции; см. store.Store.InTx. Переданный fn
// Store — копия адаптера, привязанная к транзакции (exec = *sql.Tx): его
// методы, включая вложенный InTx, работают в ней.
func (s *Store) InTx(ctx context.Context, fn func(store.Store) error) error {
	if s.scoped {
		if err := ctx.Err(); err != nil {
			return err
		}

		return fn(s)
	}

	return s.db.TxContext(ctx, func(tx *sql.Tx) error {
		return fn(&Store{st: s.st, db: s.db, exec: tx, scoped: true, cache: s.cache})
	})
}
