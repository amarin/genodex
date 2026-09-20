package models

// Source — единая абстракция доказательства. Ссылка на хранилище
// (Repository) и приватность — решение #25/#24. Текст/ядро — в Citation,
// а не здесь (решение #21).
type Source struct {
	ID           ID
	Kind         SourceKind
	Title        string
	Author       string
	Date         *FactDate
	Reliability  Reliability
	RepositoryID ID
	Notes        []TextRef
	Private      bool
}

// EntityType возвращает тип сущности.
func (s *Source) EntityType() Type { return TypeSource }
