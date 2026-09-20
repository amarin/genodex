package models

import "testing"

func validRelation() *Relation {
	return &Relation{
		ID:      testID(TypeRelation),
		Kind:    RelationKindBlood,
		PersonA: testID(TypePerson),
		PersonB: personBID(),
		Since:   &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact},
		Sources: []SourceLink{{CitationID: testID(TypeCitation)}},
		Notes:   []TextRef{{Text: "по метрике"}},
	}
}

// personBID — другой валидный идентификатор персоны (иное тело ULID).
func personBID() ID {
	id, err := BuildID(TypePerson, "01J8X4T0K2M9Q7R5V3B6N8C1D5")
	if err != nil {
		panic(err)
	}

	return id
}

func TestRelationValidateOK(t *testing.T) {
	if err := validRelation().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	assoc := validRelation()
	assoc.Kind, assoc.RelType = RelationKindAssociate, "godparent"
	if err := assoc.Validate(); err != nil {
		t.Errorf("associate с rel_type: %v", err)
	}

	for _, kind := range []RelationKind{RelationKindMarriage, RelationKindAdoption} {
		r := validRelation()
		r.Kind = kind
		if err := r.Validate(); err != nil {
			t.Errorf("kind=%s: %v", kind, err)
		}
	}
}

func TestRelationValidateErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Relation)
		field  string
	}{
		{"плохой id", func(r *Relation) { r.ID = "r-1" }, "id"},
		{"пустой kind", func(r *Relation) { r.Kind = "" }, "kind"},
		{"плохой kind", func(r *Relation) { r.Kind = "friend" }, "kind"},
		{"associate без rel_type", func(r *Relation) { r.Kind = RelationKindAssociate }, "rel_type"},
		{"associate с плохим rel_type", func(r *Relation) { r.Kind, r.RelType = RelationKindAssociate, "Godparent" }, "rel_type"},
		{"rel_type у blood", func(r *Relation) { r.RelType = "neighbor" }, "rel_type"},
		{"person_a не персона", func(r *Relation) { r.PersonA = testID(TypeFamily) }, "person_a"},
		{"person_b не персона", func(r *Relation) { r.PersonB = testID(TypeFamily) }, "person_b"},
		{"пустой person_b", func(r *Relation) { r.PersonB = "" }, "person_b"},
		{"связь с самой собой", func(r *Relation) { r.PersonB = r.PersonA }, "person_b"},
		{"начало позже конца", func(r *Relation) {
			r.Since = &FactDate{Year: 1900, Precision: PrecisionYear, Modifier: ModifierExact}
			r.Until = &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact}
		}, "since"},
		{"плохая цитата", func(r *Relation) { r.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
		{"пустая заметка", func(r *Relation) { r.Notes = append(r.Notes, TextRef{}) }, "notes[1].text"},
	}
	for _, tt := range tests {
		r := validRelation()
		tt.mutate(r)
		wantInvalid(t, tt.name, r.Validate(), TypeRelation, tt.field)
	}
}

func validResidence() *Residence {
	return &Residence{
		ID:       testID(TypeResidence),
		PersonID: testID(TypePerson),
		PlaceID:  testID(TypeAdministrativeDivision),
		Since:    &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact},
		Until:    &FactDate{Year: 1900, Precision: PrecisionYear, Modifier: ModifierExact},
		Sources:  []SourceLink{{CitationID: testID(TypeCitation)}},
		Note:     "дом у церкви",
	}
}

func TestResidenceValidate(t *testing.T) {
	if err := validResidence().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Residence)
		field  string
	}{
		{"плохой id", func(r *Residence) { r.ID = "" }, "id"},
		{"person_id не персона", func(r *Residence) { r.PersonID = testID(TypeEvent) }, "person_id"},
		{"пустой place_id", func(r *Residence) { r.PlaceID = "" }, "place_id"},
		{"place_id не деление", func(r *Residence) { r.PlaceID = testID(TypeChurch) }, "place_id"},
		{"плохое начало", func(r *Residence) { r.Since = &FactDate{Year: 0, Precision: PrecisionYear, Modifier: ModifierExact} }, "since.year"},
		{"начало позже конца", func(r *Residence) { r.Since, r.Until = r.Until, r.Since }, "since"},
		{"плохая цитата", func(r *Residence) { r.Sources[0].CitationID = "" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		r := validResidence()
		tt.mutate(r)
		wantInvalid(t, tt.name, r.Validate(), TypeResidence, tt.field)
	}
}

func validFamily() *Family {
	return &Family{
		ID:   testID(TypeFamily),
		Name: "Дорожкины",
		Members: []TextRef{
			{Text: "Иван", Ref: testID(TypePerson), Type: TypePerson},
			{Text: "какая-то Мария"},
		},
		Notes:   []TextRef{{Text: "из Давыдова"}},
		Sources: []SourceLink{{CitationID: testID(TypeCitation)}},
	}
}

func TestFamilyValidate(t *testing.T) {
	if err := validFamily().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Family)
		field  string
	}{
		{"плохой id", func(f *Family) { f.ID = "F" }, "id"},
		{"пустое имя", func(f *Family) { f.Name = "" }, "name"},
		{"имя из пробелов", func(f *Family) { f.Name = "   " }, "name"},
		{"пустой член рода", func(f *Family) { f.Members = append(f.Members, TextRef{}) }, "members[2].text"},
		{"член рода — не персона", func(f *Family) { f.Members[0] = TextRef{Ref: testID(TypeFamily), Type: TypeFamily} }, "members[0].type"},
		{"член рода — битая ссылка", func(f *Family) { f.Members[0].Ref = "x" }, "members[0].ref"},
		{"пустая заметка", func(f *Family) { f.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(f *Family) { f.Sources[0].CitationID = "c" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		f := validFamily()
		tt.mutate(f)
		wantInvalid(t, tt.name, f.Validate(), TypeFamily, tt.field)
	}
}
