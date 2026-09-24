package models

// adminDivisionTypeRanks — условная «высота» типа в иерархии деления: дочерняя
// единица должна иметь ранг строго ниже родителя (CanContain). Единицы
// деления всегда выше населённых пунктов, поэтому в населённый пункт можно
// добавить только меньший населённый пункт. Типы одного уровня разных систем
// деления (губерния/область/наместничество/край, уезд/район, волость/сельсовет)
// имеют одинаковый ранг: смешение систем допустимо, «вверх» по иерархии — нет.
// AdminDivisionOther ранга не имеет — см. CanContain.
var adminDivisionTypeRanks = map[AdminDivisionType]int{
	AdminDivisionRespublika:      100,
	AdminDivisionNamestnichestvo: 90,
	AdminDivisionGuberniya:       90,
	AdminDivisionOblast:          90,
	AdminDivisionKrai:            90,
	AdminDivisionProvintsiya:     80,
	AdminDivisionOkrug:           75,
	AdminDivisionUezd:            70,
	AdminDivisionRayon:           70,
	AdminDivisionStan:            60,
	AdminDivisionVolost:          50,
	AdminDivisionSelsovet:        50,
	AdminDivisionGorod:           40,
	AdminDivisionPoselok:         30,
	AdminDivisionSloboda:         30,
	AdminDivisionStanitsa:        30,
	AdminDivisionMestechko:       30,
	AdminDivisionSelo:            20,
	AdminDivisionSeltso:          15,
	AdminDivisionPogost:          15,
	AdminDivisionDerevnya:        10,
	AdminDivisionHutor:           5,
}

// Rank возвращает ранг типа в иерархии деления; у AdminDivisionOther и
// недопустимых значений ранга нет (ok = false).
func (t AdminDivisionType) Rank() (rank int, ok bool) {
	rank, ok = adminDivisionTypeRanks[t]

	return rank, ok
}

// CanContain сообщает, может ли единица типа t быть родителем единицы типа
// child. Правило: ранг child строго ниже ранга t. Исключения для «Иное»
// (AdminDivisionOther): в него можно добавить любой тип, а само оно
// допустимо внутри любой единицы деления, но не населённого пункта.
// Недопустимые значения ничего не содержат и ни во что не входят.
func (t AdminDivisionType) CanContain(child AdminDivisionType) bool {
	if !t.Valid() || !child.Valid() {
		return false
	}
	if t == AdminDivisionOther {
		return true
	}
	if child == AdminDivisionOther {
		return !t.IsSettlement()
	}

	return adminDivisionTypeRanks[child] < adminDivisionTypeRanks[t]
}

// AllowedChildTypes возвращает типы, допустимые внутри единицы типа t, в
// каноническом порядке AdminDivisionTypes.
func (t AdminDivisionType) AllowedChildTypes() []AdminDivisionType {
	var out []AdminDivisionType

	for _, c := range AdminDivisionTypes {
		if t.CanContain(c) {
			out = append(out, c)
		}
	}

	return out
}
