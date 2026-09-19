package storage

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestJournalAppendReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	e1, err := j.Append("save", "person", "blohin", json.RawMessage(`{"surname":"Блохин"}`), []byte("blohin"))
	if err != nil {
		t.Fatal(err)
	}
	if e1.Seq != 1 {
		t.Fatalf("first Seq = %d, want 1", e1.Seq)
	}
	if _, err := j.Append("save", "person", "dorozhkin", json.RawMessage(`{"surname":"Дорожкин"}`), []byte("dorozhkin")); err != nil {
		t.Fatal(err)
	}
	if j.LastSeq() != 2 {
		t.Fatalf("LastSeq = %d, want 2", j.LastSeq())
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	j2, err := OpenJournal(path) // повторное открытие: seq восстанавливается сканированием
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	if j2.LastSeq() != 2 {
		t.Fatalf("reopen LastSeq = %d, want 2", j2.LastSeq())
	}
	var got []JournalEntry
	if err := j2.ReplayAll(func(e JournalEntry) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].ID != "dorozhkin" {
		t.Fatalf("replay = %+v", got)
	}
}

func TestJournalAppendIdempotentSeq(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	for _, id := range []string{"a", "b", "c"} {
		if _, err := j.Append("save", "person", id, json.RawMessage(`{"x":1}`), nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := j.Append("delete", "person", "a", nil, nil); err != nil {
		t.Fatal(err)
	}
	if j.LastSeq() != 4 {
		t.Fatalf("LastSeq = %d, want 4", j.LastSeq())
	}
}
