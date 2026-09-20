package models

import "testing"

func TestAdministrativeDivisionFullForm(t *testing.T) {
	parent := ID("ad-1")
	a := AdministrativeDivision{
		ID:       ID("ad-2"),
		Name:     "Давыдово",
		Type:     AdminDivisionSelo,
		ParentID: &parent,
		Variants: []string{"Давидово"},
		Renames:  []NamedPeriod{{Text: "Давыдово", Since: "1881", Until: "1917"}},
		Items:    []TextRef{{Text: "дворы"}},
	}
	if !a.Type.IsSettlement() {
		t.Fatal("selo должна считаться населённым пунктом")
	}
	if a.ParentID == nil || *a.ParentID != parent {
		t.Fatalf("strict link lost: %+v", a.ParentID)
	}
}
