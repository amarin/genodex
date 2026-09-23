package create_source

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// SourceStore — зависимость сценария: транзакция порта store.Store. Проверка
// хранилища (если задано) и сохранение идут в одной транзакции на переданном
// fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
