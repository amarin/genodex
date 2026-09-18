package entity

type AdministrativeDivisionSystem struct {
	Name      string                       `json:"name"`
	Relations []AdministrativeDivisionType `json:"relations"`
}
