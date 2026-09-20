package models

// ArchiveDocument — документ внутри единицы учёта (метрическая книга за годы…).
type ArchiveDocument struct {
	ID          ID
	UnitID      ID
	Title       string
	Kind        string
	Since       *FactDate
	Until       *FactDate
	Parish      *TextRef
	Settlements []TextRef
	Notes       []TextRef
	Sources     []SourceLink
	Private     bool
}

// EntityType возвращает тип сущности.
func (d *ArchiveDocument) EntityType() Type { return TypeArchiveDocument }
