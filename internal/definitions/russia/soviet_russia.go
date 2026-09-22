package russia

import "github.com/amarin/genodex/internal/models"

// UnitedSovietSocialistRepublicsSystem — административное деление СССР:
// республика → область → район (три уровня, позиционно соответствующие
// губернии/уезду/волости Российской империи). Значения — канонические
// models.AdminDivisionType (решение #17); соответствие историческим
// названиям — в docs/models/places.md.
var UnitedSovietSocialistRepublicsSystem = models.AdministrativeDivisionSystem{
	Name: "Административное деление СССР",
	Relations: []models.AdminDivisionType{
		models.AdminDivisionGovernorate, // республика
		models.AdminDivisionDistrict,    // область
		models.AdminDivisionVolost,      // район
	},
}
