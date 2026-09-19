package storage

import "strconv"

// ParseUint64 парсит строку как uint64 (для meta-значений вроде applied_seq).
func ParseUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}