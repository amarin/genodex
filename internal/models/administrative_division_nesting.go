package models

import (
	"fmt"
	"strings"
)

// NestingError возвращает *ValidationError по полю field, если единица типа
// child не может входить в единицу типа parent (AdminDivisionType.CanContain),
// иначе nil. Текст называет оба типа и перечисляет допустимые дочерние типы —
// по нему вызывающий (в том числе MCP-ассистент) может исправить запрос.
func NestingError(field string, parent, child AdminDivisionType) *ValidationError {
	if parent.CanContain(child) {
		return nil
	}

	allowed := parent.AllowedChildTypes()
	names := make([]string, len(allowed))
	for i, t := range allowed {
		names[i] = string(t)
	}

	hint := "в неё ничего добавить нельзя"
	if len(names) > 0 {
		hint = "допустимые дочерние типы: " + strings.Join(names, ", ")
	}

	return &ValidationError{
		Entity: TypeAdministrativeDivision,
		Field:  field,
		Reason: fmt.Sprintf("%s (%s) не может входить в %s (%s); %s",
			child.Label(), child, strings.ToLower(parent.Label()), parent, hint),
	}
}
