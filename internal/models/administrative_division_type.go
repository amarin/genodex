package models

// AdminDivisionType — тип единицы административного деления: уровень деления
// или вид населённого пункта (решение #17). Один рекурсивный узел,
// глубина и состав системы не фиксированы.
type AdminDivisionType string

// Единицы деления.
const (
	AdminDivisionGovernorate AdminDivisionType = "governorate"
	AdminDivisionDistrict    AdminDivisionType = "district"
	AdminDivisionVolost      AdminDivisionType = "volost"
	AdminDivisionOther       AdminDivisionType = "other"
)

// Виды населённых пунктов (решение #17).
const (
	AdminDivisionGorod     AdminDivisionType = "gorod"
	AdminDivisionSelo      AdminDivisionType = "selo"
	AdminDivisionDerevnya  AdminDivisionType = "derevnya"
	AdminDivisionHutor     AdminDivisionType = "hutor"
	AdminDivisionPogost    AdminDivisionType = "pogost"
	AdminDivisionStanitsa  AdminDivisionType = "stanitsa"
	AdminDivisionMestechko AdminDivisionType = "mestechko"
)

// isDivisionUnit — единицы деления (не населённые пункты).
func isDivisionUnit(t AdminDivisionType) bool {
	switch t {
	case AdminDivisionGovernorate, AdminDivisionDistrict, AdminDivisionVolost, AdminDivisionOther:
		return true
	default:
		return false
	}
}

// IsSettlement сообщает, является ли тип видом населённого пункта.
func (t AdminDivisionType) IsSettlement() bool {
	return !isDivisionUnit(t)
}
