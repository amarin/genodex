package transport

import "github.com/amarin/genodex/internal/models"

// PersonName — контракт одного имени персоны (вложенная подформа
// Person.Names, models.PersonName): вид имени + три мягкие ссылки на
// словари (Surname/GivenName/Patronymic, того же TextRef-контракта, что и
// одиночные списки TextRef в других сущностях) + служебные части имени +
// период действия (FactDate). Первая вложенная подформа-«массив объектов» в
// программе (docs/data-model/entity-write.md §3.7).
type PersonName struct {
	Type       string    `json:"type,omitempty"`
	Surname    TextRef   `json:"surname"`
	Given      TextRef   `json:"given"`
	Patronymic TextRef   `json:"patronymic"`
	Prefix     string    `json:"prefix,omitempty"`
	Suffix     string    `json:"suffix,omitempty"`
	Since      *FactDate `json:"since,omitempty"`
	Until      *FactDate `json:"until,omitempty"`
}

// PersonNameFromModel конвертирует одно имя в контракт.
func PersonNameFromModel(n models.PersonName) PersonName {
	return PersonName{
		Type:       string(n.Type),
		Surname:    TextRefFromModel(n.Surname),
		Given:      TextRefFromModel(n.Given),
		Patronymic: TextRefFromModel(n.Patronymic),
		Prefix:     n.Prefix,
		Suffix:     n.Suffix,
		Since:      FactDateFromModel(n.Since),
		Until:      FactDateFromModel(n.Until),
	}
}

// PersonNamesFromModel конвертирует список; пустой вход даёт пустой срез, а не nil.
func PersonNamesFromModel(ns []models.PersonName) []PersonName {
	out := make([]PersonName, 0, len(ns))
	for _, n := range ns {
		out = append(out, PersonNameFromModel(n))
	}

	return out
}

// Model конвертирует контракт обратно в модель.
func (n PersonName) Model() models.PersonName {
	return models.PersonName{
		Type:       models.PersonNameType(n.Type),
		Surname:    n.Surname.Model(),
		Given:      n.Given.Model(),
		Patronymic: n.Patronymic.Model(),
		Prefix:     n.Prefix,
		Suffix:     n.Suffix,
		Since:      n.Since.Model(),
		Until:      n.Until.Model(),
	}
}

// PersonNamesToModel конвертирует список контрактов в модели; пустой вход
// даёт пустой срез, а не nil.
func PersonNamesToModel(ns []PersonName) []models.PersonName {
	out := make([]models.PersonName, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.Model())
	}

	return out
}
