package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// VerifyResult — детали проверки.
type VerifyResult struct {
	OK              bool
	Integrity       error
	ReplayedSeq     uint64
	AppliedSeq      uint64
	EntitiesByType  map[string]int
	JournalEntries  int
	ManifestMatches bool
}

// Verify проверяет целостность журнала и БД (без записи) и, если задан
// каталог бандла, свежесть копии бэкапа.
func Verify(s *Storage, backupDir string) (*VerifyResult, error) {
	res := &VerifyResult{EntitiesByType: map[string]int{}}
	res.Integrity = s.db.IntegrityCheck()
	if res.Integrity != nil {
		return res, nil // результат «бито», но вернём детали
	}

	// перечитываем журнал целиком и считаем записи (не меняя applied_seq)
	count := 0
	var last uint64
	err := s.journal.ReplayAll(func(e JournalEntry) error {
		count++
		last = e.Seq
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("journal: %w", err)
	}
	res.JournalEntries = count
	res.ReplayedSeq = last
	res.AppliedSeq, err = s.db.AppliedSeq()
	if err != nil {
		return nil, err
	}

	if backupDir != "" {
		m, err := ReadManifest(backupDir)
		if err != nil {
			return nil, fmt.Errorf("manifest: %w", err)
		}
		ok := true
		if got, err := fileSHA(filepath.Join(backupDir, m.SnapshotFile)); err != nil {
			return nil, err
		} else if got != m.SnapshotSHA {
			ok = false
		}
		if got, err := fileSHA(filepath.Join(backupDir, m.JournalFile)); err != nil {
			return nil, err
		} else if got != m.JournalSHA {
			ok = false
		}
		for et, want := range m.EntityCounts {
			n, err := s.db.Count(et)
			if err != nil {
				return nil, err
			}
			res.EntitiesByType[et] = n
			if n != want {
				ok = false
			}
		}
		res.ManifestMatches = ok
	}

	res.OK = res.Integrity == nil && (backupDir == "" || res.ManifestMatches)
	return res, nil
}

func (r *VerifyResult) String() string {
	return fmt.Sprintf(
		"verify ok=%v journal=%d replayed=%d applied=%d manifest_match=%v",
		r.OK, r.JournalEntries, r.ReplayedSeq, r.AppliedSeq, r.ManifestMatches,
	)
}

// writeGarbage пишет мусор в файл (для теста порчи бандла).
func writeGarbage(path string, size int) error {
	data := make([]byte, size)
	return os.WriteFile(path, data, 0o644)
}