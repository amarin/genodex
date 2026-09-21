package models

import "testing"

func validNote() *Note {
	return &Note{
		ID:       testID(TypeNote),
		Kind:     NoteKindChapter,
		Title:    "Глава 1",
		Text:     "# Начало\n\nтекст",
		ParentID: idPtr(noteBID()),
		Sources:  []SourceLink{{CitationID: testID(TypeCitation)}},
		Private:  true,
	}
}

// noteBID — другой валидный идентификатор заметки (иное тело ULID).
func noteBID() ID {
	id, err := BuildID(TypeNote, "01J8X4T0K2M9Q7R5V3B6N8C1D5")
	if err != nil {
		panic(err)
	}

	return id
}

func TestNoteValidate(t *testing.T) {
	if err := validNote().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Книга-контейнер: только заголовок, текст — в главах.
	book := &Note{ID: testID(TypeNote), Kind: NoteKindBook, Title: "Родословная книга"}
	if err := book.Validate(); err != nil {
		t.Errorf("книга только с заголовком: %v", err)
	}
	// Заметка без заголовка, только текст.
	if err := (&Note{ID: testID(TypeNote), Kind: NoteKindNote, Text: "выписка"}).Validate(); err != nil {
		t.Errorf("заметка только с текстом: %v", err)
	}
	// Открытый enum.
	if err := (&Note{ID: testID(TypeNote), Kind: "letter", Text: "x"}).Validate(); err != nil {
		t.Errorf("свой вид заметки: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Note)
		field  string
	}{
		{"плохой id", func(n *Note) { n.ID = "n-1" }, "id"},
		{"пустой вид", func(n *Note) { n.Kind = "" }, "kind"},
		{"вид не в формате", func(n *Note) { n.Kind = "Chapter" }, "kind"},
		{"ни заголовка, ни текста", func(n *Note) { n.Title, n.Text = "", "" }, "text"},
		{"заголовок и текст из пробелов", func(n *Note) { n.Title, n.Text = " ", "\n" }, "text"},
		{"родитель не заметка", func(n *Note) { n.ParentID = idPtr(testID(TypeSource)) }, "parent_id"},
		{"родитель — сам себе", func(n *Note) { n.ParentID = idPtr(n.ID) }, "parent_id"},
		{"плохая цитата", func(n *Note) { n.Sources[0].CitationID = "" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		n := validNote()
		tt.mutate(n)
		wantInvalid(t, tt.name, n.Validate(), TypeNote, tt.field)
	}
}

func validRepository() *Repository {
	return &Repository{
		ID:      testID(TypeRepository),
		Name:    "ГАКО",
		Type:    RepositoryTypeArchive,
		Address: "г. Калуга, ул. Ленина, 1",
		URLs:    []TextRef{{Text: "https://gako.example.org"}},
		Notes:   []TextRef{{Text: "читальный зал по записи"}},
		Sources: []SourceLink{{CitationID: testID(TypeCitation)}},
		Private: false,
	}
}

func TestRepositoryValidate(t *testing.T) {
	if err := validRepository().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	if err := (&Repository{ID: testID(TypeRepository), Name: "дом", Type: "family-home"}).Validate(); err != nil {
		t.Errorf("свой тип хранилища: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Repository)
		field  string
	}{
		{"плохой id", func(r *Repository) { r.ID = "R" }, "id"},
		{"пустое название", func(r *Repository) { r.Name = "" }, "name"},
		{"пустой тип", func(r *Repository) { r.Type = "" }, "type"},
		{"тип не в формате", func(r *Repository) { r.Type = "Archive" }, "type"},
		{"пустая ссылка", func(r *Repository) { r.URLs = append(r.URLs, TextRef{}) }, "urls[1].text"},
		{"пустая заметка", func(r *Repository) { r.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(r *Repository) { r.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		r := validRepository()
		tt.mutate(r)
		wantInvalid(t, tt.name, r.Validate(), TypeRepository, tt.field)
	}
}

func validArchive() *Archive {
	return &Archive{
		ID:           testID(TypeArchive),
		Name:         "ГАКО",
		System:       &TextRef{Text: "Фонды/Описи/Дела"},
		RepositoryID: testID(TypeRepository),
		Notes:        []TextRef{{Text: "фонд 33"}},
		Sources:      []SourceLink{{CitationID: testID(TypeCitation)}},
		Private:      true,
	}
}

func TestArchiveValidate(t *testing.T) {
	if err := validArchive().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	// Система и хранилище необязательны.
	if err := (&Archive{ID: testID(TypeArchive), Name: "домашний архив"}).Validate(); err != nil {
		t.Errorf("минимальный архив: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Archive)
		field  string
	}{
		{"плохой id", func(a *Archive) { a.ID = "" }, "id"},
		{"пустое название", func(a *Archive) { a.Name = "" }, "name"},
		{"система — пустой TextRef", func(a *Archive) { a.System = &TextRef{} }, "system.text"},
		{"система — только пробелы", func(a *Archive) { a.System = &TextRef{Text: " "} }, "system.text"},
		{"система со ссылкой", func(a *Archive) {
			a.System = &TextRef{Text: "Фонды/Описи/Дела", Ref: testID(TypeArchive), Type: TypeArchive}
		}, "system.ref"},
		{"система: только тип", func(a *Archive) { a.System = &TextRef{Text: "x", Type: TypeArchive} }, "system.type"},
		{"система: только ссылка", func(a *Archive) { a.System = &TextRef{Text: "x", Ref: testID(TypeArchive)} }, "system.ref"},
		{"хранилище — не хранилище", func(a *Archive) { a.RepositoryID = testID(TypeSource) }, "repository_id"},
		{"пустая заметка", func(a *Archive) { a.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(a *Archive) { a.Sources[0].CitationID = "c" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		a := validArchive()
		tt.mutate(a)
		wantInvalid(t, tt.name, a.Validate(), TypeArchive, tt.field)
	}
}
