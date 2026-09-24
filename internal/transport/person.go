package transport

import "github.com/amarin/genodex/internal/models"

// Person — контракт персоны (GET /api/people, MCP-тул person_list) — ядро
// графа генеалогии. Sources редактируется с рождения контракта (сущность
// заведена уже после подпроекта 5, см. internal/transport/source_link.go).
//
// Поиск (person_search, GET /api/people/search) ищет по началу фамилии,
// имени или отчества из ЛЮБОГО элемента Names (не только основного) — все
// имена персоны индексируются под единым полем "name"
// (internal/store/sqlstore/person.go:personTerms). Estates/Titles/
// Nicknames/Notes поиском не охватываются.
type Person struct {
	ID        models.ID    `json:"id"`
	Gender    string       `json:"gender,omitempty"`
	Names     []PersonName `json:"names"`
	Estates   []TextRef    `json:"estates"`
	Titles    []TextRef    `json:"titles"`
	Nicknames []TextRef    `json:"nicknames"`
	Notes     []TextRef    `json:"notes"`
	Sources   []SourceLink `json:"sources"`
	Private   bool         `json:"private"`
}

// PersonFromModel конвертирует запись в контракт.
func PersonFromModel(p models.Person) Person {
	return Person{
		ID:        p.ID,
		Gender:    string(p.Gender),
		Names:     PersonNamesFromModel(p.Names),
		Estates:   TextRefsFromModel(p.Estates),
		Titles:    TextRefsFromModel(p.Titles),
		Nicknames: TextRefsFromModel(p.Nicknames),
		Notes:     TextRefsFromModel(p.Notes),
		Sources:   SourceLinksFromModel(p.Sources),
		Private:   p.Private,
	}
}

// PeopleFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func PeopleFromModels(ps []models.Person) []Person {
	out := make([]Person, 0, len(ps))
	for _, p := range ps {
		out = append(out, PersonFromModel(p))
	}

	return out
}
