package models

import "strings"

// Validate проверяет заметку: вид обязателен (открытый enum); нужен заголовок
// или текст (книга-контейнер может иметь только заголовок, заметка — только
// текст); родитель — note, не совпадающая с самой заметкой (циклы длиннее
// проверяют сценарии).
func (n *Note) Validate() error {
	return finish(TypeNote, n.validate())
}

func (n *Note) validate() *ValidationError {
	if e := idErr("id", n.ID, TypeNote); e != nil {
		return e
	}
	if !validOpenEnum(string(n.Kind)) {
		return fieldErr("kind", "недопустимый вид заметки %q: ожидается [a-z][a-z0-9_-]*", n.Kind)
	}
	if strings.TrimSpace(n.Title) == "" && strings.TrimSpace(n.Text) == "" {
		return fieldErr("text", "нужен заголовок или текст")
	}

	if e := validateOptionalIDPtr("parent_id", n.ParentID, TypeNote); e != nil {
		return e
	}
	if e := validateNotSelf(n.ID, n.ParentID); e != nil {
		return e
	}

	return validateSourceLinks("sources", n.Sources)
}
