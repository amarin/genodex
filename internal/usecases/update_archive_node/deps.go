package update_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ArchiveNodeStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, архива, родителя (в т.ч. цепочки на цикл) и
// сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveNodeStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
