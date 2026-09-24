package transport

import "github.com/amarin/genodex/internal/models"

// AdminDivisionTypeInfo — контракт записи справочника типов единиц деления
// (GET /api/admin-division-types, MCP-тул division_type_list). rank — null у
// типа без ранга (other); children — типы, допустимые внутри единицы этого
// типа, в каноническом порядке (пустой массив — добавить ничего нельзя).
type AdminDivisionTypeInfo struct {
	Type       models.AdminDivisionType   `json:"type"`
	Label      string                     `json:"label"`
	Rank       *int                       `json:"rank"`
	Settlement bool                       `json:"settlement"`
	Children   []models.AdminDivisionType `json:"children"`
}

// AdminDivisionTypeInfosFromModels конвертирует справочник; пустой вход даёт
// пустой срез, а не nil.
func AdminDivisionTypeInfosFromModels(infos []models.AdminDivisionTypeInfo) []AdminDivisionTypeInfo {
	out := make([]AdminDivisionTypeInfo, 0, len(infos))

	for _, i := range infos {
		var rank *int
		if i.Rank != nil {
			r := *i.Rank
			rank = &r
		}

		children := append(make([]models.AdminDivisionType, 0, len(i.Children)), i.Children...)

		out = append(out, AdminDivisionTypeInfo{
			Type:       i.Type,
			Label:      i.Label,
			Rank:       rank,
			Settlement: i.Settlement,
			Children:   children,
		})
	}

	return out
}
