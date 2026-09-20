package models

// ArchiveNodeType — уровень системы иерархии архива (фонд/опись/дело/шкаф/…).
type ArchiveNodeType string

// ArchiveNode — рекурсивный узел цепочки хранения в архиве.
type ArchiveNode struct {
	ID          ID
	Type        ArchiveNodeType
	ArchiveID   ID
	ParentID    *ID
	Label       string
	Name        string
	Since       *FactDate
	Until       *FactDate
	Parish      *TextRef
	Settlements []TextRef
	Notes       []TextRef
	Sources     []SourceLink
	Private     bool
}

// EntityType возвращает тип сущности.
func (n *ArchiveNode) EntityType() Type { return TypeArchiveNode }
