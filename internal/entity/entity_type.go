package entity

// EntityType тип сущности генеалогии
type EntityType string

const (
	TypePerson                 EntityType = "person"
	TypeSurname                EntityType = "surname"
	TypeFirstName              EntityType = "first_name"
	TypePatronymic             EntityType = "patronymic"
	TypeEstate                 EntityType = "estate"
	TypeTitle                  EntityType = "title"
	TypeChurch                 EntityType = "church"
	TypeParish                 EntityType = "parish"
	TypeGovernorate            EntityType = "governorate"
	TypeAdministrativeDivision EntityType = "administrative_division"
	TypeDistrict               EntityType = "district"
	TypeVolost                 EntityType = "volost"
	TypeSettlement             EntityType = "settlement"
	TypeEvent                  EntityType = "event"
	TypeMarriage               EntityType = "marriage"
	TypeBurial                 EntityType = "burial"
	TypeSource                 EntityType = "source"
	TypeFamily                 EntityType = "family"
	TypeArchive                EntityType = "archive"
	TypeFund                   EntityType = "fund"
	TypeInventory              EntityType = "inventory"
	TypeCase                   EntityType = "case"
)
