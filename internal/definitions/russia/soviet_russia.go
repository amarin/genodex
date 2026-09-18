package russia

import "github.com/amarin/genodex/internal/entity"

const (
	Republic entity.AdministrativeDivisionType = "республика"
	Oblast   entity.AdministrativeDivisionType = "область"
	Rayon    entity.AdministrativeDivisionType = "район"
)

var UnitedSovietSocialistRepublicsSystem = entity.AdministrativeDivisionSystem{
	Name:      "Административное деление СССР",
	Relations: []entity.AdministrativeDivisionType{Republic, Oblast, Rayon},
}
