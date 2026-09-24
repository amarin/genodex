package models

// adminDivisionTypeLabels — русское название (термин) каждого типа.
var adminDivisionTypeLabels = map[AdminDivisionType]string{
	AdminDivisionNamestnichestvo: "Наместничество",
	AdminDivisionProvintsiya:     "Провинция",
	AdminDivisionGuberniya:       "Губерния",
	AdminDivisionUezd:            "Уезд",
	AdminDivisionStan:            "Стан",
	AdminDivisionVolost:          "Волость",
	AdminDivisionOblast:          "Область",
	AdminDivisionOkrug:           "Округ",
	AdminDivisionRespublika:      "Республика",
	AdminDivisionKrai:            "Край",
	AdminDivisionRayon:           "Район",
	AdminDivisionSelsovet:        "Сельсовет",
	AdminDivisionOther:           "Иное",
	AdminDivisionGorod:           "Город",
	AdminDivisionPoselok:         "Посёлок",
	AdminDivisionSloboda:         "Слобода",
	AdminDivisionSelo:            "Село",
	AdminDivisionSeltso:          "Сельцо",
	AdminDivisionDerevnya:        "Деревня",
	AdminDivisionHutor:           "Хутор",
	AdminDivisionPogost:          "Погост",
	AdminDivisionStanitsa:        "Станица",
	AdminDivisionMestechko:       "Местечко",
}

// Label возвращает русское название типа; для недопустимого значения — сам код.
func (t AdminDivisionType) Label() string {
	if l, ok := adminDivisionTypeLabels[t]; ok {
		return l
	}

	return string(t)
}
