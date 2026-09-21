package transport

import (
	"encoding/json"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

func TestAdminDivisionFromModel(t *testing.T) {
	parent := models.ID("ad-root")
	src := models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo,
		ParentID: &parent, Variants: []string{"Давыдова"},
	}

	got := AdminDivisionFromModel(src)

	if got.ID != "ad-1" || got.Name != "Давыдово" || got.Type != models.AdminDivisionSelo ||
		got.ParentID == nil || *got.ParentID != "ad-root" {
		t.Fatalf("got %+v", got)
	}

	if got.ParentID == src.ParentID {
		t.Fatal("контракт делит указатель ParentID с моделью")
	}
}

// Контракт на проводе зафиксирован строкой: пустой список — `[]`, поля —
// id/name/type/parent_id, у корня parent_id — null.
func TestAdminDivisionsJSONContract(t *testing.T) {
	empty, err := json.Marshal(AdminDivisionsFromModels(nil))
	if err != nil {
		t.Fatal(err)
	}

	if string(empty) != `[]` {
		t.Fatalf("empty = %s, want []", empty)
	}

	root := models.ID("ad-root")

	list, err := json.Marshal(AdminDivisionsFromModels([]models.AdministrativeDivision{
		{ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
	}))
	if err != nil {
		t.Fatal(err)
	}

	want := `[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null},` +
		`{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root"}]`
	if string(list) != want {
		t.Fatalf("list = %s, want %s", list, want)
	}
}
