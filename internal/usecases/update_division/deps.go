package update_division

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// DivisionStore — зависимость сценария: транзакция порта store.Store. Чтение
// текущей единицы, проверка цепочки родителей и сохранение идут в одной
// транзакции на переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type DivisionStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
