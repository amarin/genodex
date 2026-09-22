package auth

import (
	"crypto/rand"
	"fmt"
	"time"
)

// newID генерирует новый ID вида kind: 48 бит времени (мс) + 80 бит
// crypto/rand, Crockford base32. Не монотонен внутри миллисекунды —
// auth-сущности не нуждаются в строгом порядке по ID (см. пакетный
// комментарий выше).
func newID(kind Kind) (ID, error) {
	if _, ok := kindPrefix[kind]; !ok {
		return "", fmt.Errorf("%w: у вида %q нет префикса", ErrInvalidID, string(kind))
	}

	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("auth: не удалось получить случайные байты: %w", err)
	}

	ms := uint64(time.Now().UnixMilli())

	return buildID(kind, encodeULID(ms, entropy))
}

// encodeULID кодирует 48 бит времени и 80 бит энтропии в 26 символов
// Crockford base32: 128 бит дополняются двумя нулями слева до 130 (26 по 5
// бит). Копия internal/idgen.encode — та же арифметика, отдельная функция
// по решению «auth не зависит от idgen» (auth.md §2).
func encodeULID(ms uint64, entropy [10]byte) string {
	var raw [16]byte
	for i := 0; i < 6; i++ {
		raw[i] = byte(ms >> (8 * (5 - i)))
	}

	copy(raw[6:], entropy[:])

	var out [26]byte

	for i := range out {
		v := 0
		for b := 0; b < 5; b++ {
			v = v<<1 | ulidBit(&raw, i*5+b-2)
		}

		out[i] = idAlphabet[v]
	}

	return string(out[:])
}

func ulidBit(raw *[16]byte, n int) int {
	if n < 0 {
		return 0
	}

	return int(raw[n/8]>>(7-n%8)) & 1
}
