package models

import "testing"

func TestPersonFullForm(t *testing.T) {
	p := Person{
		ID:     ID("p-1"),
		Gender: PersonGenderFemale,
		Names: []PersonName{
			{
				Type:    PersonNameMain,
				Surname: TextRef{Text: "Дорожкина"},
				Given:   TextRef{Text: "Акилина"},
				Prefix:  "фон",
				// без Suffix — опционально
			},
			{
				Type:    PersonNameBirth,
				Surname: TextRef{Text: "Иванова"},
			},
		},
		Estates:   []TextRef{{Text: "крестьяне"}},
		Titles:    []TextRef{{Text: "вдова"}},
		Nicknames: []TextRef{{Text: "Акилинушка"}},
		Notes:     []TextRef{{Text: "жила в Давыдове"}},
		Sources:   []SourceLink{{CitationID: ID("c-1"), TargetType: TypePerson, TargetID: ID("p-1")}},
		Private:   true,
	}
	if p.EntityType() != TypePerson {
		t.Fatalf("EntityType() = %q, want %q", p.EntityType(), TypePerson)
	}
	if len(p.Names) != 2 {
		t.Fatalf("Names %d, want 2", len(p.Names))
	}
	if len(p.Nicknames) != 1 || !p.Private {
		t.Fatalf("nicknames/private не прочитаны")
	}
}
