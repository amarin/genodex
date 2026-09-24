// Package list_division_types — сценарий «справочник типов единиц
// административного деления».
package list_division_types

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «справочник типов единиц административного деления».
// Данные встроены в домен (models.AdminDivisionTypeInfos), зависимостей нет.
type Scenario struct{}

// New создаёт сценарий.
func New() *Scenario {
	return &Scenario{}
}

// ListDivisionTypes возвращает все типы в каноническом порядке с названием,
// рангом, признаком населённого пункта и допустимыми дочерними типами.
func (s *Scenario) ListDivisionTypes(_ context.Context) ([]models.AdminDivisionTypeInfo, error) {
	return models.AdminDivisionTypeInfos(), nil
}
