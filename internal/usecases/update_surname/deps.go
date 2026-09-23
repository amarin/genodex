package update_surname

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// SurnameStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
