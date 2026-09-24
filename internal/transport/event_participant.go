package transport

import "github.com/amarin/genodex/internal/models"

// EventParticipant — контракт одного участника события (models.
// EventParticipant): флат-объект {person_id, role, note} — в отличие от
// PersonName (подпроект 8), собственных вложенных объектов не несёт.
// PersonID — первая СТРОГАЯ (проверяемая на существование) ссылка внутри
// массива-объектов MCP-аргумента в программе (docs/data-model/entity-write.md
// §3.8): PersonName/SourceLink несли только мягкие/уже-установленные ссылки.
type EventParticipant struct {
	PersonID string `json:"person_id"`
	Role     string `json:"role"`
	Note     string `json:"note,omitempty"`
}

// EventParticipantFromModel конвертирует участника в контракт.
func EventParticipantFromModel(p models.EventParticipant) EventParticipant {
	return EventParticipant{
		PersonID: string(p.PersonID),
		Role:     p.Role,
		Note:     p.Note,
	}
}

// EventParticipantsFromModel конвертирует список; пустой вход даёт пустой
// срез, а не nil.
func EventParticipantsFromModel(ps []models.EventParticipant) []EventParticipant {
	out := make([]EventParticipant, 0, len(ps))
	for _, p := range ps {
		out = append(out, EventParticipantFromModel(p))
	}

	return out
}

// Model конвертирует контракт обратно в модель.
func (p EventParticipant) Model() models.EventParticipant {
	return models.EventParticipant{
		PersonID: models.ID(p.PersonID),
		Role:     p.Role,
		Note:     p.Note,
	}
}

// EventParticipantsToModel конвертирует список контрактов в модели; пустой
// вход даёт пустой срез, а не nil.
func EventParticipantsToModel(ps []EventParticipant) []models.EventParticipant {
	out := make([]models.EventParticipant, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Model())
	}

	return out
}
