package russia

import "github.com/amarin/genodex/internal/entity"

const (
	Governorate entity.AdministrativeDivisionType = "губерния"
	Uyezd       entity.AdministrativeDivisionType = "уезд"
	Volost      entity.AdministrativeDivisionType = "волость"
)

var TsardomRussiaGovernorateSystem = entity.AdministrativeDivisionSystem{
	Name:      "Административное деление Европейской Части Российской Империи 19 века",
	Relations: []entity.AdministrativeDivisionType{Governorate, Uyezd, Volost},
}
