package models

import "testing"

func validEvent() *Event {
	return &Event{
		ID:    testID(TypeEvent),
		Type:  EventTypeBirth,
		Date:  &FactDate{Year: 1881, Month: 3, Day: 15, Precision: PrecisionDay, Modifier: ModifierExact},
		Place: &PlaceRef{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision},
		Participants: []EventParticipant{
			{PersonID: testID(TypePerson), Role: "ребёнок", Note: "первый"},
			{PersonID: personBID(), Role: "восприемник"},
		},
		Sources: []SourceLink{{CitationID: testID(TypeCitation)}},
		Notes:   []TextRef{{Text: "метрическая запись"}},
		Private: true,
	}
}

func TestEventValidate(t *testing.T) {
	if err := validEvent().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Дата, место и участники необязательны.
	if err := (&Event{ID: testID(TypeEvent), Type: "census"}).Validate(); err != nil {
		t.Errorf("минимальное событие: %v", err)
	}
	// Открытый enum: своё значение допустимо.
	if err := (&Event{ID: testID(TypeEvent), Type: "first-communion"}).Validate(); err != nil {
		t.Errorf("своё значение типа: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Event)
		field  string
	}{
		{"плохой id", func(e *Event) { e.ID = "ev-1" }, "id"},
		{"id другого типа", func(e *Event) { e.ID = testID(TypeCitation) }, "id"},
		{"пустой тип", func(e *Event) { e.Type = "" }, "type"},
		{"тип не в формате", func(e *Event) { e.Type = "Birth" }, "type"},
		{"тип с пробелом", func(e *Event) { e.Type = "first communion" }, "type"},
		{"плохая дата", func(e *Event) { e.Date = &FactDate{} }, "date.precision"},
		{"31 апреля", func(e *Event) {
			e.Date = &FactDate{Year: 1881, Month: 4, Day: 31, Precision: PrecisionDay, Modifier: ModifierExact}
		}, "date.day"},
		{"место — персона", func(e *Event) { e.Place = &PlaceRef{Ref: testID(TypePerson), Type: TypePerson} }, "place.type"},
		{"пустое место", func(e *Event) { e.Place = &PlaceRef{} }, "place.text"},
		{"участник без персоны", func(e *Event) { e.Participants[0].PersonID = "" }, "participants[0].person_id"},
		{"участник — не персона", func(e *Event) { e.Participants[1].PersonID = testID(TypeFamily) }, "participants[1].person_id"},
		{"участник без роли", func(e *Event) { e.Participants[1].Role = "" }, "participants[1].role"},
		{"роль из пробелов", func(e *Event) { e.Participants[0].Role = "  " }, "participants[0].role"},
		{"плохая цитата", func(e *Event) { e.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
		{"пустая заметка", func(e *Event) { e.Notes = append(e.Notes, TextRef{}) }, "notes[1].text"},
	}
	for _, tt := range tests {
		e := validEvent()
		tt.mutate(e)
		wantInvalid(t, tt.name, e.Validate(), TypeEvent, tt.field)
	}
}

func validSource() *Source {
	return &Source{
		ID:           testID(TypeSource),
		Kind:         SourceKindArchivalScan,
		Title:        "МК с. Давыдово за 1881",
		Author:       "причт Никольской церкви",
		Date:         &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact},
		Reliability:  ReliabilityPrimary,
		RepositoryID: testID(TypeRepository),
		Notes:        []TextRef{{Text: "хорошая сохранность"}},
		Private:      true,
	}
}

func TestSourceValidate(t *testing.T) {
	if err := validSource().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	// Автор, дата, хранилище и заметки необязательны.
	min := &Source{ID: testID(TypeSource), Kind: SourceKindMemory, Title: "воспоминания бабушки", Reliability: ReliabilityUnknown}
	if err := min.Validate(); err != nil {
		t.Errorf("минимальный источник: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Source)
		field  string
	}{
		{"плохой id", func(s *Source) { s.ID = "" }, "id"},
		{"пустой вид", func(s *Source) { s.Kind = "" }, "kind"},
		{"неизвестный вид", func(s *Source) { s.Kind = "rumor" }, "kind"},
		{"пустое название", func(s *Source) { s.Title = "" }, "title"},
		{"название из пробелов", func(s *Source) { s.Title = " " }, "title"},
		{"плохая дата", func(s *Source) { s.Date = &FactDate{Year: 0, Precision: PrecisionYear, Modifier: ModifierExact} }, "date.year"},
		{"пустая достоверность", func(s *Source) { s.Reliability = "" }, "reliability"},
		{"неизвестная достоверность", func(s *Source) { s.Reliability = "maybe" }, "reliability"},
		{"хранилище — не хранилище", func(s *Source) { s.RepositoryID = testID(TypeArchive) }, "repository_id"},
		{"хранилище — плохой формат", func(s *Source) { s.RepositoryID = "R-1" }, "repository_id"},
		{"пустая заметка", func(s *Source) { s.Notes[0] = TextRef{} }, "notes[0].text"},
	}
	for _, tt := range tests {
		s := validSource()
		tt.mutate(s)
		wantInvalid(t, tt.name, s.Validate(), TypeSource, tt.field)
	}
}
