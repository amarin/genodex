package models

import (
	"errors"
	"testing"
)

// testID возвращает валидный идентификатор для типа сущности.
func testID(t Type) ID {
	id, err := BuildID(t, validBody)
	if err != nil {
		panic(err)
	}

	return id
}

// wantInvalid проверяет, что err — *ValidationError с ожидаемой сущностью и полем.
func wantInvalid(t *testing.T, name string, err error, entity Type, field string) {
	t.Helper()

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("%s: err = %v, want *ValidationError", name, err)

		return
	}
	if ve.Entity != entity || ve.Field != field {
		t.Errorf("%s: получено %s / %q (%s), want %s / %q", name, ve.Entity, ve.Field, ve.Reason, entity, field)
	}
}

func TestValidationErrorMessage(t *testing.T) {
	tests := []struct {
		e    ValidationError
		want string
	}{
		{ValidationError{Entity: TypePerson, Field: "names[0].surname", Reason: "пусто"}, "person: names[0].surname: пусто"},
		{ValidationError{Field: "year", Reason: "вне диапазона"}, "year: вне диапазона"},
		{ValidationError{Entity: TypeFamily, Reason: "нет имени"}, "family: нет имени"},
	}
	for _, tt := range tests {
		if got := tt.e.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

func TestWithinBuildsFieldPath(t *testing.T) {
	tests := []struct {
		field, prefix, want string
	}{
		{"", "since", "since"},
		{"year", "since", "since.year"},
		{"[0]", "names", "names[0]"},
		{"surname.type", "names[0]", "names[0].surname.type"},
	}
	for _, tt := range tests {
		e := &ValidationError{Field: tt.field, Reason: "x"}
		if got := e.within(tt.prefix).Field; got != tt.want {
			t.Errorf("within(%q) на %q = %q, want %q", tt.prefix, tt.field, got, tt.want)
		}
	}

	var nilErr *ValidationError
	if nilErr.within("x") != nil {
		t.Error("within на nil должен оставаться nil")
	}
	if got := indexed("names", 2); got != "names[2]" {
		t.Errorf("indexed = %q", got)
	}
}

func TestFinishNoTypedNil(t *testing.T) {
	if err := finish(TypePerson, nil); err != nil {
		t.Errorf("finish(nil) = %v, want nil-интерфейс", err)
	}

	err := finish(TypePerson, fieldErr("id", "плохой %s", "формат"))
	wantInvalid(t, "finish", err, TypePerson, "id")
}

func TestClosedEnumsValid(t *testing.T) {
	enums := []struct {
		name  string
		valid []string
		check func(string) bool
	}{
		{"PersonGender", []string{"unknown", "male", "female"}, func(s string) bool { return PersonGender(s).Valid() }},
		{"NameGender", []string{"male", "female", "neutral"}, func(s string) bool { return NameGender(s).Valid() }},
		{"PersonNameType", []string{"main", "birth", "married", "changed", "pseudonym"}, func(s string) bool { return PersonNameType(s).Valid() }},
		{"RelationKind", []string{"blood", "marriage", "adoption", "associate"}, func(s string) bool { return RelationKind(s).Valid() }},
		{"SourceKind", []string{"archival-scan", "transcription", "document", "audio", "photo", "memory", "external"}, func(s string) bool { return SourceKind(s).Valid() }},
		{"AttachmentKind", []string{"scan", "document", "audio", "photo"}, func(s string) bool { return AttachmentKind(s).Valid() }},
		{"Reliability", []string{"primary", "contemporary", "memory", "indirect", "unknown"}, func(s string) bool { return Reliability(s).Valid() }},
		{"AnchorKind", []string{"archive", "file", "url"}, func(s string) bool { return AnchorKind(s).Valid() }},
		{"FactPrecision", []string{"unknown", "year", "month", "day"}, func(s string) bool { return FactPrecision(s).Valid() }},
		{"FactModifier", []string{"exact", "approx", "before", "after", "between"}, func(s string) bool { return FactModifier(s).Valid() }},
		{"FactCalendar", []string{"gregorian", "julian", "unknown"}, func(s string) bool { return FactCalendar(s).Valid() }},
		{"AdminDivisionType", []string{"namestnichestvo", "provintsiya", "guberniya", "uezd", "stan", "volost", "oblast", "okrug", "respublika", "krai", "rayon", "selsovet", "other", "gorod", "poselok", "sloboda", "selo", "seltso", "derevnya", "hutor", "pogost", "stanitsa", "mestechko"}, func(s string) bool { return AdminDivisionType(s).Valid() }},
	}
	for _, e := range enums {
		for _, v := range e.valid {
			if !e.check(v) {
				t.Errorf("%s(%q) должен быть допустим", e.name, v)
			}
		}
		for _, bad := range []string{"", "bogus", "MALE", " male"} {
			if e.check(bad) {
				t.Errorf("%s(%q) не должен быть допустим", e.name, bad)
			}
		}
	}
}

func TestTypeValid(t *testing.T) {
	for _, typ := range AllTypes() {
		if !typ.Valid() {
			t.Errorf("Type(%q) должен быть допустим", typ)
		}
	}
	for _, bad := range []Type{"", "nonsense", "Person"} {
		if bad.Valid() {
			t.Errorf("Type(%q) не должен быть допустим", bad)
		}
	}
}

func TestValidOpenEnum(t *testing.T) {
	for _, ok := range []string{"neighbor", "god-parent", "a", "witness_2", "x9"} {
		if !validOpenEnum(ok) {
			t.Errorf("validOpenEnum(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "Neighbor", "2fast", "-x", "_x", "a b", "друг", "a.b", "a\x00"} {
		if validOpenEnum(bad) {
			t.Errorf("validOpenEnum(%q) = true, want false", bad)
		}
	}
}

func TestClosedEnumsRejectForeignValues(t *testing.T) {
	// Значения, допустимые для другого enum'а, здесь недопустимы.
	checks := []struct {
		name  string
		valid bool
	}{
		{"NameGender(unknown)", NameGender("unknown").Valid()},
		{"PersonGender(neutral)", PersonGender("neutral").Valid()},
		{"Reliability(male)", Reliability("male").Valid()},
		{"SourceKind(scan)", SourceKind("scan").Valid()},
		{"AttachmentKind(memory)", AttachmentKind("memory").Valid()},
		{"FactPrecision(exact)", FactPrecision("exact").Valid()},
		{"FactModifier(year)", FactModifier("year").Valid()},
		{"AnchorKind(archival-scan)", AnchorKind("archival-scan").Valid()},
		{"RelationKind(friend)", RelationKind("friend").Valid()},
		{"FactCalendar(male)", FactCalendar("male").Valid()},
	}
	for _, c := range checks {
		if c.valid {
			t.Errorf("%s не должен быть допустим", c.name)
		}
	}
}

func TestValidationErrorUnwrapsID(t *testing.T) {
	// Разворачивание до ErrInvalidID и сохранение *ValidationError с теми же
	// сущностью и полем.
	cases := []struct {
		name   string
		err    error
		entity Type
		field  string
	}{
		{"id персоны", (&Person{ID: "garbage"}).Validate(), TypePerson, "id"},
		{"citation_id в персоне",
			(&Person{ID: testID(TypePerson), Sources: []SourceLink{{CitationID: "c"}}}).Validate(),
			TypePerson, "sources[0].citation_id"},
		{"ref в TextRef", TextRef{Ref: "x", Type: TypeNote}.Validate(), "", "ref"},
	}
	for _, c := range cases {
		if !errors.Is(c.err, ErrInvalidID) {
			t.Errorf("%s: errors.Is(ErrInvalidID) = false, err = %v", c.name, c.err)
		}
		wantInvalid(t, c.name, c.err, c.entity, c.field)
	}

	if (&ValidationError{Reason: "x"}).Unwrap() != nil {
		t.Error("Unwrap без Err должен возвращать nil")
	}
}
