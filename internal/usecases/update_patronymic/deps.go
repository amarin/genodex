package update_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// PatronymicStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
