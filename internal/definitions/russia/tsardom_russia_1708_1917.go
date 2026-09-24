package russia

import "github.com/amarin/genodex/internal/models"

// TsardomRussiaGovernorateSystem — административное деление Российской империи
// после губернской реформы (1708–1917): губерния → уезд → волость. Значения —
// models.AdminDivisionType (решение #17), каждый тип однозначно соответствует
// термину.
var TsardomRussiaGovernorateSystem = models.AdministrativeDivisionSystem{
	Name: "Административное деление Европейской Части Российской Империи 19 века",
	Relations: []models.AdminDivisionType{
		models.AdminDivisionGuberniya,
		models.AdminDivisionUezd,
		models.AdminDivisionVolost,
	},
}
