package models

// Archive — архив (хранилище с системой иерархии). RepositoryID — ссылка на
// репозиторий (решение #25); Private — приватность (решение #24).
type Archive struct {
	ID           ID
	Name         string
	System       *TextRef
	RepositoryID ID
	Notes        []TextRef
	Sources      []SourceLink
	Private      bool
}

// EntityType возвращает тип сущности.
func (a *Archive) EntityType() Type { return TypeArchive }
