package models

// AdminDivisionTypeInfo — справочная запись о типе единицы деления: название,
// ранг в иерархии, признак населённого пункта и допустимые дочерние типы.
type AdminDivisionTypeInfo struct {
	Type       AdminDivisionType
	Label      string
	Rank       *int // nil — у типа нет ранга (AdminDivisionOther)
	Settlement bool
	Children   []AdminDivisionType
}

// AdminDivisionTypeInfos возвращает справочник всех типов в каноническом
// порядке AdminDivisionTypes.
func AdminDivisionTypeInfos() []AdminDivisionTypeInfo {
	out := make([]AdminDivisionTypeInfo, len(AdminDivisionTypes))

	for i, t := range AdminDivisionTypes {
		out[i] = AdminDivisionTypeInfo{
			Type:       t,
			Label:      t.Label(),
			Settlement: t.IsSettlement(),
			Children:   t.AllowedChildTypes(),
		}
		if r, ok := t.Rank(); ok {
			out[i].Rank = &r
		}
	}

	return out
}
