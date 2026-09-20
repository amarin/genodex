package models

import "testing"

func TestDictionaryFullForm(t *testing.T) {
	s := Surname{
		ID:        ID("sur-1"),
		Canonical: "Дорожкин",
		Variants:  []TextRef{{Text: "Дарожкин"}, {Text: "Дорожкина"}},
		Items:     []TextRef{{Text: "Иван Дорожкин", Ref: "p-1", Type: TypePerson}},
	}
	if s.EntityType() != TypeSurname {
		t.Fatalf("EntityType() = %q", s.EntityType())
	}
	if len(s.Variants) != 2 || s.Items[0].Ref != "p-1" {
		t.Fatalf("form lost: %+v", s)
	}
}

func TestGivenNameGenderRequired(t *testing.T) {
	// Gender — обязательный номинальный домен: без него нельзя строить
	// вывод пола (docs/models/people.md).
	g := GivenName{ID: ID("g-1"), Canonical: "Акилина", Gender: FemaleName}
	if g.Gender != FemaleName {
		t.Fatalf("Gender: expected %q, got %q", FemaleName, g.Gender)
	}
	if g.EntityType() != TypeGivenName {
		t.Fatalf("EntityType(): expected %q, got %q", TypeGivenName, g.EntityType())
	}
}
