package update_repository

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// RepositoryStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
