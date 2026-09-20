package models

import (
	"fmt"
	"strings"
)

// ValidationError — нарушение инварианта сущности или значения. Validate()
// возвращает первую найденную ошибку.
type ValidationError struct {
	// Entity — тип сущности (пусто, если проверяется отдельный value-тип).
	Entity Type
	// Field — путь к полю: names[0].surname.type (имена — как в docs/models).
	Field string
	// Reason — причина по-русски.
	Reason string
}

// Error возвращает «сущность: поле: причина» (пустые части опускаются).
func (e *ValidationError) Error() string {
	var b strings.Builder
	if e.Entity != "" {
		b.WriteString(string(e.Entity))
		b.WriteString(": ")
	}
	if e.Field != "" {
		b.WriteString(e.Field)
		b.WriteString(": ")
	}
	b.WriteString(e.Reason)

	return b.String()
}

// fieldErr создаёт ошибку для поля.
func fieldErr(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// within добавляет к пути поля префикс родительского поля; nil остаётся nil.
func (e *ValidationError) within(prefix string) *ValidationError {
	if e == nil {
		return nil
	}

	switch {
	case e.Field == "":
		e.Field = prefix
	case strings.HasPrefix(e.Field, "["):
		e.Field = prefix + e.Field
	default:
		e.Field = prefix + "." + e.Field
	}

	return e
}

// indexed возвращает путь элемента списка: names[2].
func indexed(field string, i int) string {
	return fmt.Sprintf("%s[%d]", field, i)
}

// finish превращает внутреннюю ошибку в error публичного Validate: nil
// остаётся nil-интерфейсом (без typed-nil), иначе проставляется сущность.
func finish(entity Type, e *ValidationError) error {
	if e == nil {
		return nil
	}
	e.Entity = entity

	return e
}

// validOpenEnum проверяет значение открытого enum'а: непустое, формат
// [a-z][a-z0-9_-]*.
func validOpenEnum(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}

	return true
}
