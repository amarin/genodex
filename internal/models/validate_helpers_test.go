package models

import "testing"

// idPtr возвращает указатель на идентификатор (для необязательных ссылок в тестах).
func idPtr(id ID) *ID { return &id }

func TestRequireText(t *testing.T) {
	if e := requireText("name", "Давыдово"); e != nil {
		t.Errorf("непустой текст: %v", e)
	}
	for _, bad := range []string{"", "   ", "\t\n"} {
		wantInvalid(t, "пустой "+bad, finish("", requireText("name", bad)), "", "name")
	}
}

func TestValidateStrings(t *testing.T) {
	if e := validateStrings("variants", nil); e != nil {
		t.Errorf("пустой список: %v", e)
	}
	if e := validateStrings("variants", []string{"Давыдово", "Давидово"}); e != nil {
		t.Errorf("корректный список: %v", e)
	}
	wantInvalid(t, "пустой элемент",
		finish("", validateStrings("variants", []string{"a", " ", ""})), "", "variants[1]")
}

func TestValidateOptionalIDs(t *testing.T) {
	person := testID(TypePerson)

	if e := validateOptionalID("repository_id", "", TypeRepository); e != nil {
		t.Errorf("пустой необязательный id: %v", e)
	}
	if e := validateOptionalID("repository_id", testID(TypeRepository), TypeRepository); e != nil {
		t.Errorf("корректный id: %v", e)
	}
	wantInvalid(t, "плохой формат", finish("", validateOptionalID("repository_id", "R-1", TypeRepository)), "", "repository_id")
	wantInvalid(t, "не тот тип", finish("", validateOptionalID("repository_id", person, TypeRepository)), "", "repository_id")

	if e := validateOptionalIDPtr("parent_id", nil, TypeNote); e != nil {
		t.Errorf("nil-указатель: %v", e)
	}
	if e := validateOptionalIDPtr("parent_id", idPtr(testID(TypeNote)), TypeNote); e != nil {
		t.Errorf("корректный указатель: %v", e)
	}
	// Указатель на пустой id — «задан, но пуст»: это ошибка.
	wantInvalid(t, "указатель на пустой", finish("", validateOptionalIDPtr("parent_id", idPtr(""), TypeNote)), "", "parent_id")
	wantInvalid(t, "указатель не того типа", finish("", validateOptionalIDPtr("parent_id", idPtr(person), TypeNote)), "", "parent_id")
}

func TestValidateNotSelf(t *testing.T) {
	self := testID(TypeNote)
	if e := validateNotSelf(self, nil); e != nil {
		t.Errorf("без родителя: %v", e)
	}
	if e := validateNotSelf(self, idPtr(personBID())); e != nil {
		t.Errorf("другой родитель: %v", e)
	}
	wantInvalid(t, "сам себе родитель", finish("", validateNotSelf(self, idPtr(self))), "", "parent_id")
}

func TestValidateOptionalTextRefPtr(t *testing.T) {
	if e := validateOptionalTextRefPtr("parish", nil, TypeParish); e != nil {
		t.Errorf("nil: %v", e)
	}
	ok := &TextRef{Text: "Никольский приход", Ref: testID(TypeParish), Type: TypeParish}
	if e := validateOptionalTextRefPtr("parish", ok, TypeParish); e != nil {
		t.Errorf("корректная ссылка: %v", e)
	}
	if e := validateOptionalTextRefPtr("parish", &TextRef{Text: "просто текст"}, TypeParish); e != nil {
		t.Errorf("текст без ссылки: %v", e)
	}
	// Указатель задан, но TextRef пуст — ошибка (в отличие от значения-части имени).
	wantInvalid(t, "пустой TextRef", finish("", validateOptionalTextRefPtr("parish", &TextRef{}, TypeParish)), "", "parish.text")
	wantInvalid(t, "не тот тип",
		finish("", validateOptionalTextRefPtr("parish", &TextRef{Ref: testID(TypeChurch), Type: TypeChurch}, TypeParish)), "", "parish.type")
}

func TestPlaceRefValidate(t *testing.T) {
	for _, typ := range []Type{TypeAdministrativeDivision, TypeChurch, TypeParish} {
		p := PlaceRef{Text: "место", Ref: testID(typ), Type: typ}
		if err := p.Validate(); err != nil {
			t.Errorf("ссылка на %s: %v", typ, err)
		}
	}
	if err := (PlaceRef{Text: "Давыдово (нет сущности)"}).Validate(); err != nil {
		t.Errorf("свободный текст: %v", err)
	}

	invalid := []struct {
		name  string
		p     PlaceRef
		field string
	}{
		{"пустое", PlaceRef{}, "text"},
		{"ссылка на персону", PlaceRef{Ref: testID(TypePerson), Type: TypePerson}, "type"},
		{"тип без ссылки", PlaceRef{Text: "x", Type: TypeParish}, "type"},
		{"плохой формат ссылки", PlaceRef{Ref: "AD-1", Type: TypeAdministrativeDivision}, "ref"},
		{"префикс не того типа", PlaceRef{Ref: testID(TypeChurch), Type: TypeParish}, "ref"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.p.Validate(), "", tt.field)
	}
}

func TestNamedPeriodValidate(t *testing.T) {
	valid := map[string]NamedPeriod{
		"с периодом":      {Text: "Петроград", Since: "1914", Until: "1924"},
		"открытое начало": {Text: "Ленинград", Until: "1991-09-06"},
		"открытый конец":  {Text: "Санкт-Петербург", Since: "1991-09-06"},
		"без периода":     {Text: "Питер"},
		"с формулировкой": {Text: "Санкт-Петербург", Since: "около 1703", Until: "между 1914 и 1915"},
		"юлианский":       {Text: "Петроград", Since: "1914-08-18 ст. ст."},
	}
	for name, n := range valid {
		if err := n.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := []struct {
		name  string
		n     NamedPeriod
		field string
	}{
		{"пустое наименование", NamedPeriod{Since: "1914"}, "text"},
		{"наименование из пробелов", NamedPeriod{Text: " "}, "text"},
		{"мусор в начале", NamedPeriod{Text: "x", Since: "когда-то"}, "since"},
		{"мусор в конце", NamedPeriod{Text: "x", Until: "1914-13"}, "until"},
		{"31 апреля", NamedPeriod{Text: "x", Since: "1914-04-31"}, "since.day"},
		{"31 апреля в конце", NamedPeriod{Text: "x", Until: "1914-04-31"}, "until.day"},
		{"начало позже конца", NamedPeriod{Text: "x", Since: "1924", Until: "1914"}, "since"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.n.Validate(), "", tt.field)
	}

	renames := []NamedPeriod{{Text: "Петроград", Since: "1914"}, {Text: "", Since: "1924"}}
	wantInvalid(t, "список переименований", finish("", validateRenames("renames", renames)), "", "renames[1].text")
	wantInvalid(t, "плохая дата в переименовании", finish("", validateRenames("renames", []NamedPeriod{{Text: "x", Since: "1914-04-31"}})), "", "renames[0].since.day")
	if e := validateRenames("renames", renames[:1]); e != nil {
		t.Errorf("корректный список: %v", e)
	}
	if e := validateRenames("renames", nil); e != nil {
		t.Errorf("пустой список: %v", e)
	}
}
