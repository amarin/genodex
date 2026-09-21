package models

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound — запрошенной сущности нет. Get-методы порта возвращают её
// (проверять через errors.Is); обработчики превращают в «не найдено».
var ErrNotFound = errors.New("не найдено")

// MaxReferrers — предел списка ссылающихся сущностей в InUseError.
const MaxReferrers = 20

// EntityRef — ссылка на сущность: тип и идентификатор.
type EntityRef struct {
	Type Type
	ID   ID
}

// InUseError — удаление запрещено: на сущность ссылаются другие. Referrers —
// первые MaxReferrers ссылающихся в стабильном порядке, без повторов.
// Обработчики находят её через errors.As и превращают в конфликт (409).
type InUseError struct {
	Type      Type
	ID        ID
	Referrers []EntityRef
}

func (e *InUseError) Error() string {
	refs := make([]string, len(e.Referrers))
	for i, r := range e.Referrers {
		refs[i] = string(r.Type) + " " + string(r.ID)
	}

	return fmt.Sprintf("%s %q используется: %s", e.Type, e.ID, strings.Join(refs, ", "))
}
