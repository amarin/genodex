package update_citation

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// CitationStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, проверка источника/ссылок якоря и сохранение идут
// в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
