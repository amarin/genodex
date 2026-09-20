package models

import "strings"

// Validate проверяет «текст-или-ссылку»: без ссылки нужен непустой текст и не
// задан тип; со ссылкой тип обязателен, известен и совпадает с префиксом Ref.
func (r TextRef) Validate() error {
	return finish("", r.validateAs(""))
}

// validateAs — Validate с ожидаемым типом ссылки; want пусто — любой известный
// тип. Ожидаемый тип не требует наличия ссылки: обычный текст допустим всегда.
func (r TextRef) validateAs(want Type) *ValidationError {
	if r.Ref == "" {
		if strings.TrimSpace(r.Text) == "" {
			return fieldErr("text", "текст обязателен, если нет ссылки")
		}
		if r.Type != "" {
			return fieldErr("type", "тип задаётся только вместе со ссылкой")
		}

		return nil
	}

	if r.Type == "" {
		return fieldErr("type", "при ссылке тип обязателен")
	}
	if !r.Type.Valid() {
		return fieldErr("type", "неизвестный тип сущности %q", r.Type)
	}
	if want != "" && r.Type != want {
		return fieldErr("type", "ожидается тип %s, получен %s", want, r.Type)
	}
	if e := idErr("ref", r.Ref, r.Type); e != nil {
		return e
	}

	return nil
}

// isZero сообщает, что TextRef не заполнен (часть имени отсутствует).
func (r TextRef) isZero() bool {
	return r.Text == "" && r.Ref == "" && r.Type == ""
}

// validateOptionalTextRef проверяет необязательное поле-TextRef: незаполненное
// допустимо, заполненное должно быть корректным.
func validateOptionalTextRef(r TextRef, want Type) *ValidationError {
	if r.isZero() {
		return nil
	}

	return r.validateAs(want)
}

// validateTextRefs проверяет список TextRef; путь ошибки — field[i].….
func validateTextRefs(field string, refs []TextRef, want Type) *ValidationError {
	for i, r := range refs {
		if e := r.validateAs(want); e != nil {
			return e.within(indexed(field, i))
		}
	}

	return nil
}
