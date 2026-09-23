package update_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// AttachmentStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, узла, документа (если задан) и сохранение идут в
// одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AttachmentStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
