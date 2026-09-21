package models

// NameGender — пол, выводимый из имени (словарь GivenName).
type NameGender string

const (
	NameGenderMale    NameGender = "male"
	NameGenderFemale  NameGender = "female"
	NameGenderNeutral NameGender = "neutral"
)
