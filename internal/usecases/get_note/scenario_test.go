package get_note

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	notes     map[models.ID]*models.Note
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) GetNote(_ context.Context, id models.ID) (*models.Note, error) {
	n, ok := f.notes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return n, nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestGetNoteReturnsRecord(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{notes: map[models.ID]*models.Note{id: {ID: id, Kind: "note", Text: "текст"}}}

	got, err := New(repo).GetNote(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}

	if got.Text != "текст" {
		t.Fatalf("Text = %q", got.Text)
	}
}

func TestGetNoteNotFound(t *testing.T) {
	repo := &fakeRepo{notes: map[models.ID]*models.Note{}}

	_, err := New(repo).GetNote(context.Background(), models.AccessFull, "N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetNoteInvalidID(t *testing.T) {
	repo := &fakeRepo{notes: map[models.ID]*models.Note{}}

	_, err := New(repo).GetNote(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetNotePrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{notes: map[models.ID]*models.Note{id: {ID: id, Kind: "note", Text: "текст", Private: true}}}

	_, err := New(repo).GetNote(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetNotePrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{notes: map[models.ID]*models.Note{id: {ID: id, Kind: "note", Text: "текст", Private: true}}}

	got, err := New(repo).GetNote(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}

	if got.Text != "текст" {
		t.Fatalf("Text = %q", got.Text)
	}
}

// TestGetNoteHiddenWhenReferencesPrivateCitation: заметка сама по себе
// публична, но ссылается (Sources[i].CitationID) на приватную цитату — для
// вызывающего без полного доступа она отдаётся как models.ErrNotFound, тот
// же принцип «прячем как отсутствующее», что и для собственного Private.
func TestGetNoteHiddenWhenReferencesPrivateCitation(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		notes: map[models.ID]*models.Note{
			id: {ID: id, Kind: "note", Text: "текст", Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	_, err := New(repo).GetNote(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (ссылка на приватную цитату)", err)
	}
}

// TestGetNoteVisibleWithFullAccessWhenReferencesPrivateCitation: та же
// заметка, но для вызывающего с полным доступом — видна.
func TestGetNoteVisibleWithFullAccessWhenReferencesPrivateCitation(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		notes: map[models.ID]*models.Note{
			id: {ID: id, Kind: "note", Text: "текст", Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).GetNote(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}

	if got.Text != "текст" {
		t.Fatalf("Text = %q", got.Text)
	}
}
