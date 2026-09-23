package update_estate

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// EstateStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
