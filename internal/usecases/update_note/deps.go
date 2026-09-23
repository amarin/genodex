package update_note

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// NoteStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, цепочки родителей и сохранение идут в одной
// транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type NoteStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
