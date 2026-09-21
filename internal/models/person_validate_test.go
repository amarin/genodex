package models

import "testing"

// validPerson — образец корректной персоны; тесты портят по одному полю.
func validPerson() *Person {
	return &Person{
		ID:     testID(TypePerson),
		Gender: PersonGenderFemale,
		Names: []PersonName{{
			Type:    PersonNameMain,
			Surname: TextRef{Text: "Дорожкина", Ref: testID(TypeSurname), Type: TypeSurname},
			Given:   TextRef{Text: "Акилина"},
			Prefix:  "фон",
			Since:   &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact},
		}},
		Estates:   []TextRef{{Text: "крестьяне", Ref: testID(TypeEstate), Type: TypeEstate}},
		Titles:    []TextRef{{Text: "вдова"}},
		Nicknames: []TextRef{{Text: "Акилинушка"}},
		Notes:     []TextRef{{Text: "жила в Давыдове"}},
		Sources:   []SourceLink{{CitationID: testID(TypeCitation), Reliability: ReliabilityPrimary}},
		Private:   true,
	}
}

func TestPersonValidateOK(t *testing.T) {
	if err := validPerson().Validate(); err != nil {
		t.Fatalf("образец должен быть валиден: %v", err)
	}

	// Все поля, кроме id, необязательны (people.md).
	if err := (&Person{ID: testID(TypePerson)}).Validate(); err != nil {
		t.Errorf("персона только с id: %v", err)
	}
}

func TestPersonValidateErrors(t *testing.T) {
	past := &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact}
	future := &FactDate{Year: 1900, Precision: PrecisionYear, Modifier: ModifierExact}

	tests := []struct {
		name   string
		mutate func(*Person)
		field  string
	}{
		{"пустой id", func(p *Person) { p.ID = "" }, "id"},
		{"плохой формат id", func(p *Person) { p.ID = "p-1" }, "id"},
		{"id другого типа", func(p *Person) { p.ID = testID(TypeFamily) }, "id"},
		{"плохой пол", func(p *Person) { p.Gender = "x" }, "gender"},
		{"имя без частей", func(p *Person) { p.Names[0] = PersonName{Type: PersonNameMain} }, "names[0]"},
		{"плохой тип имени", func(p *Person) { p.Names[0].Type = "nick" }, "names[0].type"},
		{"фамилия — ссылка на персону", func(p *Person) {
			p.Names[0].Surname = TextRef{Ref: testID(TypePerson), Type: TypePerson}
		}, "names[0].surname.type"},
		{"имя — ссылка на фамилию", func(p *Person) {
			p.Names[0].Given = TextRef{Ref: testID(TypeSurname), Type: TypeSurname}
		}, "names[0].given.type"},
		{"отчество — битый текст", func(p *Person) { p.Names[0].Patronymic = TextRef{Text: "  "} }, "names[0].patronymic.text"},
		{"плохое начало имени", func(p *Person) { p.Names[0].Since = &FactDate{} }, "names[0].since.precision"},
		{"начало имени позже конца", func(p *Person) { p.Names[0].Since, p.Names[0].Until = future, past }, "names[0].since"},
		{"второе имя пустое", func(p *Person) { p.Names = append(p.Names, PersonName{}) }, "names[1]"},
		{"сословие — не estate", func(p *Person) {
			p.Estates[0] = TextRef{Ref: testID(TypeTitle), Type: TypeTitle}
		}, "estates[0].type"},
		{"титул — не title", func(p *Person) {
			p.Titles[0] = TextRef{Ref: testID(TypeEstate), Type: TypeEstate}
		}, "titles[0].type"},
		{"пустое прозвище", func(p *Person) { p.Nicknames = append(p.Nicknames, TextRef{}) }, "nicknames[1].text"},
		{"битая ссылка в заметке", func(p *Person) { p.Notes[0] = TextRef{Ref: "x", Type: TypeNote} }, "notes[0].ref"},
		{"плохая цитата", func(p *Person) { p.Sources[0].CitationID = "c-1" }, "sources[0].citation_id"},
		{"плохая достоверность", func(p *Person) { p.Sources[0].Reliability = "maybe" }, "sources[0].reliability"},
	}
	for _, tt := range tests {
		p := validPerson()
		tt.mutate(p)
		wantInvalid(t, tt.name, p.Validate(), TypePerson, tt.field)
	}
}

func TestPersonNameValidateStandalone(t *testing.T) {
	ok := PersonName{Given: TextRef{Text: "Акилина"}}
	if err := ok.Validate(); err != nil {
		t.Errorf("имя только с именем: %v", err)
	}
	// Отчество без имени и фамилии — тоже имя (части необязательны, но хотя бы одна нужна).
	if err := (PersonName{Patronymic: TextRef{Text: "Ивановна"}}).Validate(); err != nil {
		t.Errorf("имя только с отчеством: %v", err)
	}

	wantInvalid(t, "префикс и суффикс — не имя",
		PersonName{Prefix: "фон", Suffix: "мл."}.Validate(), "", "")
	wantInvalid(t, "плохой тип", PersonName{Type: "x", Given: TextRef{Text: "А"}}.Validate(), "", "type")
}
