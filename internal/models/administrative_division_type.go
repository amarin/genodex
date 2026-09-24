package models

// AdminDivisionType — тип единицы административного деления: уровень деления
// или вид населённого пункта (решение #17). Один рекурсивный узел,
// глубина и состав системы не фиксированы. Каждое значение однозначно
// соответствует одному историческому термину; коды — транслитерация термина.
type AdminDivisionType string

// Единицы деления Российской империи.
const (
	AdminDivisionNamestnichestvo AdminDivisionType = "namestnichestvo" // наместничество
	AdminDivisionProvintsiya     AdminDivisionType = "provintsiya"     // провинция
	AdminDivisionGuberniya       AdminDivisionType = "guberniya"       // губерния
	AdminDivisionUezd            AdminDivisionType = "uezd"            // уезд
	AdminDivisionStan            AdminDivisionType = "stan"            // стан
	AdminDivisionVolost          AdminDivisionType = "volost"          // волость
)

// Единицы деления, общие для империи и СССР/современности.
const (
	AdminDivisionOblast AdminDivisionType = "oblast" // область
	AdminDivisionOkrug  AdminDivisionType = "okrug"  // округ
)

// Единицы деления СССР и современности.
const (
	AdminDivisionRespublika AdminDivisionType = "respublika" // республика
	AdminDivisionKrai       AdminDivisionType = "krai"       // край
	AdminDivisionRayon      AdminDivisionType = "rayon"      // район
	AdminDivisionSelsovet   AdminDivisionType = "selsovet"   // сельсовет
)

// AdminDivisionOther — единица деления, для которой нет своего термина.
const AdminDivisionOther AdminDivisionType = "other"

// Виды населённых пунктов (решение #17).
const (
	AdminDivisionGorod     AdminDivisionType = "gorod"     // город
	AdminDivisionPoselok   AdminDivisionType = "poselok"   // посёлок
	AdminDivisionSloboda   AdminDivisionType = "sloboda"   // слобода
	AdminDivisionSelo      AdminDivisionType = "selo"      // село
	AdminDivisionSeltso    AdminDivisionType = "seltso"    // сельцо
	AdminDivisionDerevnya  AdminDivisionType = "derevnya"  // деревня
	AdminDivisionHutor     AdminDivisionType = "hutor"     // хутор
	AdminDivisionPogost    AdminDivisionType = "pogost"    // погост
	AdminDivisionStanitsa  AdminDivisionType = "stanitsa"  // станица
	AdminDivisionMestechko AdminDivisionType = "mestechko" // местечко
)

// AdminDivisionTypes — все допустимые значения в каноническом порядке:
// единицы деления, other, затем виды населённых пунктов. Источник для
// перечней в описаниях публичных интерфейсов.
var AdminDivisionTypes = []AdminDivisionType{
	AdminDivisionNamestnichestvo, AdminDivisionProvintsiya, AdminDivisionGuberniya,
	AdminDivisionUezd, AdminDivisionStan, AdminDivisionVolost,
	AdminDivisionOblast, AdminDivisionOkrug,
	AdminDivisionRespublika, AdminDivisionKrai, AdminDivisionRayon, AdminDivisionSelsovet,
	AdminDivisionOther,
	AdminDivisionGorod, AdminDivisionPoselok, AdminDivisionSloboda, AdminDivisionSelo,
	AdminDivisionSeltso, AdminDivisionDerevnya, AdminDivisionHutor, AdminDivisionPogost,
	AdminDivisionStanitsa, AdminDivisionMestechko,
}

// IsSettlement сообщает, является ли тип видом населённого пункта.
// Белый список: неизвестные и пустые значения населёнными пунктами не считаются.
func (t AdminDivisionType) IsSettlement() bool {
	switch t {
	case AdminDivisionGorod, AdminDivisionPoselok, AdminDivisionSloboda, AdminDivisionSelo,
		AdminDivisionSeltso, AdminDivisionDerevnya, AdminDivisionHutor, AdminDivisionPogost,
		AdminDivisionStanitsa, AdminDivisionMestechko:
		return true
	default:
		return false
	}
}
