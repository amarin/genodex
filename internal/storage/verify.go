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
	EntitiesByType  map[string]int
	ManifestMatches bool
}

// Verify проверяет целостность БД (без записи) и, если задан каталог бандла,
// свежесть копии бэкапа (снапшот и манифест).
func Verify(s *Storage, backupDir string) (*VerifyResult, error) {
	res := &VerifyResult{EntitiesByType: map[string]int{}}
	res.Integrity = s.db.IntegrityCheck()
	if res.Integrity != nil {
		return res, nil // результат «бито», но вернём детали
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
		"verify ok=%v manifest_match=%v entities=%d",
		r.OK, r.ManifestMatches, len(r.EntitiesByType),
	)
}

// writeGarbage пишет мусор в файл (для теста порчи бандла).
func writeGarbage(path string, size int) error {
	data := make([]byte, size)
	return os.WriteFile(path, data, 0o644)
}
