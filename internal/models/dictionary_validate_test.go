package models

import "testing"

var dictionaryTypes = []Type{TypeSurname, TypeGivenName, TypePatronymic, TypeEstate, TypeTitle}

// Общие правила словарной записи проверяются на validateDictionary для каждого
// из пяти словарей.
func TestValidateDictionaryRules(t *testing.T) {
	for _, typ := range dictionaryTypes {
		id := testID(typ)
		variants := []TextRef{{Text: "вариант"}, {Text: "другой", Ref: testID(typ), Type: typ}}
		items := []TextRef{{Text: "Иван"}, {Text: "Пётр", Ref: testID(TypePerson), Type: TypePerson}}
		notes := []TextRef{{Text: "заметка"}}

		if e := validateDictionary(typ, id, "образец", variants, items, notes); e != nil {
			t.Errorf("%s: образец должен быть валиден: %v", typ, e)
		}
		if e := validateDictionary(typ, id, "без вариантов", nil, nil, nil); e != nil {
			t.Errorf("%s: запись без списков: %v", typ, e)
		}

		other := TypeSurname
		if typ == TypeSurname {
			other = TypeGivenName
		}

		tests := []struct {
			name      string
			id        ID
			canonical string
			variants  []TextRef
			items     []TextRef
			notes     []TextRef
			field     string
		}{
			{"пустой id", "", "x", nil, nil, nil, "id"},
			{"id другого типа", testID(TypeEvent), "x", nil, nil, nil, "id"},
			{"пустая каноническая", id, "", nil, nil, nil, "canonical"},
			{"каноническая из пробелов", id, "  ", nil, nil, nil, "canonical"},
			{"пустой вариант", id, "x", []TextRef{{Text: "a"}, {}}, nil, nil, "variants[1].text"},
			{"вариант другого типа", id, "x", []TextRef{{Ref: testID(other), Type: other}}, nil, nil, "variants[0].type"},
			{"пустой носитель", id, "x", nil, []TextRef{{}}, nil, "items[0].text"},
			{"пустая заметка", id, "x", nil, nil, []TextRef{{Text: "a"}, {}}, "notes[1].text"},
		}
		for _, tt := range tests {
			e := validateDictionary(typ, tt.id, tt.canonical, tt.variants, tt.items, tt.notes)
			wantInvalid(t, string(typ)+": "+tt.name, finish(typ, e), typ, tt.field)
		}
	}
}

// Публичные Validate передают в общие правила правильный тип сущности.
func TestDictionaryPublicValidate(t *testing.T) {
	type entry struct {
		typ   Type
		build func(id ID) interface{ Validate() error }
	}
	entries := []entry{
		{TypeSurname, func(id ID) interface{ Validate() error } { return &Surname{ID: id, Canonical: "Дорожкин"} }},
		{TypeGivenName, func(id ID) interface{ Validate() error } {
			return &GivenName{ID: id, Canonical: "Иван", Gender: MaleName}
		}},
		{TypePatronymic, func(id ID) interface{ Validate() error } { return &Patronymic{ID: id, Canonical: "Иванович"} }},
		{TypeEstate, func(id ID) interface{ Validate() error } { return &Estate{ID: id, Canonical: "крестьяне"} }},
		{TypeTitle, func(id ID) interface{ Validate() error } { return &Title{ID: id, Canonical: "вдова"} }},
	}
	for _, e := range entries {
		if err := e.build(testID(e.typ)).Validate(); err != nil {
			t.Errorf("%s: %v", e.typ, err)
		}

		wrong := TypeEvent
		wantInvalid(t, string(e.typ)+": чужой id", e.build(testID(wrong)).Validate(), e.typ, "id")
	}
}

func TestGivenNameValidateGender(t *testing.T) {
	base := func() *GivenName {
		return &GivenName{ID: testID(TypeGivenName), Canonical: "Женя", Gender: NeutralName}
	}
	if err := base().Validate(); err != nil {
		t.Errorf("нейтральное имя: %v", err)
	}
	for _, g := range []NameGender{"", "other"} {
		n := base()
		n.Gender = g
		wantInvalid(t, "пол "+string(g), n.Validate(), TypeGivenName, "gender")
	}
}
