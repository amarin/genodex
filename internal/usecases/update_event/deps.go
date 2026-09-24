package update_event

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// EventStore — зависимость сценария: транзакция порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
