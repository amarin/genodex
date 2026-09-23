package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveNodeCreate — тело POST /api/archive-nodes и аргументы тула
// archive_node_create. Идентификатор генерирует сценарий. Sources —
// редактируемое с самого начала (это новая сущность, не ретрофит, см.
// docs/data-model/entity-write.md §3.4).
type ArchiveNodeCreate struct {
	Type        string       `json:"type"`
	ArchiveID   models.ID    `json:"archive_id"`
	ParentID    *models.ID   `json:"parent_id,omitempty"`
	Label       string       `json:"label"`
	Name        string       `json:"name"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (n ArchiveNodeCreate) Model() models.ArchiveNode {
	return models.ArchiveNode{
		Type:        models.ArchiveNodeType(n.Type),
		ArchiveID:   n.ArchiveID,
		ParentID:    n.ParentID,
		Label:       n.Label,
		Name:        n.Name,
		Since:       n.Since.Model(),
		Until:       n.Until.Model(),
		Parish:      n.Parish.ModelPtr(),
		Settlements: TextRefsToModel(n.Settlements),
		Notes:       TextRefsToModel(n.Notes),
		Sources:     SourceLinksToModel(n.Sources),
		Private:     n.Private,
	}
}

// ArchiveNodeUpdate — тело PUT /api/archive-nodes/{id} и аргументы тула
// archive_node_update: полная замена всех полей ниже id.
type ArchiveNodeUpdate struct {
	Type        string       `json:"type"`
	ArchiveID   models.ID    `json:"archive_id"`
	ParentID    *models.ID   `json:"parent_id,omitempty"`
	Label       string       `json:"label"`
	Name        string       `json:"name"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (n ArchiveNodeUpdate) Model() models.ArchiveNode {
	return models.ArchiveNode{
		Type:        models.ArchiveNodeType(n.Type),
		ArchiveID:   n.ArchiveID,
		ParentID:    n.ParentID,
		Label:       n.Label,
		Name:        n.Name,
		Since:       n.Since.Model(),
		Until:       n.Until.Model(),
		Parish:      n.Parish.ModelPtr(),
		Settlements: TextRefsToModel(n.Settlements),
		Notes:       TextRefsToModel(n.Notes),
		Sources:     SourceLinksToModel(n.Sources),
		Private:     n.Private,
	}
}
