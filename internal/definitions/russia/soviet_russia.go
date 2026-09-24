package russia

import "github.com/amarin/genodex/internal/models"

// UnitedSovietSocialistRepublicsSystem — административное деление СССР:
// республика → область → район. Значения — models.AdminDivisionType
// (решение #17), каждый тип однозначно соответствует термину.
var UnitedSovietSocialistRepublicsSystem = models.AdministrativeDivisionSystem{
	Name: "Административное деление СССР",
	Relations: []models.AdminDivisionType{
		models.AdminDivisionRespublika,
		models.AdminDivisionOblast,
		models.AdminDivisionRayon,
	},
}
