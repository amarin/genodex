package models

import "testing"

func validDivision() *AdministrativeDivision {
	return &AdministrativeDivision{
		ID:         testID(TypeAdministrativeDivision),
		Name:       "Санкт-Петербург",
		Type:       AdminDivisionGorod,
		ParentID:   idPtr(divisionBID()),
		Items:      []TextRef{{Text: "Васильевский остров", Ref: divisionBID(), Type: TypeAdministrativeDivision}, {Text: "Охта"}},
		Variants:   []string{"Санктпетербург", "Питербурх"},
		Renames:    []NamedPeriod{{Text: "Петроград", Since: "1914", Until: "1924"}},
		Successors: []TextRef{{Text: "Ленинград"}},
		Since:      &FactDate{Year: 1703, Precision: PrecisionYear, Modifier: ModifierExact},
		Notes:      []TextRef{{Text: "столица"}},
		Sources:    []SourceLink{{CitationID: testID(TypeCitation)}},
	}
}

// divisionBID — другой валидный идентификатор деления (иное тело ULID).
func divisionBID() ID {
	id, err := BuildID(TypeAdministrativeDivision, "01J8X4T0K2M9Q7R5V3B6N8C1D5")
	if err != nil {
		panic(err)
	}

	return id
}

func TestAdministrativeDivisionValidate(t *testing.T) {
	if err := validDivision().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Корень иерархии: без родителя и без необязательных списков.
	root := &AdministrativeDivision{ID: testID(TypeAdministrativeDivision), Name: "Российская империя", Type: AdminDivisionOther}
	if err := root.Validate(); err != nil {
		t.Errorf("минимальное деление: %v", err)
	}

	past := &FactDate{Year: 1703, Precision: PrecisionYear, Modifier: ModifierExact}
	future := &FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact}

	tests := []struct {
		name   string
		mutate func(*AdministrativeDivision)
		field  string
	}{
		{"плохой id", func(a *AdministrativeDivision) { a.ID = "ad-1" }, "id"},
		{"id другого типа", func(a *AdministrativeDivision) { a.ID = testID(TypeChurch) }, "id"},
		{"пустое название", func(a *AdministrativeDivision) { a.Name = "" }, "name"},
		{"название из пробелов", func(a *AdministrativeDivision) { a.Name = "  " }, "name"},
		{"пустой тип", func(a *AdministrativeDivision) { a.Type = "" }, "type"},
		{"неизвестный тип", func(a *AdministrativeDivision) { a.Type = "kray" }, "type"},
		{"родитель не деление", func(a *AdministrativeDivision) { a.ParentID = idPtr(testID(TypeParish)) }, "parent_id"},
		{"указатель на пустой родитель", func(a *AdministrativeDivision) { a.ParentID = idPtr("") }, "parent_id"},
		{"родитель — сам себе", func(a *AdministrativeDivision) { a.ParentID = idPtr(a.ID) }, "parent_id"},
		{"составляющая — не деление", func(a *AdministrativeDivision) {
			a.Items[0] = TextRef{Ref: testID(TypeParish), Type: TypeParish}
		}, "items[0].type"},
		{"пустая составляющая", func(a *AdministrativeDivision) { a.Items = append(a.Items, TextRef{}) }, "items[2].text"},
		{"пустой вариант", func(a *AdministrativeDivision) { a.Variants = append(a.Variants, " ") }, "variants[2]"},
		{"переименование без имени", func(a *AdministrativeDivision) { a.Renames[0].Text = "" }, "renames[0].text"},
		{"переименование с плохой датой", func(a *AdministrativeDivision) { a.Renames[0].Since = "давно" }, "renames[0].since"},
		{"преемник — не деление", func(a *AdministrativeDivision) {
			a.Successors[0] = TextRef{Ref: testID(TypeChurch), Type: TypeChurch}
		}, "successors[0].type"},
		{"плохое начало", func(a *AdministrativeDivision) { a.Since = &FactDate{} }, "since.precision"},
		{"начало позже конца", func(a *AdministrativeDivision) { a.Since, a.Until = future, past }, "since"},
		{"пустая заметка", func(a *AdministrativeDivision) { a.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(a *AdministrativeDivision) { a.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		a := validDivision()
		tt.mutate(a)
		wantInvalid(t, tt.name, a.Validate(), TypeAdministrativeDivision, tt.field)
	}
}

func validChurch() *Church {
	return &Church{
		ID:          testID(TypeChurch),
		Name:        "Никольская церковь",
		Parish:      &TextRef{Text: "Никольский приход", Ref: testID(TypeParish), Type: TypeParish},
		Settlements: []TextRef{{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision}, {Text: "Никифорово"}},
		Variants:    []string{"Никольская ц."},
		Notes:       []TextRef{{Text: "деревянная"}},
		Sources:     []SourceLink{{CitationID: testID(TypeCitation)}},
	}
}

func TestChurchValidate(t *testing.T) {
	if err := validChurch().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	if err := (&Church{ID: testID(TypeChurch), Name: "ц."}).Validate(); err != nil {
		t.Errorf("минимальная церковь: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Church)
		field  string
	}{
		{"плохой id", func(c *Church) { c.ID = "" }, "id"},
		{"пустое название", func(c *Church) { c.Name = "" }, "name"},
		{"приход — не приход", func(c *Church) { c.Parish = &TextRef{Ref: testID(TypeChurch), Type: TypeChurch} }, "parish.type"},
		{"пустой приход", func(c *Church) { c.Parish = &TextRef{} }, "parish.text"},
		{"населённый пункт — не деление", func(c *Church) {
			c.Settlements[0] = TextRef{Ref: testID(TypeParish), Type: TypeParish}
		}, "settlements[0].type"},
		{"пустой вариант", func(c *Church) { c.Variants = []string{""} }, "variants[0]"},
		{"пустая заметка", func(c *Church) { c.Notes = append(c.Notes, TextRef{}) }, "notes[1].text"},
		{"плохая цитата", func(c *Church) { c.Sources[0].CitationID = "" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		c := validChurch()
		tt.mutate(c)
		wantInvalid(t, tt.name, c.Validate(), TypeChurch, tt.field)
	}
}

func validParish() *Parish {
	return &Parish{
		ID:          testID(TypeParish),
		Name:        "Никольский приход",
		Church:      &TextRef{Text: "Никольская церковь", Ref: testID(TypeChurch), Type: TypeChurch},
		Settlements: []TextRef{{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision}},
		Since:       &FactDate{Year: 1800, Precision: PrecisionYear, Modifier: ModifierExact},
		Until:       &FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact},
		Notes:       []TextRef{{Text: "упразднён"}},
		Sources:     []SourceLink{{CitationID: testID(TypeCitation)}},
	}
}

func TestParishValidate(t *testing.T) {
	if err := validParish().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Parish)
		field  string
	}{
		{"плохой id", func(p *Parish) { p.ID = testID(TypeChurch) }, "id"},
		{"пустое название", func(p *Parish) { p.Name = " " }, "name"},
		{"церковь — не церковь", func(p *Parish) { p.Church = &TextRef{Ref: testID(TypeParish), Type: TypeParish} }, "church.type"},
		{"населённый пункт — не деление", func(p *Parish) {
			p.Settlements[0] = TextRef{Ref: testID(TypeChurch), Type: TypeChurch}
		}, "settlements[0].type"},
		{"начало позже конца", func(p *Parish) { p.Since, p.Until = p.Until, p.Since }, "since"},
		{"плохой конец", func(p *Parish) { p.Until = &FactDate{Year: 1917} }, "until.precision"},
		{"пустая заметка", func(p *Parish) { p.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(p *Parish) { p.Sources[0].Reliability = "maybe" }, "sources[0].reliability"},
	}
	for _, tt := range tests {
		p := validParish()
		tt.mutate(p)
		wantInvalid(t, tt.name, p.Validate(), TypeParish, tt.field)
	}
}
