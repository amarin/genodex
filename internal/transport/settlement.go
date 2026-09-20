// Package transport — DTO публичных контрактов (/api и MCP-тулы) и конвертеры
// из домена. Единственное место, где определена форма JSON на проводе; домен
// (internal/models) о JSON не знает.
package transport

import "github.com/amarin/genodex/internal/models"

// Settlement — контракт списка населённых пунктов (GET /api/settlements,
// MCP-тул settlement_list).
type Settlement struct {
	ID   models.ID                `json:"id"`
	Name string                   `json:"name"`
	Type models.AdminDivisionType `json:"type"`
}

// SettlementFromModel конвертирует единицу деления в контракт.
func SettlementFromModel(d models.AdministrativeDivision) Settlement {
	return Settlement{ID: d.ID, Name: d.Name, Type: d.Type}
}

// SettlementsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func SettlementsFromModels(ds []models.AdministrativeDivision) []Settlement {
	out := make([]Settlement, 0, len(ds))
	for _, d := range ds {
		out = append(out, SettlementFromModel(d))
	}
	return out
}
