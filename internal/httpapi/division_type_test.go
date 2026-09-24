package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"testing/fstest"
)

func TestDivisionTypeListContract(t *testing.T) {
	rec := get(t, NewHandler(Deps{Divisions: &fakeDivisions{}, DocsFS: fstest.MapFS{}}), "/api/admin-division-types")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("тело не JSON-массив: %v", err)
	}

	byType := map[string]map[string]any{}
	for _, item := range got {
		byType[item["type"].(string)] = item
	}

	selo := byType["selo"]
	if selo["label"] != "Село" || selo["rank"] != float64(20) || selo["settlement"] != true {
		t.Errorf("selo = %v", selo)
	}
	if other := byType["other"]; other["rank"] != nil {
		t.Errorf("other.rank = %v, ожидался null", other["rank"])
	}
	if hutor := byType["hutor"]; hutor["children"] == nil {
		t.Errorf("hutor.children = null, ожидался пустой массив")
	}
}
