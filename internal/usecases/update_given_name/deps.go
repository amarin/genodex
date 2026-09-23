package update_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// GivenNameStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
