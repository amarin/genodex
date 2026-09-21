package httpapi

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionService — контракт сценария «список единиц административного деления».
type DivisionService interface {
	ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
}
