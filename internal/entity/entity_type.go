package entity

// EntityType тип сущности генеалогии
type EntityType string

// Каталог типов соответствует решениям docs/data-model/decisions.md:
//
//	#1  брак/погребение — значения Event.type, отдельных сущностей нет
//	#2  единая рекурсивная AdministrativeDivision (без Governorate/District/Volost)
//	#14 архивы: рекурсивный ArchiveNode (без типизированных Fund/Inventory/Case)
//	#15 ArchiveDocument
//	#16 Attachment
const (
	TypePerson                 EntityType = "person"
	TypeSurname                EntityType = "surname"
	TypeFirstName              EntityType = "first_name"
	TypePatronymic             EntityType = "patronymic"
	TypeEstate                 EntityType = "estate"
	TypeTitle                  EntityType = "title"
	TypeChurch                 EntityType = "church"
	TypeParish                 EntityType = "parish"
	TypeAdministrativeDivision EntityType = "administrative_division"
	TypeSettlement             EntityType = "settlement"
	TypeEvent                  EntityType = "event"
	TypeSource                 EntityType = "source"
	TypeFamily                 EntityType = "family"
	TypeArchive                EntityType = "archive"
	TypeArchiveNode            EntityType = "archive_node"
	TypeArchiveDocument        EntityType = "archive_document"
	TypeAttachment             EntityType = "attachment"
)
