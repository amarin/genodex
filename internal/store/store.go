package store

import (
	"sync"

	"github.com/amarin/genodex/internal/entity"
)

// Store хранилище сущностей генеалогии
type Store struct {
	mu sync.RWMutex

	people       map[string]*entity.Person
	settlements  map[string]*entity.Settlement
	churches     map[string]*entity.Church
	parishes     map[string]*entity.Parish
	governorates map[string]*entity.Governorate
	districts    map[string]*entity.District
	volosts      map[string]*entity.Volost
	archives     map[string]*entity.Archive
	funds        map[string]*entity.Fund
	inventories  map[string]*entity.Inventory
	cases        map[string]*entity.Case
	events       map[string]*entity.Event
	marriages    map[string]*entity.Marriage
	sources      map[string]*entity.Source
}

// New создаёт новое хранилище
func New() *Store {
	return &Store{
		people:       make(map[string]*entity.Person),
		settlements:  make(map[string]*entity.Settlement),
		churches:     make(map[string]*entity.Church),
		parishes:     make(map[string]*entity.Parish),
		governorates: make(map[string]*entity.Governorate),
		districts:    make(map[string]*entity.District),
		volosts:      make(map[string]*entity.Volost),
		archives:     make(map[string]*entity.Archive),
		funds:        make(map[string]*entity.Fund),
		inventories:  make(map[string]*entity.Inventory),
		cases:        make(map[string]*entity.Case),
		events:       make(map[string]*entity.Event),
		marriages:    make(map[string]*entity.Marriage),
		sources:      make(map[string]*entity.Source),
	}
}

// Person методы для работы с персонами

// GetPerson получает персону по ID
func (s *Store) GetPerson(id string) *entity.Person {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.people[id]
}

// SavePerson сохраняет персону
func (s *Store) SavePerson(p *entity.Person) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.people[p.ID] = p
}

// ListPeople возвращает всех персон
func (s *Store) ListPeople() []*entity.Person {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Person, 0, len(s.people))
	for _, p := range s.people {
		result = append(result, p)
	}
	return result
}

// Settlement методы для работы с населёнными пунктами

// GetSettlement получает населённый пункт по ID
func (s *Store) GetSettlement(id string) *entity.Settlement {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settlements[id]
}

// SaveSettlement сохраняет населённый пункт
func (s *Store) SaveSettlement(settlement *entity.Settlement) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settlements[settlement.ID] = settlement
}

// ListSettlements возвращает все населённые пункты
func (s *Store) ListSettlements() []*entity.Settlement {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Settlement, 0, len(s.settlements))
	for _, s := range s.settlements {
		result = append(result, s)
	}
	return result
}

// Church методы для работы с церквями

// GetChurch получает церковь по ID
func (s *Store) GetChurch(id string) *entity.Church {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.churches[id]
}

// SaveChurch сохраняет церковь
func (s *Store) SaveChurch(church *entity.Church) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.churches[church.ID] = church
}

// ListChurches возвращает все церкви
func (s *Store) ListChurches() []*entity.Church {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Church, 0, len(s.churches))
	for _, c := range s.churches {
		result = append(result, c)
	}
	return result
}

// Parish методы для работы с приходами

// GetParish получает приход по ID
func (s *Store) GetParish(id string) *entity.Parish {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.parishes[id]
}

// SaveParish сохраняет приход
func (s *Store) SaveParish(parish *entity.Parish) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.parishes[parish.ID] = parish
}

// ListParishes возвращает все приходы
func (s *Store) ListParishes() []*entity.Parish {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Parish, 0, len(s.parishes))
	for _, p := range s.parishes {
		result = append(result, p)
	}
	return result
}

// Governorate методы для работы с губерниями

// GetGovernorate получает губернию по ID
func (s *Store) GetGovernorate(id string) *entity.Governorate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.governorates[id]
}

// SaveGovernorate сохраняет губернию
func (s *Store) SaveGovernorate(g *entity.Governorate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.governorates[g.ID] = g
}

// ListGovernorates возвращает все губернии
func (s *Store) ListGovernorates() []*entity.Governorate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Governorate, 0, len(s.governorates))
	for _, g := range s.governorates {
		result = append(result, g)
	}
	return result
}

// District методы для работы с уездами

// GetDistrict получает уезд по ID
func (s *Store) GetDistrict(id string) *entity.District {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.districts[id]
}

// SaveDistrict сохраняет уезд
func (s *Store) SaveDistrict(d *entity.District) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.districts[d.ID] = d
}

// ListDistricts возвращает все уезды
func (s *Store) ListDistricts() []*entity.District {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.District, 0, len(s.districts))
	for _, d := range s.districts {
		result = append(result, d)
	}
	return result
}

// Volost методы для работы с волостями

// GetVolost получает волость по ID
func (s *Store) GetVolost(id string) *entity.Volost {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.volosts[id]
}

// SaveVolost сохраняет волость
func (s *Store) SaveVolost(v *entity.Volost) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.volosts[v.ID] = v
}

// ListVolosts возвращает все волости
func (s *Store) ListVolosts() []*entity.Volost {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Volost, 0, len(s.volosts))
	for _, v := range s.volosts {
		result = append(result, v)
	}
	return result
}

// Archive методы для работы с архивами

// GetArchive получает архив по ID
func (s *Store) GetArchive(id string) *entity.Archive {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.archives[id]
}

// SaveArchive сохраняет архив
func (s *Store) SaveArchive(a *entity.Archive) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.archives[a.ID] = a
}

// ListArchives возвращает все архивы
func (s *Store) ListArchives() []*entity.Archive {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Archive, 0, len(s.archives))
	for _, a := range s.archives {
		result = append(result, a)
	}
	return result
}

// Fund методы для работы с фондами

// GetFund получает фонд по ID
func (s *Store) GetFund(id string) *entity.Fund {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.funds[id]
}

// SaveFund сохраняет фонд
func (s *Store) SaveFund(f *entity.Fund) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.funds[f.ID] = f
}

// ListFunds возвращает все фонды
func (s *Store) ListFunds() []*entity.Fund {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Fund, 0, len(s.funds))
	for _, f := range s.funds {
		result = append(result, f)
	}
	return result
}

// Inventory методы для работы с описями

// GetInventory получает опись по ID
func (s *Store) GetInventory(id string) *entity.Inventory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inventories[id]
}

// SaveInventory сохраняет опись
func (s *Store) SaveInventory(inv *entity.Inventory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inventories[inv.ID] = inv
}

// ListInventories возвращает все описи
func (s *Store) ListInventories() []*entity.Inventory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Inventory, 0, len(s.inventories))
	for _, inv := range s.inventories {
		result = append(result, inv)
	}
	return result
}

// Case методы для работы с делами

// GetCase получает дело по ID
func (s *Store) GetCase(id string) *entity.Case {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cases[id]
}

// SaveCase сохраняет дело
func (s *Store) SaveCase(c *entity.Case) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cases[c.ID] = c
}

// ListCases возвращает все дела
func (s *Store) ListCases() []*entity.Case {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Case, 0, len(s.cases))
	for _, c := range s.cases {
		result = append(result, c)
	}
	return result
}

// Event методы для работы с событиями

// GetEvent получает событие по ID
func (s *Store) GetEvent(id string) *entity.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events[id]
}

// SaveEvent сохраняет событие
func (s *Store) SaveEvent(e *entity.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[e.ID] = e
}

// ListEvents возвращает все события
func (s *Store) ListEvents() []*entity.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Event, 0, len(s.events))
	for _, e := range s.events {
		result = append(result, e)
	}
	return result
}

// Marriage методы для работы с браками

// GetMarriage получает брак по ID
func (s *Store) GetMarriage(id string) *entity.Marriage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.marriages[id]
}

// SaveMarriage сохраняет брак
func (s *Store) SaveMarriage(m *entity.Marriage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marriages[m.ID] = m
}

// ListMarriages возвращает все браки
func (s *Store) ListMarriages() []*entity.Marriage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Marriage, 0, len(s.marriages))
	for _, m := range s.marriages {
		result = append(result, m)
	}
	return result
}

// Source методы для работы с источниками

// GetSource получает источник по ID
func (s *Store) GetSource(id string) *entity.Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sources[id]
}

// SaveSource сохраняет источник
func (s *Store) SaveSource(src *entity.Source) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources[src.ID] = src
}

// ListSources возвращает все источники
func (s *Store) ListSources() []*entity.Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*entity.Source, 0, len(s.sources))
	for _, src := range s.sources {
		result = append(result, src)
	}
	return result
}
