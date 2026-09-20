package models

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

const validBody = "01J8X4T0K2M9Q7R5V3B6N8C1D4"

func TestIDPrefixTable(t *testing.T) {
	types := AllTypes()
	if len(types) != 21 {
		t.Fatalf("AllTypes: %d типов, want 21", len(types))
	}

	seen := map[string]Type{}
	for _, typ := range types {
		p := typ.IDPrefix()
		if p == "" || len(p) > 2 || strings.ToUpper(p) != p {
			t.Errorf("%s: префикс %q должен быть 1–2 заглавными буквами", typ, p)
		}
		if other, dup := seen[p]; dup {
			t.Errorf("префикс %q у %s и %s", p, other, typ)
		}
		seen[p] = typ

		back, ok := TypeByIDPrefix(p)
		if !ok || back != typ {
			t.Errorf("TypeByIDPrefix(%q) = %q, %v; want %q", p, back, ok, typ)
		}
	}
}

// Префиксы, совпадающие с GEDCOM (INDI, FAM, SOUR, REPO, NOTE, OBJE).
func TestIDPrefixGEDCOM(t *testing.T) {
	want := map[Type]string{
		TypePerson:     "I",
		TypeFamily:     "F",
		TypeSource:     "S",
		TypeRepository: "R",
		TypeNote:       "N",
		TypeAttachment: "O",
	}
	for typ, p := range want {
		if got := typ.IDPrefix(); got != p {
			t.Errorf("%s: префикс %q, want %q", typ, got, p)
		}
	}
}

func TestIDPrefixUnknown(t *testing.T) {
	if got := Type("nonsense").IDPrefix(); got != "" {
		t.Errorf("IDPrefix неизвестного типа = %q, want пусто", got)
	}
	if _, ok := TypeByIDPrefix("ZZ"); ok {
		t.Error("TypeByIDPrefix(ZZ) не должен находить тип")
	}
}

// Каждая константа типа Type из type.go должна быть в таблице префиксов: тест
// разбирает исходник, чтобы новый тип не появился без префикса.
func TestAllTypesCoverTypeConstants(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "type.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	declared := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		if ident, ok := spec.Type.(*ast.Ident); !ok || ident.Name != "Type" {
			return true
		}
		for _, v := range spec.Values {
			if lit, ok := v.(*ast.BasicLit); ok {
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				declared[value] = true
			}
		}

		return true
	})

	inTable := map[string]bool{}
	for _, typ := range AllTypes() {
		inTable[string(typ)] = true
	}
	for v := range declared {
		if !inTable[v] {
			t.Errorf("тип %q объявлен в type.go, но не имеет префикса в таблице", v)
		}
	}
	for v := range inTable {
		if !declared[v] {
			t.Errorf("тип %q есть в таблице префиксов, но не объявлен в type.go", v)
		}
	}
	if len(declared) == 0 {
		t.Error("в type.go не найдено ни одной константы Type")
	}
}

func TestParseID(t *testing.T) {
	valid := map[string]Type{
		"I-" + validBody:               TypePerson,
		"F-" + validBody:               TypeFamily,
		"RL-" + validBody:              TypeRelation,
		"AD-" + validBody:              TypeAdministrativeDivision,
		"DC-" + validBody:              TypeArchiveDocument,
		"I-00000000000000000000000000": TypePerson,
		"I-7ZZZZZZZZZZZZZZZZZZZZZZZZZ": TypePerson,
	}
	for in, want := range valid {
		got, err := ParseID(ID(in))
		if err != nil || got != want {
			t.Errorf("ParseID(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	invalid := []string{
		"",
		"I",
		"I-",
		"-" + validBody,
		"I" + validBody,                   // нет разделителя
		"X-" + validBody,                  // неизвестный префикс
		"i-" + validBody,                  // префикс в нижнем регистре
		"I-" + strings.ToLower(validBody), // тело в нижнем регистре
		"I-" + validBody[:25],             // короче 26
		"I-" + validBody + "0",            // длиннее 26
		"I-" + validBody[:25] + "I",       // запрещённая буква I
		"I-" + validBody[:25] + "L",       // запрещённая буква L
		"I-" + validBody[:25] + "O",       // запрещённая буква O
		"I-" + validBody[:25] + "U",       // запрещённая буква U
		"I-8" + validBody[1:],             // переполнение 48 бит времени
		"I-" + validBody[:10] + "-" + validBody[11:], // лишний дефис в теле
	}
	for _, in := range invalid {
		if got, err := ParseID(ID(in)); err == nil || !errors.Is(err, ErrInvalidID) {
			t.Errorf("ParseID(%q) = %q, %v; want ErrInvalidID", in, got, err)
		}
	}
}

func TestIDValidate(t *testing.T) {
	id := ID("I-" + validBody)
	if err := id.Validate(TypePerson); err != nil {
		t.Errorf("Validate(person): %v", err)
	}
	if err := id.Validate(TypeFamily); err == nil || !errors.Is(err, ErrInvalidID) {
		t.Errorf("Validate(family) для персоны: %v, want ErrInvalidID", err)
	}
	if err := ID("").Validate(TypePerson); err == nil {
		t.Error("пустой ID должен быть отвергнут")
	}
}

func TestBuildID(t *testing.T) {
	id, err := BuildID(TypeAdministrativeDivision, validBody)
	if err != nil || id != ID("AD-"+validBody) {
		t.Fatalf("BuildID = %q, %v", id, err)
	}
	if err := id.Validate(TypeAdministrativeDivision); err != nil {
		t.Errorf("собранный ID не проходит Validate: %v", err)
	}

	if _, err := BuildID(Type("nonsense"), validBody); !errors.Is(err, ErrInvalidID) {
		t.Errorf("BuildID неизвестного типа: %v", err)
	}
	if _, err := BuildID(TypePerson, "short"); !errors.Is(err, ErrInvalidID) {
		t.Errorf("BuildID с плохим телом: %v", err)
	}
}
