package models

import "testing"

func TestTextRefValidate(t *testing.T) {
	person := testID(TypePerson)

	valid := map[string]TextRef{
		"только текст":      {Text: "Давыдово"},
		"ссылка с текстом":  {Text: "Иван", Ref: person, Type: TypePerson},
		"ссылка без текста": {Ref: person, Type: TypePerson},
	}
	for name, r := range valid {
		if err := r.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := []struct {
		name  string
		r     TextRef
		field string
	}{
		{"пустой", TextRef{}, "text"},
		{"только пробелы", TextRef{Text: "  "}, "text"},
		{"тип без ссылки", TextRef{Text: "x", Type: TypePerson}, "type"},
		{"ссылка без типа", TextRef{Text: "x", Ref: person}, "type"},
		{"неизвестный тип", TextRef{Ref: person, Type: "nonsense"}, "type"},
		{"формат ссылки", TextRef{Ref: "p-1", Type: TypePerson}, "ref"},
		{"префикс не того типа", TextRef{Ref: person, Type: TypeFamily}, "ref"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.r.Validate(), "", tt.field)
	}
}

func TestTextRefValidateAs(t *testing.T) {
	surname := testID(TypeSurname)
	r := TextRef{Text: "Иванов", Ref: surname, Type: TypeSurname}
	if e := r.validateAs(TypeSurname); e != nil {
		t.Errorf("ожидаемый тип: %v", e)
	}
	if e := r.validateAs(""); e != nil {
		t.Errorf("любой тип: %v", e)
	}
	wantInvalid(t, "другой тип", finish("", r.validateAs(TypeGivenName)), "", "type")

	// Ожидаемый тип не требует ссылки: обычный текст допустим.
	if e := (TextRef{Text: "Иван"}).validateAs(TypeGivenName); e != nil {
		t.Errorf("текст без ссылки: %v", e)
	}
}

func TestOptionalAndListTextRefs(t *testing.T) {
	if e := validateOptionalTextRef(TextRef{}, TypeSurname); e != nil {
		t.Errorf("пустая необязательная часть: %v", e)
	}
	wantInvalid(t, "необязательная, но битая",
		finish("", validateOptionalTextRef(TextRef{Text: " "}, "")), "", "text")

	refs := []TextRef{{Text: "a"}, {Text: "b"}, {}}
	wantInvalid(t, "список", finish("", validateTextRefs("notes", refs, "")), "", "notes[2].text")
	if e := validateTextRefs("notes", refs[:2], ""); e != nil {
		t.Errorf("корректный список: %v", e)
	}
	if e := validateTextRefs("notes", nil, ""); e != nil {
		t.Errorf("пустой список: %v", e)
	}
}

func TestTextRefIsZero(t *testing.T) {
	if !(TextRef{}).isZero() {
		t.Error("пустой TextRef должен быть zero")
	}
	for _, r := range []TextRef{{Text: "x"}, {Ref: "x"}, {Type: TypePerson}} {
		if r.isZero() {
			t.Errorf("%+v не должен быть zero", r)
		}
	}
}

func TestSourceLinkValidate(t *testing.T) {
	cit := testID(TypeCitation)

	valid := map[string]SourceLink{
		"минимум":          {CitationID: cit},
		"с достоверностью": {CitationID: cit, Reliability: ReliabilityPrimary, Role: "имя", Note: "л. 12"},
		// TargetType/TargetID выставляет адаптер из владельца — не проверяются.
		"цели игнорируются": {CitationID: cit, TargetType: "nonsense", TargetID: "мусор"},
	}
	for name, l := range valid {
		if err := l.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := []struct {
		name  string
		l     SourceLink
		field string
	}{
		{"пустая цитата", SourceLink{}, "citation_id"},
		{"плохой формат цитаты", SourceLink{CitationID: "c-1"}, "citation_id"},
		{"цитата не того типа", SourceLink{CitationID: testID(TypeSource)}, "citation_id"},
		{"плохая достоверность", SourceLink{CitationID: cit, Reliability: "maybe"}, "reliability"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.l.Validate(), "", tt.field)
	}

	links := []SourceLink{{CitationID: cit}, {CitationID: "bad"}}
	wantInvalid(t, "список", finish("", validateSourceLinks("sources", links)), "", "sources[1].citation_id")
	if e := validateSourceLinks("sources", links[:1]); e != nil {
		t.Errorf("корректный список: %v", e)
	}
}
