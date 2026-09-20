package models

// Type — тип сущности генеалогии (дискриминатор хранилища и поискового индекса).
//
// Каталог типов соответствует решениям docs/data-model/decisions.md:
//
//	#1  брак/погребение — значения Event.type, отдельных сущностей нет
//	#2  единая рекурсивная AdministrativeDivision (без Governorate/District/Volost)
//	#14 архивы: рекурсивный ArchiveNode (без типизированных Fund/Inventory/Case)
//	#15 ArchiveDocument
//	#16 Attachment
//	#17 населённый пункт — вид AdministrativeDivision.type (без типа settlement)
//	#21 цитирование: Citation (промежуточное звено доказательства)
//	#22 Note (заметки первого класса)
//	#25 Repository (хранилище-контейнер источников)
type Type string

const (
	TypePerson                 Type = "person"
	TypeSurname                Type = "surname"
	TypeGivenName              Type = "given_name"
	TypePatronymic             Type = "patronymic"
	TypeEstate                 Type = "estate"
	TypeTitle                  Type = "title"
	TypeChurch                 Type = "church"
	TypeParish                 Type = "parish"
	TypeAdministrativeDivision Type = "administrative_division"
	TypeEvent                  Type = "event"
	TypeSource                 Type = "source"
	TypeCitation               Type = "citation"
	TypeNote                   Type = "note"
	TypeRepository             Type = "repository"
	TypeRelation               Type = "relation"
	TypeResidence              Type = "residence"
	TypeFamily                 Type = "family"
	TypeArchive                Type = "archive"
	TypeArchiveNode            Type = "archive_node"
	TypeArchiveDocument        Type = "archive_document"
	TypeAttachment             Type = "attachment"
)
