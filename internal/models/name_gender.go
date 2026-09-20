package models

// NameGender — пол, выводимый из имени (словарь GivenName).
type NameGender string

const (
	MaleName    NameGender = "male"
	FemaleName  NameGender = "female"
	NeutralName NameGender = "neutral"
)
