package models

import "testing"

func TestValidateAnchor(t *testing.T) {
	valid := map[string]Anchor{
		"nil-интерфейс": nil,
		"архивный":      &ArchiveAnchor{NodeID: testID(TypeArchiveNode), Page: 12, Rect: "10,10,20,20"},
		"архивный с документом": &ArchiveAnchor{
			NodeID: testID(TypeArchiveNode), DocumentID: testID(TypeArchiveDocument), Page: 1,
		},
		"файл":          &FileAnchor{AttachmentID: testID(TypeAttachment)},
		"файл с меткой": &FileAnchor{AttachmentID: testID(TypeAttachment), Timecode: "00:12:30"},
		"url":           &URLAnchor{URL: "https://example.org/page?a=1#x"},
		"http":          &URLAnchor{URL: "http://example.org"},
	}
	for name, a := range valid {
		if e := validateAnchor(a); e != nil {
			t.Errorf("%s: %v", name, e)
		}
	}

	var nilArchive *ArchiveAnchor
	var nilFile *FileAnchor
	var nilURL *URLAnchor

	invalid := []struct {
		name  string
		a     Anchor
		field string
	}{
		{"nil *ArchiveAnchor", nilArchive, ""},
		{"nil *FileAnchor", nilFile, ""},
		{"nil *URLAnchor", nilURL, ""},
		{"архивный без узла", &ArchiveAnchor{Page: 1}, "node_id"},
		{"архивный узел не того типа", &ArchiveAnchor{NodeID: testID(TypeArchive), Page: 1}, "node_id"},
		{"архивный документ не того типа", &ArchiveAnchor{NodeID: testID(TypeArchiveNode), DocumentID: testID(TypeArchiveNode), Page: 1}, "document_id"},
		{"архивный page 0", &ArchiveAnchor{NodeID: testID(TypeArchiveNode)}, "page"},
		{"архивный page < 0", &ArchiveAnchor{NodeID: testID(TypeArchiveNode), Page: -3}, "page"},
		{"файл без вложения", &FileAnchor{}, "attachment_id"},
		{"файл вложение не того типа", &FileAnchor{AttachmentID: testID(TypeNote)}, "attachment_id"},
		{"url пустой", &URLAnchor{}, "url"},
		{"url без схемы", &URLAnchor{URL: "example.org/page"}, "url"},
		{"url ftp", &URLAnchor{URL: "ftp://example.org/file"}, "url"},
		{"url без хоста", &URLAnchor{URL: "https:///path"}, "url"},
		{"url мусор", &URLAnchor{URL: "http://exa mple.org"}, "url"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, finish("", validateAnchor(tt.a)), "", tt.field)
	}
}

// Неизвестная реализация интерфейса Anchor отвергается.
type foreignAnchor struct{}

func (foreignAnchor) Kind() AnchorKind { return "foreign" }

func TestValidateAnchorUnknownImplementation(t *testing.T) {
	wantInvalid(t, "чужая реализация", finish("", validateAnchor(foreignAnchor{})), "", "")
}

func validCitation() *Citation {
	return &Citation{
		ID:       testID(TypeCitation),
		SourceID: testID(TypeSource),
		Anchor:   &ArchiveAnchor{NodeID: testID(TypeArchiveNode), Page: 12, Rect: "10,10,20,20"},
		Text:     "Акилина Иванова, 53 лет",
		Note:     "запись 14",
		Private:  true,
	}
}

func TestCitationValidate(t *testing.T) {
	if err := validCitation().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Якорь и текст необязательны: цитата может ссылаться на источник целиком.
	if err := (&Citation{ID: testID(TypeCitation), SourceID: testID(TypeSource)}).Validate(); err != nil {
		t.Errorf("цитата без якоря и текста: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Citation)
		field  string
	}{
		{"плохой id", func(c *Citation) { c.ID = "c-1" }, "id"},
		{"id другого типа", func(c *Citation) { c.ID = testID(TypeSource) }, "id"},
		{"пустой source_id", func(c *Citation) { c.SourceID = "" }, "source_id"},
		{"source_id не источник", func(c *Citation) { c.SourceID = testID(TypeCitation) }, "source_id"},
		{"плохой якорь", func(c *Citation) { c.Anchor = &ArchiveAnchor{Page: 1} }, "anchor.node_id"},
		{"nil внутри якоря", func(c *Citation) { c.Anchor = (*URLAnchor)(nil) }, "anchor"},
		{"плохой url", func(c *Citation) { c.Anchor = &URLAnchor{URL: "nope"} }, "anchor.url"},
	}
	for _, tt := range tests {
		c := validCitation()
		tt.mutate(c)
		wantInvalid(t, tt.name, c.Validate(), TypeCitation, tt.field)
	}
}
