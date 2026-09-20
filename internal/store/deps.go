package store

import "github.com/amarin/genodex/internal/entity"

// Store — порт хранилища сущностей генеалогии.
// Реализация: internal/store/sqlstore (адаптер поверх internal/storage).
// MCP/API никогда не работают с портом напрямую — только через usecases.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type Store interface {
	// Person
	GetPerson(id string) (*entity.Person, error)
	SavePerson(p *entity.Person) error
	ListPeople() ([]*entity.Person, error)

	// Settlement
	GetSettlement(id string) (*entity.Settlement, error)
	SaveSettlement(settlement *entity.Settlement) error
	ListSettlements() ([]*entity.Settlement, error)

	// Church
	GetChurch(id string) (*entity.Church, error)
	SaveChurch(church *entity.Church) error
	ListChurches() ([]*entity.Church, error)

	// Parish
	GetParish(id string) (*entity.Parish, error)
	SaveParish(parish *entity.Parish) error
	ListParishes() ([]*entity.Parish, error)

	// Archive
	GetArchive(id string) (*entity.Archive, error)
	SaveArchive(a *entity.Archive) error
	ListArchives() ([]*entity.Archive, error)

	// Event
	GetEvent(id string) (*entity.Event, error)
	SaveEvent(e *entity.Event) error
	ListEvents() ([]*entity.Event, error)

	// Source
	GetSource(id string) (*entity.Source, error)
	SaveSource(src *entity.Source) error
	ListSources() ([]*entity.Source, error)
}
