package mcp

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionService — контракт сценариев административного деления, отдаваемых в
// MCP-тулы: список, чтение, создание, изменение, удаление.
type DivisionService interface {
	ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
	SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error)
	GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error)
	CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error)
	UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error
	DeleteDivision(ctx context.Context, id models.ID) error
}

// SurnameService — контракт сценариев словарных записей фамилий, отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type SurnameService interface {
	ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error)
	SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error)
	GetSurname(ctx context.Context, id models.ID) (models.Surname, error)
	CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error)
	UpdateSurname(ctx context.Context, sn models.Surname) error
	DeleteSurname(ctx context.Context, id models.ID) error
}
