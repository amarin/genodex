package models

// Repository — хранилище-контейнер источников (архив, библиотека, музей,
// частное собрание и т.п.). Source.repository_id / Archive.repository_id —
// мягкие ссылки на него.
type Repository struct {
	ID      ID
	Name    string
	Type    RepositoryType
	Address string
	URLs    []TextRef
	Notes   []TextRef
	Sources []SourceLink
	Private bool
}

// EntityType возвращает тип сущности.
func (r *Repository) EntityType() Type { return TypeRepository }
