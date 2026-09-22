package russia

import (
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// TestSystemsUseCanonicalTypes: значения Relations — валидные
// models.AdminDivisionType (не слова для показа).
func TestSystemsUseCanonicalTypes(t *testing.T) {
	systems := map[string]models.AdministrativeDivisionSystem{
		"TsardomRussiaGovernorateSystem":       TsardomRussiaGovernorateSystem,
		"UnitedSovietSocialistRepublicsSystem": UnitedSovietSocialistRepublicsSystem,
	}

	for name, sys := range systems {
		for _, t2 := range sys.Relations {
			if !t2.Valid() {
				t.Errorf("%s: %q не валидно как models.AdminDivisionType", name, t2)
			}
		}
	}
}
