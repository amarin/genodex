package russia

import "github.com/amarin/genodex/internal/models"

// TsardomRussiaGovernorateSystem — административное деление Российской империи
// после губернской реформы (1708–1917): губерния → уезд → волость. Значения —
// канонические models.AdminDivisionType (решение #17); соответствие
// историческим названиям — в docs/models/places.md.
var TsardomRussiaGovernorateSystem = models.AdministrativeDivisionSystem{
	Name: "Административное деление Европейской Части Российской Империи 19 века",
	Relations: []models.AdminDivisionType{
		models.AdminDivisionGovernorate, // губерния
		models.AdminDivisionDistrict,    // уезд
		models.AdminDivisionVolost,      // волость
	},
}
