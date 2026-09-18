package entity

type SurnameType string

const (
	MainSurname    SurnameType = "main"
	MarriedSurname SurnameType = "married"
	PseudonymName  SurnameType = "pseudonym"
	ChangedSurname SurnameType = "changed"
	BirthSurname   SurnameType = "birth"
)
