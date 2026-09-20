package russia

import "github.com/amarin/genodex/internal/models"

const (
	Governorate models.AdminDivisionType = "губерния"
	Uyezd       models.AdminDivisionType = "уезд"
	Volost      models.AdminDivisionType = "волость"
)

var TsardomRussiaGovernorateSystem = models.AdministrativeDivisionSystem{
	Name:      "Административное деление Европейской Части Российской Империи 19 века",
	Relations: []models.AdminDivisionType{Governorate, Uyezd, Volost},
}
