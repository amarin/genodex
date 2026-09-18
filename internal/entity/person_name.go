package entity

type PersonName struct {
	FirstName  Name   `json:"first_name"`
	Patronymic string `json:"patronymic,omitempty"`
	Surname    string `json:"surname"`
}
