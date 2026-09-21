package models

// Validate проверяет узел цепочки хранения: уровень обязателен (открытый enum
// — системы иерархии архивов расширяются данными, а не кодом); архив — archive;
// родитель — archive_node, не совпадающий с самим узлом (циклы длиннее и
// соответствие уровня системе архива проверяют сценарии); шифр обязателен;
// период, приход и населённые пункты корректны.
func (n *ArchiveNode) Validate() error {
	return finish(TypeArchiveNode, n.validate())
}

func (n *ArchiveNode) validate() *ValidationError {
	if e := idErr("id", n.ID, TypeArchiveNode); e != nil {
		return e
	}
	if !validOpenEnum(string(n.Type)) {
		return fieldErr("type", "недопустимый уровень узла %q: ожидается [a-z][a-z0-9_-]*", n.Type)
	}
	if e := idErr("archive_id", n.ArchiveID, TypeArchive); e != nil {
		return e
	}

	if e := validateOptionalIDPtr("parent_id", n.ParentID, TypeArchiveNode); e != nil {
		return e
	}
	if e := validateNotSelf(n.ID, n.ParentID); e != nil {
		return e
	}
	if e := requireText("label", n.Label); e != nil {
		return e
	}

	if e := validatePeriod(n.Since, n.Until); e != nil {
		return e
	}
	if e := validateOptionalTextRefPtr("parish", n.Parish, TypeParish); e != nil {
		return e
	}
	if e := validateTextRefs("settlements", n.Settlements, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateTextRefs("notes", n.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", n.Sources)
}
