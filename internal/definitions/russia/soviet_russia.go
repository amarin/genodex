package russia

import "github.com/amarin/genodex/internal/models"

// Словарь временный, до блока D в docs/todo.md: значения — слова для показа,
// а не канонические значения перечисления models.AdminDivisionType.
const (
	Republic models.AdminDivisionType = "республика"
	Oblast   models.AdminDivisionType = "область"
	Rayon    models.AdminDivisionType = "район"
)

var UnitedSovietSocialistRepublicsSystem = models.AdministrativeDivisionSystem{
	Name:      "Административное деление СССР",
	Relations: []models.AdminDivisionType{Republic, Oblast, Rayon},
}
