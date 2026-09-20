package transport

import (
	"encoding/json"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

func TestSettlementFromModel(t *testing.T) {
	got := SettlementFromModel(models.AdministrativeDivision{
		ID:       "ad-1",
		Name:     "Давыдово",
		Type:     models.AdminDivisionSelo,
		Variants: []string{"Давыдова"},
	})
	want := Settlement{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Контракт на проводе зафиксирован строкой: пустой список — `[]`, поля — id/name/type.
func TestSettlementsJSONContract(t *testing.T) {
	empty, err := json.Marshal(SettlementsFromModels(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != `[]` {
		t.Fatalf("empty = %s, want []", empty)
	}

	list, err := json.Marshal(SettlementsFromModels([]models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
		{ID: "ad-2", Name: "Никифорово", Type: models.AdminDivisionDerevnya},
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"id":"ad-1","name":"Давыдово","type":"selo"},{"id":"ad-2","name":"Никифорово","type":"derevnya"}]`
	if string(list) != want {
		t.Fatalf("list = %s, want %s", list, want)
	}
}
