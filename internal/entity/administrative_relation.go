package entity

type AdministrativeDivisionTypeRelation struct {
	Parent AdministrativeDivisionType `json:"parent"`
	Child  AdministrativeDivisionType `json:"child"`
}
