package models

import "strings"

// Validate проверяет именование с периодом: наименование обязательно; начало и
// конец — строки в формате ParseFactDate (пустое допустимо, период открытый),
// проходят FactDate.Validate, начало не позже конца.
func (n NamedPeriod) Validate() error {
	return finish("", n.validate())
}

func (n NamedPeriod) validate() *ValidationError {
	if e := requireText("text", n.Text); e != nil {
		return e
	}

	since, e := parseNamedDate("since", n.Since)
	if e != nil {
		return e
	}
	until, e := parseNamedDate("until", n.Until)
	if e != nil {
		return e
	}

	return validatePeriod(since, until)
}

// parseNamedDate разбирает необязательную дату периода; пустая строка — nil.
func parseNamedDate(field, s string) (*FactDate, *ValidationError) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	d, err := ParseFactDate(s)
	if err != nil {
		return nil, fieldErr(field, "недопустимая дата %q: %v", s, err)
	}

	return &d, nil
}

// validateRenames проверяет список исторических наименований.
func validateRenames(field string, renames []NamedPeriod) *ValidationError {
	for i, n := range renames {
		if e := n.validate(); e != nil {
			return e.within(indexed(field, i))
		}
	}

	return nil
}
