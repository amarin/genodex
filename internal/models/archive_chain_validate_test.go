package models

import "testing"

// archiveNodeBID — другой валидный идентификатор узла архива (иное тело ULID).
func archiveNodeBID() ID {
	id, err := BuildID(TypeArchiveNode, "01J8X4T0K2M9Q7R5V3B6N8C1D5")
	if err != nil {
		panic(err)
	}

	return id
}

func validArchiveNode() *ArchiveNode {
	return &ArchiveNode{
		ID:          testID(TypeArchiveNode),
		Type:        "case",
		ArchiveID:   testID(TypeArchive),
		ParentID:    idPtr(archiveNodeBID()),
		Label:       "дело 264",
		Name:        "Метрические книги с. Давыдово",
		Since:       &FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierExact},
		Until:       &FactDate{Year: 1890, Precision: PrecisionYear, Modifier: ModifierExact},
		Parish:      &TextRef{Text: "Никольский приход", Ref: testID(TypeParish), Type: TypeParish},
		Settlements: []TextRef{{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision}},
		Notes:       []TextRef{{Text: "подшито"}},
		Sources:     []SourceLink{{CitationID: testID(TypeCitation)}},
		Private:     true,
	}
}

func TestArchiveNodeValidate(t *testing.T) {
	if err := validArchiveNode().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Промежуточный уровень: только тип, архив и шифр.
	fond := &ArchiveNode{ID: testID(TypeArchiveNode), Type: "fond", ArchiveID: testID(TypeArchive), Label: "ф. 33"}
	if err := fond.Validate(); err != nil {
		t.Errorf("минимальный узел: %v", err)
	}

	past := &FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierExact}
	future := &FactDate{Year: 1890, Precision: PrecisionYear, Modifier: ModifierExact}

	tests := []struct {
		name   string
		mutate func(*ArchiveNode)
		field  string
	}{
		{"плохой id", func(n *ArchiveNode) { n.ID = "an-1" }, "id"},
		{"пустой уровень", func(n *ArchiveNode) { n.Type = "" }, "type"},
		{"уровень не в формате", func(n *ArchiveNode) { n.Type = "Дело" }, "type"},
		{"пустой archive_id", func(n *ArchiveNode) { n.ArchiveID = "" }, "archive_id"},
		{"archive_id — не архив", func(n *ArchiveNode) { n.ArchiveID = testID(TypeRepository) }, "archive_id"},
		{"родитель не узел", func(n *ArchiveNode) { n.ParentID = idPtr(testID(TypeArchive)) }, "parent_id"},
		{"родитель — сам себе", func(n *ArchiveNode) { n.ParentID = idPtr(n.ID) }, "parent_id"},
		{"пустой шифр", func(n *ArchiveNode) { n.Label = "" }, "label"},
		{"шифр из пробелов", func(n *ArchiveNode) { n.Label = "  " }, "label"},
		{"начало позже конца", func(n *ArchiveNode) { n.Since, n.Until = future, past }, "since"},
		{"плохое начало", func(n *ArchiveNode) { n.Since = &FactDate{} }, "since.precision"},
		{"приход — не приход", func(n *ArchiveNode) { n.Parish = &TextRef{Ref: testID(TypeChurch), Type: TypeChurch} }, "parish.type"},
		{"населённый пункт — не деление", func(n *ArchiveNode) {
			n.Settlements[0] = TextRef{Ref: testID(TypeParish), Type: TypeParish}
		}, "settlements[0].type"},
		{"пустая заметка", func(n *ArchiveNode) { n.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(n *ArchiveNode) { n.Sources[0].CitationID = "" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		n := validArchiveNode()
		tt.mutate(n)
		wantInvalid(t, tt.name, n.Validate(), TypeArchiveNode, tt.field)
	}
}

func validArchiveDocument() *ArchiveDocument {
	return &ArchiveDocument{
		ID:          testID(TypeArchiveDocument),
		UnitID:      testID(TypeArchiveNode),
		Title:       "Метрическая книга Николаевской ц. с. Давыдово за 1880–1882",
		Kind:        "метрическая книга",
		Since:       &FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierExact},
		Until:       &FactDate{Year: 1882, Precision: PrecisionYear, Modifier: ModifierExact},
		Parish:      &TextRef{Text: "Никольский приход", Ref: testID(TypeParish), Type: TypeParish},
		Settlements: []TextRef{{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision}},
		Notes:       []TextRef{{Text: "книга за три года"}},
		Sources:     []SourceLink{{CitationID: testID(TypeCitation)}},
		Private:     true,
	}
}

func TestArchiveDocumentValidate(t *testing.T) {
	if err := validArchiveDocument().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	// Вид — свободный текст и необязателен.
	min := &ArchiveDocument{ID: testID(TypeArchiveDocument), UnitID: testID(TypeArchiveNode), Title: "карточка 12"}
	if err := min.Validate(); err != nil {
		t.Errorf("минимальный документ: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ArchiveDocument)
		field  string
	}{
		{"плохой id", func(d *ArchiveDocument) { d.ID = "" }, "id"},
		{"пустой unit_id", func(d *ArchiveDocument) { d.UnitID = "" }, "unit_id"},
		{"unit_id — не узел", func(d *ArchiveDocument) { d.UnitID = testID(TypeArchive) }, "unit_id"},
		{"пустое название", func(d *ArchiveDocument) { d.Title = "" }, "title"},
		{"название из пробелов", func(d *ArchiveDocument) { d.Title = "\t" }, "title"},
		{"начало позже конца", func(d *ArchiveDocument) { d.Since, d.Until = d.Until, d.Since }, "since"},
		{"приход — не приход", func(d *ArchiveDocument) { d.Parish = &TextRef{Ref: testID(TypeChurch), Type: TypeChurch} }, "parish.type"},
		{"населённый пункт — не деление", func(d *ArchiveDocument) {
			d.Settlements[0] = TextRef{Ref: testID(TypeChurch), Type: TypeChurch}
		}, "settlements[0].type"},
		{"пустая заметка", func(d *ArchiveDocument) { d.Notes = append(d.Notes, TextRef{}) }, "notes[1].text"},
		{"плохая цитата", func(d *ArchiveDocument) { d.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		d := validArchiveDocument()
		tt.mutate(d)
		wantInvalid(t, tt.name, d.Validate(), TypeArchiveDocument, tt.field)
	}
}

func validAttachment() *Attachment {
	return &Attachment{
		ID:         testID(TypeAttachment),
		Kind:       AttachmentKindScan,
		URI:        "file:///archive/gako/f33/o6/d264/0012.jpg",
		Filename:   "0012.jpg",
		MIME:       "image/jpeg",
		Page:       12,
		NodeID:     testID(TypeArchiveNode),
		DocumentID: idPtr(testID(TypeArchiveDocument)),
		Note:       "скан разворота",
		Private:    true,
	}
}

func TestAttachmentValidate(t *testing.T) {
	if err := validAttachment().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	// Достаточно uri или filename; страница, документ и MIME необязательны.
	byURI := &Attachment{ID: testID(TypeAttachment), Kind: AttachmentKindAudio, URI: "https://example.org/a.mp3", NodeID: testID(TypeArchiveNode)}
	if err := byURI.Validate(); err != nil {
		t.Errorf("только uri: %v", err)
	}
	byName := &Attachment{ID: testID(TypeAttachment), Kind: AttachmentKindPhoto, Filename: "photo.png", NodeID: testID(TypeArchiveNode)}
	if err := byName.Validate(); err != nil {
		t.Errorf("только filename: %v", err)
	}
	withParams := validAttachment()
	withParams.MIME = "text/plain; charset=utf-8"
	if err := withParams.Validate(); err != nil {
		t.Errorf("MIME с параметрами: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Attachment)
		field  string
	}{
		{"плохой id", func(a *Attachment) { a.ID = "att" }, "id"},
		{"пустой вид", func(a *Attachment) { a.Kind = "" }, "kind"},
		{"неизвестный вид", func(a *Attachment) { a.Kind = "video" }, "kind"},
		{"ни uri, ни filename", func(a *Attachment) { a.URI, a.Filename = "", "" }, "uri"},
		{"uri и filename из пробелов", func(a *Attachment) { a.URI, a.Filename = " ", " " }, "uri"},
		{"MIME без слэша", func(a *Attachment) { a.MIME = "image" }, "mime"},
		{"MIME с пробелом", func(a *Attachment) { a.MIME = "image /jpeg" }, "mime"},
		{"отрицательная страница", func(a *Attachment) { a.Page = -1 }, "page"},
		{"пустой node_id", func(a *Attachment) { a.NodeID = "" }, "node_id"},
		{"node_id — не узел", func(a *Attachment) { a.NodeID = testID(TypeArchive) }, "node_id"},
		{"document_id — не документ", func(a *Attachment) { a.DocumentID = idPtr(testID(TypeArchiveNode)) }, "document_id"},
		{"указатель на пустой document_id", func(a *Attachment) { a.DocumentID = idPtr("") }, "document_id"},
	}
	for _, tt := range tests {
		a := validAttachment()
		tt.mutate(a)
		wantInvalid(t, tt.name, a.Validate(), TypeAttachment, tt.field)
	}
}
