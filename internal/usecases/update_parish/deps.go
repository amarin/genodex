package update_parish

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ParishStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
