// Package transport — DTO публичных контрактов (/api и MCP-тулы) и конвертеры
// из домена. Единственное место, где определена форма JSON на проводе; домен
// (internal/models) о JSON не знает.
package transport

import "github.com/amarin/genodex/internal/models"

// AdminDivision — контракт единицы административного деления
// (GET /api/admin-divisions, MCP-тул division_list). parent_id — null у
// корня. Sources — единственное поле полной модели, кроме name/type/
// parent_id, отдаваемое в контракте (подпроект 5: Sources редактируется у
// всех сущностей, где есть, остальные поля — Items/Variants/Renames/
// Successors/Since/Until/Notes — по-прежнему вне контракта, узкий DTO с
// подпроекта 1).
type AdminDivision struct {
	ID       models.ID                `json:"id"`
	Name     string                   `json:"name"`
	Type     models.AdminDivisionType `json:"type"`
	ParentID *models.ID               `json:"parent_id"`
	Sources  []SourceLink             `json:"sources"`
}

// AdminDivisionFromModel конвертирует единицу деления в контракт.
func AdminDivisionFromModel(d models.AdministrativeDivision) AdminDivision {
	var parent *models.ID

	if d.ParentID != nil {
		p := *d.ParentID // копия: контракт не делит указатель с моделью
		parent = &p
	}

	return AdminDivision{ID: d.ID, Name: d.Name, Type: d.Type, ParentID: parent, Sources: SourceLinksFromModel(d.Sources)}
}

// AdminDivisionsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func AdminDivisionsFromModels(ds []models.AdministrativeDivision) []AdminDivision {
	out := make([]AdminDivision, 0, len(ds))
	for _, d := range ds {
		out = append(out, AdminDivisionFromModel(d))
	}

	return out
}
