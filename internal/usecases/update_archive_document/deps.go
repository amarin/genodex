package update_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ArchiveDocumentStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, единицы учёта (unit_id) и сохранение идут в одной
// транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
