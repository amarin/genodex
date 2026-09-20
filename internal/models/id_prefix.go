package models

// idPrefixTable — соответствие типа сущности и префикса идентификатора.
// Префиксы I, F, S, R, N, O совпадают с GEDCOM (INDI, FAM, SOUR, REPO, NOTE,
// OBJE); остальные уникальны. Порядок задаёт порядок AllTypes.
var idPrefixTable = [...]struct {
	Type   Type
	Prefix string
}{
	{TypePerson, "I"},
	{TypeFamily, "F"},
	{TypeSource, "S"},
	{TypeRepository, "R"},
	{TypeNote, "N"},
	{TypeAttachment, "O"},
	{TypeEvent, "E"},
	{TypeCitation, "C"},
	{TypeRelation, "RL"},
	{TypeResidence, "RS"},
	{TypeSurname, "SN"},
	{TypeGivenName, "GN"},
	{TypePatronymic, "PN"},
	{TypeEstate, "ES"},
	{TypeTitle, "TT"},
	{TypeChurch, "CH"},
	{TypeParish, "PR"},
	{TypeAdministrativeDivision, "AD"},
	{TypeArchive, "AR"},
	{TypeArchiveNode, "AN"},
	{TypeArchiveDocument, "DC"},
}

// IDPrefix возвращает префикс идентификаторов этого типа сущности; для
// неизвестного типа — пустую строку.
func (t Type) IDPrefix() string {
	for _, e := range idPrefixTable {
		if e.Type == t {
			return e.Prefix
		}
	}

	return ""
}

// TypeByIDPrefix возвращает тип сущности по префиксу идентификатора.
func TypeByIDPrefix(prefix string) (Type, bool) {
	for _, e := range idPrefixTable {
		if e.Prefix == prefix {
			return e.Type, true
		}
	}

	return "", false
}

// AllTypes возвращает все типы сущностей в стабильном порядке таблицы префиксов.
func AllTypes() []Type {
	out := make([]Type, len(idPrefixTable))
	for i, e := range idPrefixTable {
		out[i] = e.Type
	}

	return out
}
