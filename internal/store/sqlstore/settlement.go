package sqlstore

import "github.com/amarin/genodex/internal/entity"

// GetSettlement получает населённый пункт по ID.
func (s *Store) GetSettlement(id string) (*entity.Settlement, error) {
	return getJSON[entity.Settlement](s, string(entity.TypeSettlement), id)
}

// SaveSettlement сохраняет населённый пункт; поисковый индекс — по названию.
func (s *Store) SaveSettlement(settlement *entity.Settlement) error {
	return s.saveJSON(string(entity.TypeSettlement), settlement.ID, settlement, settlement.Name)
}

// ListSettlements возвращает все населённые пункты.
func (s *Store) ListSettlements() ([]*entity.Settlement, error) {
	return listJSON[entity.Settlement](s, string(entity.TypeSettlement))
}
