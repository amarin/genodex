package russia

import "github.com/amarin/genodex/internal/models"

const (
	Republic models.AdminDivisionType = "республика"
	Oblast   models.AdminDivisionType = "область"
	Rayon    models.AdminDivisionType = "район"
)

var UnitedSovietSocialistRepublicsSystem = models.AdministrativeDivisionSystem{
	Name:      "Административное деление СССР",
	Relations: []models.AdminDivisionType{Republic, Oblast, Rayon},
}
