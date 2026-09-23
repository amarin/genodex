package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveNode — контракт узла архивного дерева (GET /api/archive-nodes,
// MCP-тул archive_node_list). ArchiveID — обязательная строгая ссылка на
// архив, ParentID — необязательная ссылка на родительский узел того же
// архива (см. create_archive_node/update_archive_node — сценарии проверяют
// оба инварианта). Parish — одиночная необязательная ссылка (текст или
// ссылка на приход), по образцу Parish.Church/Church.Parish.
type ArchiveNode struct {
	ID          models.ID    `json:"id"`
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

// ArchiveNodeFromModel конвертирует запись в контракт.
func ArchiveNodeFromModel(n models.ArchiveNode) ArchiveNode {
	return ArchiveNode{
		ID:          n.ID,
		Type:        string(n.Type),
		ArchiveID:   n.ArchiveID,
		ParentID:    n.ParentID,
		Label:       n.Label,
		Name:        n.Name,
		Since:       FactDateFromModel(n.Since),
		Until:       FactDateFromModel(n.Until),
		Parish:      TextRefFromModelPtr(n.Parish),
		Settlements: TextRefsFromModel(n.Settlements),
		Notes:       TextRefsFromModel(n.Notes),
		Sources:     SourceLinksFromModel(n.Sources),
		Private:     n.Private,
	}
}

// ArchiveNodesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ArchiveNodesFromModels(ns []models.ArchiveNode) []ArchiveNode {
	out := make([]ArchiveNode, 0, len(ns))
	for _, n := range ns {
		out = append(out, ArchiveNodeFromModel(n))
	}

	return out
}
