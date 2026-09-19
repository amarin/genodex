package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// JournalEntry — одна запись журнала. Payload — ПОЛНЫЙ после-образ сущности
// (не диф), поэтому replay идемпотентен: повторная запись того же образа безопасна.
// Search хранится отдельно, чтобы replay восстановил поисковый индекс даже
// без знания модели (storage модель-агностичен).
type JournalEntry struct {
	Seq        uint64          `json:"seq"`
	Op         string          `json:"op"`
	EntityType string          `json:"entity,omitempty"`
	ID         string          `json:"id,omitempty"`
	At         string          `json:"at"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Search     []byte          `json:"search,omitempty"`
}

// Journal — append-only JSONL-журнал. Каждая Append делает fsync (Flush+Sync),
// поэтому при крахе журнал не младше подтверждённой записи.
type Journal struct {
	f   *os.File
	w   *bufio.Writer
	seq uint64
}

// OpenJournal открывает журнал в режиме O_APPEND и восстанавливает последний seq.
func OpenJournal(path string) (*Journal, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	j := &Journal{f: f, w: bufio.NewWriter(f)}
	if err := j.scanSeq(); err != nil {
		f.Close()
		return nil, err
	}
	return j, nil
}

func (j *Journal) scanSeq() error {
	dec := json.NewDecoder(j.f)
	for {
		var e JournalEntry
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("journal: decoder: %w", err)
		}
		if e.Seq > j.seq {
			j.seq = e.Seq
		}
	}
}

// Append инкрементирует seq и делает durable-запись (Flush + Sync).
func (j *Journal) Append(op, entityType, id string, payload json.RawMessage, search []byte) (JournalEntry, error) {
	j.seq++
	e := JournalEntry{
		Seq:        j.seq,
		Op:         op,
		EntityType: entityType,
		ID:         id,
		At:         time.Now().UTC().Format(time.RFC3339Nano),
		Payload:    payload,
		Search:     search,
	}
	line, err := json.Marshal(e)
	if err != nil {
		return JournalEntry{}, err
	}
	if _, err := j.w.Write(append(line, '\n')); err != nil {
		return JournalEntry{}, err
	}
	if err := j.w.Flush(); err != nil {
		return JournalEntry{}, err
	}
	if err := j.f.Sync(); err != nil {
		return JournalEntry{}, err
	}
	return e, nil
}

// LastSeq возвращает последний записанный seq.
func (j *Journal) LastSeq() uint64 { return j.seq }

// ReplayAll вызывает fn для каждой записи в порядке возрастания seq.
func (j *Journal) ReplayAll(fn func(JournalEntry) error) error {
	// перечитываем с начала файла; для масштаба ~30 MB это быстро
	if _, err := j.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	dec := json.NewDecoder(j.f)
	for {
		var e JournalEntry
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err := fn(e); err != nil {
			return err
		}
	}
}

// Close сбрасывает буфер и закрывает файл.
func (j *Journal) Close() error {
	if err := j.w.Flush(); err != nil {
		j.f.Close()
		return err
	}
	return j.f.Close()
}