package models

import "strings"

// requireText проверяет обязательное текстовое поле: непустое после обрезки
// пробелов.
func requireText(field, s string) *ValidationError {
	if strings.TrimSpace(s) == "" {
		return fieldErr(field, "значение обязательно (непустое после обрезки пробелов)")
	}

	return nil
}

// validateStrings проверяет список строк: пустых элементов быть не должно.
func validateStrings(field string, values []string) *ValidationError {
	for i, s := range values {
		if strings.TrimSpace(s) == "" {
			return fieldErr(indexed(field, i), "значение не может быть пустым")
		}
	}

	return nil
}

// validateOptionalID проверяет необязательный идентификатор: пустой допустим,
// непустой должен иметь формат и префикс ожидаемого типа.
func validateOptionalID(field string, id ID, want Type) *ValidationError {
	if id == "" {
		return nil
	}

	return idErr(field, id, want)
}

// validateOptionalIDPtr проверяет необязательную ссылку-указатель: nil допустим,
// указатель на пустой id — нет («задан, но пуст»).
func validateOptionalIDPtr(field string, id *ID, want Type) *ValidationError {
	if id == nil {
		return nil
	}

	return idErr(field, *id, want)
}

// validateNotSelf проверяет, что родитель не совпадает с самой сущностью
// (циклы длиннее — забота сценариев: для них нужно хранилище).
func validateNotSelf(self ID, parent *ID) *ValidationError {
	if parent != nil && *parent == self {
		return fieldErr("parent_id", "родитель совпадает с самой сущностью")
	}

	return nil
}

// validateOptionalTextRefPtr проверяет необязательный *TextRef: nil допустим;
// заданный указатель проверяется целиком (пустой TextRef — ошибка).
func validateOptionalTextRefPtr(field string, r *TextRef, want Type) *ValidationError {
	if r == nil {
		return nil
	}

	return r.validateAs(want).within(field)
}
