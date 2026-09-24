package list_churches

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// fakeRepo отдаёт окна списка как настоящий репозиторий и запоминает вызовы.
type fakeRepo struct {
	list      []*models.Church
	err       error
	errAt     int // номер вызова (с 1), на котором возвращается err; 0 — на любом
	calls     []models.Page
	citations map[models.ID]*models.Citation
}

// window нарезает список по окну, как настоящий репозиторий.
func window(list []*models.Church, page models.Page) []*models.Church {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListChurches(_ context.Context, _ models.Access, page models.Page) ([]*models.Church, error) {
	f.calls = append(f.calls, page)

	if f.err != nil && (f.errAt == 0 || f.errAt == len(f.calls)) {
		return nil, f.err
	}

	return window(f.list, page), nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestListChurchesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{list: []*models.Church{{ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Никольская церковь"}}}

	got, err := New(repo).ListChurches(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListChurches: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольская церковь" {
		t.Fatalf("got = %+v", got)
	}
}

func TestListChurchesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListChurches(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}

func TestListChurchesRejectsNegativeOffset(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListChurches(context.Background(), models.AccessFull, models.Page{Offset: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "offset" {
		t.Fatalf("err = %v, want ValidationError on offset", err)
	}
}

func TestListChurchesEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListChurches(context.Background(), models.AccessFull, models.Page{})
	if err != nil {
		t.Fatalf("ListChurches: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListChurchesPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListChurches(context.Background(), models.AccessFull, models.Page{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestListChurchesHidesReferenceToPrivateCitation: запись, ссылающаяся на
// приватную цитату среди источников, исключается из результата для
// вызывающего без полного доступа, но видна с AccessFull.
func TestListChurchesHidesReferenceToPrivateCitation(t *testing.T) {
	citID := models.ID("CI-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	hidden := &models.Church{ID: "CH-hidden", Name: "Скрытая церковь", Sources: []models.SourceLink{{CitationID: citID}}}
	visible := &models.Church{ID: "CH-visible", Name: "Видимая церковь"}

	repo := &fakeRepo{
		list:      []*models.Church{hidden, visible},
		citations: map[models.ID]*models.Citation{citID: {ID: citID, Private: true}},
	}

	got, err := New(repo).ListChurches(context.Background(), models.AccessPublic, models.Page{})
	if err != nil || len(got) != 1 || got[0].ID != visible.ID {
		t.Fatalf("got %+v, %v; ожидалась только видимая запись", got, err)
	}

	got, err = New(repo).ListChurches(context.Background(), models.AccessFull, models.Page{})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v; с AccessFull ожидались обе записи", got, err)
	}
}

// TestListChurchesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden: запись,
// скрытая приватной цитатой и предшествующая запрошенному offset, не должна
// «съедать» offset-бюджет вслепую — постранично (limit=1) с offset=0 и
// offset=1 должны вернуться разные видимые записи, без дублей и пропусков.
func TestListChurchesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden(t *testing.T) {
	citID := models.ID("CI-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	hidden := &models.Church{ID: "CH-hidden", Name: "Скрытая церковь", Sources: []models.SourceLink{{CitationID: citID}}}
	a := &models.Church{ID: "CH-a", Name: "A"}
	b := &models.Church{ID: "CH-b", Name: "B"}

	repo := &fakeRepo{
		list:      []*models.Church{hidden, a, b},
		citations: map[models.ID]*models.Citation{citID: {ID: citID, Private: true}},
	}

	page1, err := New(repo).ListChurches(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 0})
	if err != nil || len(page1) != 1 || page1[0].ID != a.ID {
		t.Fatalf("page1 = %+v, %v; want [%v]", page1, err, a.ID)
	}

	page2, err := New(repo).ListChurches(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 1})
	if err != nil || len(page2) != 1 || page2[0].ID != b.ID {
		t.Fatalf("page2 = %+v, %v; want [%v]", page2, err, b.ID)
	}
}

// TestListChurchesWalksAllRepositoryWindows: записи за первым окном
// репозитория не теряются, окна запрашиваются подряд — каждая вторая запись
// скрыта приватной цитатой, поэтому одного окна репозитория (по числу равного
// размеру окна запроса) не хватает, чтобы заполнить окно запроса.
func TestListChurchesWalksAllRepositoryWindows(t *testing.T) {
	total := 2*models.MaxPageLimit + 7
	citID := models.ID("CI-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{citations: map[models.ID]*models.Citation{citID: {ID: citID, Private: true}}}
	for i := 0; i < total; i++ {
		c := &models.Church{ID: models.ID("CH-" + strconv.Itoa(i)), Name: "N"}
		if i%2 == 1 {
			c.Sources = []models.SourceLink{{CitationID: citID}} // скрыта под AccessPublic
		}

		repo.list = append(repo.list, c)
	}

	got, err := New(repo).ListChurches(context.Background(), models.AccessPublic, models.Page{Limit: models.MaxPageLimit})
	if err != nil || len(got) != models.MaxPageLimit {
		t.Fatalf("got %d, %v; ожидалось %d", len(got), err, models.MaxPageLimit)
	}

	if len(repo.calls) != 2 {
		t.Fatalf("вызовов репозитория %d (%v), ожидалось 2", len(repo.calls), repo.calls)
	}
}
