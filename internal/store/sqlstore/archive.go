package sqlstore

import (
	"strconv"

	"github.com/amarin/genodex/internal/entity"
)

// GetArchive получает архив по ID.
func (s *Store) GetArchive(id string) (*entity.Archive, error) {
	return getJSON[entity.Archive](s, string(entity.TypeArchive), id)
}

// SaveArchive сохраняет архив; поисковый индекс — по названию.
func (s *Store) SaveArchive(a *entity.Archive) error {
	return s.saveJSON(string(entity.TypeArchive), a.ID, a, a.Name)
}

// ListArchives возвращает все архивы.
func (s *Store) ListArchives() ([]*entity.Archive, error) {
	return listJSON[entity.Archive](s, string(entity.TypeArchive))
}

// GetFund получает фонд по ID.
func (s *Store) GetFund(id string) (*entity.Fund, error) {
	return getJSON[entity.Fund](s, string(entity.TypeFund), id)
}

// SaveFund сохраняет фонд; поисковый индекс — по коду.
func (s *Store) SaveFund(f *entity.Fund) error {
	return s.saveJSON(string(entity.TypeFund), f.ID, f, f.Code)
}

// ListFunds возвращает все фонды.
func (s *Store) ListFunds() ([]*entity.Fund, error) {
	return listJSON[entity.Fund](s, string(entity.TypeFund))
}

// GetInventory получает опись по ID.
func (s *Store) GetInventory(id string) (*entity.Inventory, error) {
	return getJSON[entity.Inventory](s, string(entity.TypeInventory), id)
}

// SaveInventory сохраняет опись; поисковый индекс — по номеру.
func (s *Store) SaveInventory(inv *entity.Inventory) error {
	return s.saveJSON(string(entity.TypeInventory), inv.ID, inv, strconv.Itoa(inv.Number))
}

// ListInventories возвращает все описи.
func (s *Store) ListInventories() ([]*entity.Inventory, error) {
	return listJSON[entity.Inventory](s, string(entity.TypeInventory))
}

// GetCase получает дело по ID.
func (s *Store) GetCase(id string) (*entity.Case, error) {
	return getJSON[entity.Case](s, string(entity.TypeCase), id)
}

// SaveCase сохраняет дело; поисковый индекс — по номеру.
func (s *Store) SaveCase(c *entity.Case) error {
	return s.saveJSON(string(entity.TypeCase), c.ID, c, strconv.Itoa(c.Number))
}

// ListCases возвращает все дела.
func (s *Store) ListCases() ([]*entity.Case, error) {
	return listJSON[entity.Case](s, string(entity.TypeCase))
}
