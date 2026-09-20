package models

// Validate проверяет одно имя персоны отдельно от неё.
func (n PersonName) Validate() error {
	return finish("", n.validate())
}

// validate: вид имени необязателен, но если задан — константа; хотя бы одна из
// частей (фамилия, имя, отчество) заполнена; ссылки частей — на словари
// ожидаемых типов; период корректен.
func (n PersonName) validate() *ValidationError {
	if n.Type != "" && !n.Type.Valid() {
		return fieldErr("type", "недопустимый вид имени %q", n.Type)
	}

	if n.Surname.isZero() && n.Given.isZero() && n.Patronymic.isZero() {
		return fieldErr("", "нужна хотя бы одна часть имени: фамилия, имя или отчество")
	}
	if e := validateOptionalTextRef(n.Surname, TypeSurname); e != nil {
		return e.within("surname")
	}
	if e := validateOptionalTextRef(n.Given, TypeGivenName); e != nil {
		return e.within("given")
	}
	if e := validateOptionalTextRef(n.Patronymic, TypePatronymic); e != nil {
		return e.within("patronymic")
	}

	return validatePeriod(n.Since, n.Until)
}
