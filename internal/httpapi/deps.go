package httpapi

import (
	"context"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

// DivisionService — контракт сценариев административного деления, отдаваемых
// в HTTP: список, чтение, создание, изменение, удаление.
type DivisionService interface {
	ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
	SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error)
	GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error)
	CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error)
	UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error
	DeleteDivision(ctx context.Context, id models.ID) error
}

// SurnameService — контракт сценариев словарных записей фамилий, отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type SurnameService interface {
	ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error)
	SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error)
	GetSurname(ctx context.Context, id models.ID) (models.Surname, error)
	CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error)
	UpdateSurname(ctx context.Context, sn models.Surname) error
	DeleteSurname(ctx context.Context, id models.ID) error
}

// PatronymicService — контракт сценариев словарных записей (отчеств), отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type PatronymicService interface {
	ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error)
	SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error)
	GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error)
	CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error)
	UpdatePatronymic(ctx context.Context, x models.Patronymic) error
	DeletePatronymic(ctx context.Context, id models.ID) error
}

// EstateService — контракт сценариев словарных записей (сословий), отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type EstateService interface {
	ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error)
	SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error)
	GetEstate(ctx context.Context, id models.ID) (models.Estate, error)
	CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error)
	UpdateEstate(ctx context.Context, x models.Estate) error
	DeleteEstate(ctx context.Context, id models.ID) error
}

// TitleService — контракт сценариев словарных записей (званий/титулов), отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type TitleService interface {
	ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error)
	SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error)
	GetTitle(ctx context.Context, id models.ID) (models.Title, error)
	CreateTitle(ctx context.Context, x models.Title) (models.Title, error)
	UpdateTitle(ctx context.Context, x models.Title) error
	DeleteTitle(ctx context.Context, id models.ID) error
}

// GivenNameService — контракт сценариев словарных записей имён, отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type GivenNameService interface {
	ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error)
	SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error)
	GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error)
	CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error)
	UpdateGivenName(ctx context.Context, x models.GivenName) error
	DeleteGivenName(ctx context.Context, id models.ID) error
}

// RepositoryService — контракт сценариев хранилищ-контейнеров источников,
// отдаваемых в HTTP: список, поиск, чтение, создание, изменение, удаление.
type RepositoryService interface {
	ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error)
	SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error)
	GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error)
	CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error)
	UpdateRepository(ctx context.Context, r models.Repository) error
	DeleteRepository(ctx context.Context, id models.ID) error
}

// ChurchService — контракт сценариев церквей, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type ChurchService interface {
	ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error)
	SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error)
	GetChurch(ctx context.Context, id models.ID) (models.Church, error)
	CreateChurch(ctx context.Context, c models.Church) (models.Church, error)
	UpdateChurch(ctx context.Context, c models.Church) error
	DeleteChurch(ctx context.Context, id models.ID) error
}

// ParishService — контракт сценариев приходов, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type ParishService interface {
	ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error)
	SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error)
	GetParish(ctx context.Context, id models.ID) (models.Parish, error)
	CreateParish(ctx context.Context, p models.Parish) (models.Parish, error)
	UpdateParish(ctx context.Context, p models.Parish) error
	DeleteParish(ctx context.Context, id models.ID) error
}

// ArchiveService — контракт сценариев архивов, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type ArchiveService interface {
	ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error)
	SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error)
	GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error)
	CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error)
	UpdateArchive(ctx context.Context, a models.Archive) error
	DeleteArchive(ctx context.Context, id models.ID) error
}

// ArchiveNodeService — контракт сценариев узлов архивного дерева, отдаваемых
// в HTTP: список (обязательный archive_id — см. models.ArchiveNodeQuery),
// поиск, чтение, создание, изменение, удаление.
type ArchiveNodeService interface {
	ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error)
	SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error)
	GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error)
	CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error)
	UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error
	DeleteArchiveNode(ctx context.Context, id models.ID) error
}

// ArchiveDocumentService — контракт сценариев документов внутри единиц
// учёта, отдаваемых в HTTP: список, поиск, чтение, создание, изменение,
// удаление.
type ArchiveDocumentService interface {
	ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error)
	SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error)
	GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error)
	CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error)
	UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error
	DeleteArchiveDocument(ctx context.Context, id models.ID) error
}

// NoteService — контракт сценариев заметок, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type NoteService interface {
	ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error)
	SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error)
	GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error)
	CreateNote(ctx context.Context, n models.Note) (models.Note, error)
	UpdateNote(ctx context.Context, n models.Note) error
	DeleteNote(ctx context.Context, id models.ID) error
}

// AttachmentService — контракт сценариев файловых вложений, отдаваемых в
// HTTP: список, поиск, чтение, создание, изменение, удаление.
type AttachmentService interface {
	ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error)
	SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error)
	GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error)
	CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error)
	UpdateAttachment(ctx context.Context, a models.Attachment) error
	DeleteAttachment(ctx context.Context, id models.ID) error
}

// SourceService — контракт сценариев источников доказательств, отдаваемых в
// HTTP: список, поиск, чтение, создание, изменение, удаление.
type SourceService interface {
	ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error)
	SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error)
	GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error)
	CreateSource(ctx context.Context, s models.Source) (models.Source, error)
	UpdateSource(ctx context.Context, s models.Source) error
	DeleteSource(ctx context.Context, id models.ID) error
}

// CitationService — контракт сценариев цитат, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type CitationService interface {
	ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error)
	SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error)
	GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error)
	CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error)
	UpdateCitation(ctx context.Context, c models.Citation) error
	DeleteCitation(ctx context.Context, id models.ID) error
}

// FamilyService — контракт сценариев родов/линий, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type FamilyService interface {
	ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error)
	SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error)
	GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error)
	CreateFamily(ctx context.Context, f models.Family) (models.Family, error)
	UpdateFamily(ctx context.Context, f models.Family) error
	DeleteFamily(ctx context.Context, id models.ID) error
}

// PersonService — контракт сценариев персон, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление. Список/поиск используют
// имена ListPeople/SearchPeople (неправильное множественное число,
// зеркалирует store.Store.ListPeople) — единственное исключение из общего
// правила «<Глагол><ИмяСущностиВоМножественномЧисле>» в этой программе.
type PersonService interface {
	ListPeople(ctx context.Context, access models.Access, page models.Page) ([]models.Person, error)
	SearchPeople(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Person, error)
	GetPerson(ctx context.Context, access models.Access, id models.ID) (models.Person, error)
	CreatePerson(ctx context.Context, p models.Person) (models.Person, error)
	UpdatePerson(ctx context.Context, p models.Person) error
	DeletePerson(ctx context.Context, id models.ID) error
}

// RelationService — контракт сценариев рёбер графа родства, отдаваемых в
// HTTP: список (необязательный фильтр по персоне, см. models.RelationQuery),
// чтение, создание, изменение, удаление. Без поиска — search_relations
// намеренно не заводится, см. docs/data-model/entity-write.md §3.8.
type RelationService interface {
	ListRelations(ctx context.Context, access models.Access, q models.RelationQuery) ([]models.Relation, error)
	GetRelation(ctx context.Context, access models.Access, id models.ID) (models.Relation, error)
	CreateRelation(ctx context.Context, r models.Relation) (models.Relation, error)
	UpdateRelation(ctx context.Context, r models.Relation) error
	DeleteRelation(ctx context.Context, id models.ID) error
}

// ResidenceService — контракт сценариев проживаний, отдаваемых в HTTP:
// список (необязательные фильтры по персоне и месту, см.
// models.ResidenceQuery), чтение, создание, изменение, удаление. Без поиска —
// search_residences намеренно не заводится, см.
// docs/data-model/entity-write.md §3.8.
type ResidenceService interface {
	ListResidences(ctx context.Context, access models.Access, q models.ResidenceQuery) ([]models.Residence, error)
	GetResidence(ctx context.Context, access models.Access, id models.ID) (models.Residence, error)
	CreateResidence(ctx context.Context, r models.Residence) (models.Residence, error)
	UpdateResidence(ctx context.Context, r models.Residence) error
	DeleteResidence(ctx context.Context, id models.ID) error
}

// EventService — контракт сценариев событий, отдаваемых в HTTP: список
// (необязательный фильтр по участнику, см. models.EventQuery), поиск (по
// началу текста места), чтение, создание, изменение, удаление.
type EventService interface {
	ListEvents(ctx context.Context, access models.Access, q models.EventQuery) ([]models.Event, error)
	SearchEvents(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Event, error)
	GetEvent(ctx context.Context, access models.Access, id models.ID) (models.Event, error)
	CreateEvent(ctx context.Context, e models.Event) (models.Event, error)
	UpdateEvent(ctx context.Context, e models.Event) error
	DeleteEvent(ctx context.Context, id models.ID) error
}

// AuthService — контракт auth.Service, отдаваемый в HTTP-обработчики.
type AuthService interface {
	Bootstrap(ctx context.Context) (bool, error)
	Register(ctx context.Context, login, password string, invite *string) (authpkg.AuthResult, error)
	Login(ctx context.Context, login, password string) (authpkg.AuthResult, error)
	Refresh(ctx context.Context, rawRefresh string) (authpkg.AuthResult, error)
	Logout(ctx context.Context, rawAccess string) error
	ResolveAccess(ctx context.Context, rawAccess string) (models.Access, *authpkg.ID, error)
	ChangePassword(ctx context.Context, ownerID authpkg.ID, current, newPassword string) error
	CreateInvite(ctx context.Context, ownerID authpkg.ID) (string, error)
	CreateAPIToken(ctx context.Context, ownerID authpkg.ID, label string) (string, authpkg.ID, error)
	ListAPITokens(ctx context.Context, ownerID authpkg.ID) ([]authpkg.APIToken, error)
	RevokeAPIToken(ctx context.Context, ownerID, tokenID authpkg.ID) error
	GetOwner(ctx context.Context, id authpkg.ID) (*authpkg.Owner, error)
}
