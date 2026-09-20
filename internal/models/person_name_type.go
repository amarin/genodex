package models

// PersonNameType — вид имени персоны (много имён у одной персоны).
type PersonNameType string

const (
	PersonNameMain      PersonNameType = "main"
	PersonNameBirth     PersonNameType = "birth"
	PersonNameMarried   PersonNameType = "married"
	PersonNameChanged   PersonNameType = "changed"
	PersonNamePseudonym PersonNameType = "pseudonym"
)
