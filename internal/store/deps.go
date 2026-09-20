package store

import "github.com/amarin/genodex/internal/models"

// Store — порт хранилища сущностей генеалогии.
// Реализация: internal/store/sqlstore (адаптер поверх internal/storage).
// MCP/API никогда не работают с портом напрямую — только через usecases.
//
// Контракт порта:
//   - Get* возвращает (nil, nil), если сущности с таким id нет; ошибка — только
//     при сбое хранилища.
//   - Save* — upsert: сохраняет сущность целиком и заменяет её дочерние строки
//     (имена, списки TextRef, доказательства и т. п.), а не дополняет их.
//   - Внешние ключи проверяются хранилищем: SourceLink.CitationID должен
//     указывать на существующую Citation, Citation.SourceID — на существующий
//     Source, Residence и Relation требуют существующих персон и мест,
//     непустые строгие id (ссылки на архив, репозиторий, приход и т. п.) должны
//     существовать. Иначе Save* возвращает ошибку внешнего ключа, и ничего не
//     записывается (сохранение атомарно). Ошибка Save* содержит вид и id
//     сущности.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type Store interface {
	// Person
	GetPerson(id models.ID) (*models.Person, error)
	SavePerson(p *models.Person) error
	ListPeople() ([]*models.Person, error)

	// Relation
	GetRelation(id models.ID) (*models.Relation, error)
	SaveRelation(r *models.Relation) error
	ListRelations() ([]*models.Relation, error)

	// Residence
	GetResidence(id models.ID) (*models.Residence, error)
	SaveResidence(r *models.Residence) error
	ListResidences() ([]*models.Residence, error)

	// Family
	GetFamily(id models.ID) (*models.Family, error)
	SaveFamily(f *models.Family) error
	ListFamilies() ([]*models.Family, error)

	// Surname
	GetSurname(id models.ID) (*models.Surname, error)
	SaveSurname(s *models.Surname) error
	ListSurnames() ([]*models.Surname, error)

	// GivenName
	GetGivenName(id models.ID) (*models.GivenName, error)
	SaveGivenName(g *models.GivenName) error
	ListGivenNames() ([]*models.GivenName, error)

	// Patronymic
	GetPatronymic(id models.ID) (*models.Patronymic, error)
	SavePatronymic(p *models.Patronymic) error
	ListPatronymics() ([]*models.Patronymic, error)

	// Estate
	GetEstate(id models.ID) (*models.Estate, error)
	SaveEstate(e *models.Estate) error
	ListEstates() ([]*models.Estate, error)

	// Title
	GetTitle(id models.ID) (*models.Title, error)
	SaveTitle(t *models.Title) error
	ListTitles() ([]*models.Title, error)

	// AdministrativeDivision
	GetAdministrativeDivision(id models.ID) (*models.AdministrativeDivision, error)
	SaveAdministrativeDivision(a *models.AdministrativeDivision) error
	ListAdministrativeDivisions() ([]*models.AdministrativeDivision, error)

	// Church
	GetChurch(id models.ID) (*models.Church, error)
	SaveChurch(c *models.Church) error
	ListChurches() ([]*models.Church, error)

	// Parish
	GetParish(id models.ID) (*models.Parish, error)
	SaveParish(p *models.Parish) error
	ListParishes() ([]*models.Parish, error)

	// Event
	GetEvent(id models.ID) (*models.Event, error)
	SaveEvent(e *models.Event) error
	ListEvents() ([]*models.Event, error)

	// Source
	GetSource(id models.ID) (*models.Source, error)
	SaveSource(s *models.Source) error
	ListSources() ([]*models.Source, error)

	// Archive
	GetArchive(id models.ID) (*models.Archive, error)
	SaveArchive(a *models.Archive) error
	ListArchives() ([]*models.Archive, error)

	// ArchiveNode
	GetArchiveNode(id models.ID) (*models.ArchiveNode, error)
	SaveArchiveNode(n *models.ArchiveNode) error
	ListArchiveNodes() ([]*models.ArchiveNode, error)

	// ArchiveDocument
	GetArchiveDocument(id models.ID) (*models.ArchiveDocument, error)
	SaveArchiveDocument(d *models.ArchiveDocument) error
	ListArchiveDocuments() ([]*models.ArchiveDocument, error)

	// Attachment
	GetAttachment(id models.ID) (*models.Attachment, error)
	SaveAttachment(a *models.Attachment) error
	ListAttachments() ([]*models.Attachment, error)

	// Citation
	GetCitation(id models.ID) (*models.Citation, error)
	SaveCitation(c *models.Citation) error
	ListCitations() ([]*models.Citation, error)

	// Note
	GetNote(id models.ID) (*models.Note, error)
	SaveNote(n *models.Note) error
	ListNotes() ([]*models.Note, error)

	// Repository
	GetRepository(id models.ID) (*models.Repository, error)
	SaveRepository(r *models.Repository) error
	ListRepositories() ([]*models.Repository, error)
}
