package transport

import (
	"encoding/json"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

func TestAdminDivisionCreateJSONAndModel(t *testing.T) {
	src := []byte(`{"name":"Давыдово","type":"selo","parent_id":"AD-root"}`)
	var got AdminDivisionCreate
	if err := json.Unmarshal(src, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "Давыдово" || got.Type != models.AdminDivisionSelo {
		t.Fatalf("got %+v", got)
	}
	if got.ParentID == nil || *got.ParentID != "AD-root" {
		t.Fatalf("parent_id = %v", got.ParentID)
	}

	m := got.Model()
	if m.ID != "" {
		t.Fatalf("model id заведомо пуст: %q", m.ID)
	}
	if m.Name != "Давыдово" || m.Type != models.AdminDivisionSelo || m.ParentID == nil || *m.ParentID != "AD-root" {
		t.Fatalf("model = %+v", m)
	}
	if m.ParentID == got.ParentID {
		t.Fatal("model делит указатель parent_id с DTO")
	}
}

func TestAdminDivisionCreateRoot(t *testing.T) {
	src := []byte(`{"name":"Московская","type":"guberniya"}`)
	var got AdminDivisionCreate
	if err := json.Unmarshal(src, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	m := got.Model()
	if m.ParentID != nil {
		t.Fatalf("parent_id должен быть nil, got %v", m.ParentID)
	}
}

func TestAdminDivisionUpdateModel(t *testing.T) {
	src := []byte(`{"name":"Давыдово","type":"selo"}`)
	var got AdminDivisionUpdate
	if err := json.Unmarshal(src, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	m := got.Model()
	if m.Name != "Давыдово" || m.Type != models.AdminDivisionSelo || m.ParentID != nil {
		t.Fatalf("model = %+v", m)
	}
	if m.ID != "" {
		t.Fatalf("id задаёт обработчик при слиянии, в Model() пусто: %q", m.ID)
	}
}

func TestInUseErrorBodyJSONContract(t *testing.T) {
	const idV = "AD-01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const id0 = "AD-01ARZ3NDEKTSV4RRFFQ69G5FA0"
	e := &models.InUseError{
		Type: models.TypeAdministrativeDivision,
		ID:   models.ID(idV),
		Referrers: []models.EntityRef{
			{Type: models.TypeAdministrativeDivision, ID: models.ID(id0)},
		},
	}
	data, err := json.Marshal(InUseErrorBodyFromModel(e))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"error":"administrative_division \"` + idV + `\" используется: administrative_division ` + id0 + `","referrers":[{"type":"administrative_division","id":"` + id0 + `"}]}`
	if string(data) != want {
		t.Fatalf("body = %s, want %s", data, want)
	}
}
