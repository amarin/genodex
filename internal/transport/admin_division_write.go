package transport

import "github.com/amarin/genodex/internal/models"

// AdminDivisionCreate — тело POST /api/admin-divisions и аргументы тула
// division_create. Идентификатор генерирует сценарий; parent_id = null у корня.
type AdminDivisionCreate struct {
	Name     string                   `json:"name"`
	Type     models.AdminDivisionType `json:"type"`
	ParentID *models.ID               `json:"parent_id"`
}

// Model возвращает доменную единицу с пустым ID.
func (d AdminDivisionCreate) Model() models.AdministrativeDivision {
	return models.AdministrativeDivision{
		Name:     d.Name,
		Type:     d.Type,
		ParentID: cloneParentID(d.ParentID),
	}
}

// AdminDivisionUpdate — тело PUT /api/admin-divisions/{id} и аргументы тула
// division_update: полная замена полей name/type/parent_id.
type AdminDivisionUpdate struct {
	Name     string                   `json:"name"`
	Type     models.AdminDivisionType `json:"type"`
	ParentID *models.ID               `json:"parent_id"`
}

func (d AdminDivisionUpdate) Model() models.AdministrativeDivision {
	return models.AdministrativeDivision{
		Name:     d.Name,
		Type:     d.Type,
		ParentID: cloneParentID(d.ParentID),
	}
}

func cloneParentID(p *models.ID) *models.ID {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
