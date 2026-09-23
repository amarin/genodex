package update_source

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// SourceStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, проверка хранилища (если задано) и сохранение идут
// в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
