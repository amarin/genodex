package store

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Store — порт хранилища сущностей генеалогии.
// Реализация: internal/store/sqlstore (адаптер поверх internal/storage).
// MCP/API никогда не работают с портом напрямую — только через usecases.
//
// Контракт порта:
//   - Все методы принимают ctx первым аргументом: отмена или истечение срока
//     прерывают запрос и возвращают ошибку контекста (запись при этом не
//     применяется).
//   - Get* возвращает models.ErrNotFound (проверять через errors.Is), если
//     сущности с таким id нет; остальные ошибки — сбой хранилища.
//   - Delete* удаляет сущность вместе с её связными строками, записями
//     поискового индекса и source_links и осиротевшими значениями. Нет такой
//     сущности — models.ErrNotFound; на сущность ссылаются другие (строгие
//     ссылки) — *models.InUseError со списком ссылающихся (до
//     models.MaxReferrers), ничего не удаляется; ссылки «ON DELETE SET NULL»
//     (например, Attachment.DocumentID) обнуляются.
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
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
	SavePerson(ctx context.Context, p *models.Person) error
	ListPeople(ctx context.Context) ([]*models.Person, error)
	DeletePerson(ctx context.Context, id models.ID) error

	// Relation
	GetRelation(ctx context.Context, id models.ID) (*models.Relation, error)
	SaveRelation(ctx context.Context, r *models.Relation) error
	ListRelations(ctx context.Context) ([]*models.Relation, error)
	DeleteRelation(ctx context.Context, id models.ID) error

	// Residence
	GetResidence(ctx context.Context, id models.ID) (*models.Residence, error)
	SaveResidence(ctx context.Context, r *models.Residence) error
	ListResidences(ctx context.Context) ([]*models.Residence, error)
	DeleteResidence(ctx context.Context, id models.ID) error

	// Family
	GetFamily(ctx context.Context, id models.ID) (*models.Family, error)
	SaveFamily(ctx context.Context, f *models.Family) error
	ListFamilies(ctx context.Context) ([]*models.Family, error)
	DeleteFamily(ctx context.Context, id models.ID) error

	// Surname
	GetSurname(ctx context.Context, id models.ID) (*models.Surname, error)
	SaveSurname(ctx context.Context, s *models.Surname) error
	ListSurnames(ctx context.Context) ([]*models.Surname, error)
	DeleteSurname(ctx context.Context, id models.ID) error

	// GivenName
	GetGivenName(ctx context.Context, id models.ID) (*models.GivenName, error)
	SaveGivenName(ctx context.Context, g *models.GivenName) error
	ListGivenNames(ctx context.Context) ([]*models.GivenName, error)
	DeleteGivenName(ctx context.Context, id models.ID) error

	// Patronymic
	GetPatronymic(ctx context.Context, id models.ID) (*models.Patronymic, error)
	SavePatronymic(ctx context.Context, p *models.Patronymic) error
	ListPatronymics(ctx context.Context) ([]*models.Patronymic, error)
	DeletePatronymic(ctx context.Context, id models.ID) error

	// Estate
	GetEstate(ctx context.Context, id models.ID) (*models.Estate, error)
	SaveEstate(ctx context.Context, e *models.Estate) error
	ListEstates(ctx context.Context) ([]*models.Estate, error)
	DeleteEstate(ctx context.Context, id models.ID) error

	// Title
	GetTitle(ctx context.Context, id models.ID) (*models.Title, error)
	SaveTitle(ctx context.Context, t *models.Title) error
	ListTitles(ctx context.Context) ([]*models.Title, error)
	DeleteTitle(ctx context.Context, id models.ID) error

	// AdministrativeDivision
	GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error)
	SaveAdministrativeDivision(ctx context.Context, a *models.AdministrativeDivision) error
	ListAdministrativeDivisions(ctx context.Context) ([]*models.AdministrativeDivision, error)
	DeleteAdministrativeDivision(ctx context.Context, id models.ID) error

	// Church
	GetChurch(ctx context.Context, id models.ID) (*models.Church, error)
	SaveChurch(ctx context.Context, c *models.Church) error
	ListChurches(ctx context.Context) ([]*models.Church, error)
	DeleteChurch(ctx context.Context, id models.ID) error

	// Parish
	GetParish(ctx context.Context, id models.ID) (*models.Parish, error)
	SaveParish(ctx context.Context, p *models.Parish) error
	ListParishes(ctx context.Context) ([]*models.Parish, error)
	DeleteParish(ctx context.Context, id models.ID) error

	// Event
	GetEvent(ctx context.Context, id models.ID) (*models.Event, error)
	SaveEvent(ctx context.Context, e *models.Event) error
	ListEvents(ctx context.Context) ([]*models.Event, error)
	DeleteEvent(ctx context.Context, id models.ID) error

	// Source
	GetSource(ctx context.Context, id models.ID) (*models.Source, error)
	SaveSource(ctx context.Context, s *models.Source) error
	ListSources(ctx context.Context) ([]*models.Source, error)
	DeleteSource(ctx context.Context, id models.ID) error

	// Archive
	GetArchive(ctx context.Context, id models.ID) (*models.Archive, error)
	SaveArchive(ctx context.Context, a *models.Archive) error
	ListArchives(ctx context.Context) ([]*models.Archive, error)
	DeleteArchive(ctx context.Context, id models.ID) error

	// ArchiveNode
	GetArchiveNode(ctx context.Context, id models.ID) (*models.ArchiveNode, error)
	SaveArchiveNode(ctx context.Context, n *models.ArchiveNode) error
	ListArchiveNodes(ctx context.Context) ([]*models.ArchiveNode, error)
	DeleteArchiveNode(ctx context.Context, id models.ID) error

	// ArchiveDocument
	GetArchiveDocument(ctx context.Context, id models.ID) (*models.ArchiveDocument, error)
	SaveArchiveDocument(ctx context.Context, d *models.ArchiveDocument) error
	ListArchiveDocuments(ctx context.Context) ([]*models.ArchiveDocument, error)
	DeleteArchiveDocument(ctx context.Context, id models.ID) error

	// Attachment
	GetAttachment(ctx context.Context, id models.ID) (*models.Attachment, error)
	SaveAttachment(ctx context.Context, a *models.Attachment) error
	ListAttachments(ctx context.Context) ([]*models.Attachment, error)
	DeleteAttachment(ctx context.Context, id models.ID) error

	// Citation
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
	SaveCitation(ctx context.Context, c *models.Citation) error
	ListCitations(ctx context.Context) ([]*models.Citation, error)
	DeleteCitation(ctx context.Context, id models.ID) error

	// Note
	GetNote(ctx context.Context, id models.ID) (*models.Note, error)
	SaveNote(ctx context.Context, n *models.Note) error
	ListNotes(ctx context.Context) ([]*models.Note, error)
	DeleteNote(ctx context.Context, id models.ID) error

	// Repository
	GetRepository(ctx context.Context, id models.ID) (*models.Repository, error)
	SaveRepository(ctx context.Context, r *models.Repository) error
	ListRepositories(ctx context.Context) ([]*models.Repository, error)
	DeleteRepository(ctx context.Context, id models.ID) error
}
