package models

import (
	"reflect"
	"testing"
)

func TestEventFullForm(t *testing.T) {
	d, _ := ParseFactDate("1881-03-15")
	e := Event{
		ID:    ID("ev-1"),
		Type:  EventTypeBirth,
		Date:  &d,
		Place: &PlaceRef{Text: "Давыдово"},
		Participants: []EventParticipant{
			{PersonID: ID("p-1"), Role: "subject"},
			{PersonID: ID("p-2"), Role: "mother"},
		},
	}
	if e.EntityType() != TypeEvent {
		t.Fatalf("EntityType() = %q", e.EntityType())
	}
	if e.Date.String() != "1881-03-15" {
		t.Fatalf("date lost: %s", e.Date)
	}
}

func TestSourceFullForm(t *testing.T) {
	s := Source{
		ID:           ID("s-1"),
		Kind:         SourceKindArchivalScan,
		Title:        "МК с. Давыдово за 1881",
		Reliability:  ReliabilityPrimary,
		RepositoryID: ID("r-1"),
		Private:      true,
	}
	if s.EntityType() != TypeSource {
		t.Fatalf("EntityType() = %q", s.EntityType())
	}
	if _, has := reflect.TypeOf(s).FieldByName("Anchor"); has {
		t.Fatalf("Source не несёт Anchor (решение #21)")
	}
}

func TestCitationFullForm(t *testing.T) {
	c := Citation{
		ID:       ID("c-1"),
		SourceID: ID("s-1"),
		Anchor:   &ArchiveAnchor{NodeID: ID("n-1"), Page: 12, Rect: "10,10,20,20"},
		Text:     "Акилина Иванова, 53 лет",
	}
	if c.EntityType() != TypeCitation {
		t.Fatalf("EntityType() = %q", c.EntityType())
	}
	if _, ok := c.Anchor.(*ArchiveAnchor); !ok {
		t.Fatalf("anchor kind lost: %T", c.Anchor)
	}
}
