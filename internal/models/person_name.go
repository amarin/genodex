package models

// PersonName — одно имя персоны (фамилия/имя/отчество могут быть нескольких
// родов: основное, по браку, при рождении…). Каждое из полей — TextRef
// с мягкой ссылкой на словарную запись (Surname/GivenName/Patronymic).
// Prefix/Suffix — служебные части имени (фон/де, ст./мл.) (решение #28).
type PersonName struct {
	Type       PersonNameType
	Surname    TextRef
	Given      TextRef
	Patronymic TextRef
	Prefix     string
	Suffix     string
	Since      *FactDate
	Until      *FactDate
}
