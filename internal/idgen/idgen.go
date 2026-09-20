// Package idgen генерирует идентификаторы сущностей формата ПРЕФИКС-ULID
// (см. models.ID). ULID — 48 бит времени в миллисекундах и 80 бит
// случайности в кодировке Crockford base32 (26 символов). В пределах одной
// миллисекунды энтропия увеличивается на единицу, поэтому идентификаторы
// одного генератора строго возрастают.
package idgen

import (
	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/amarin/genodex/internal/models"
)

// alphabet — алфавит Crockford base32 (заглавные, без I L O U).
const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Generator — потокобезопасный монотонный генератор идентификаторов.
type Generator struct {
	mu      sync.Mutex
	now     func() time.Time
	rand    io.Reader
	started bool
	lastMs  uint64
	entropy [10]byte
}

// New создаёт генератор на системных часах и crypto/rand.
func New() *Generator {
	return &Generator{now: time.Now, rand: rand.Reader}
}

// New возвращает новый идентификатор для сущности типа t. Паникует, если у типа
// нет префикса или источник случайности вернул ошибку: это ошибка программиста
// или среды, а не входных данных.
func (g *Generator) New(t models.Type) models.ID {
	// Тип проверяем до генерации, чтобы неизвестный тип не сдвигал состояние.
	if t.IDPrefix() == "" {
		panic(fmt.Sprintf("idgen: для типа %q нет префикса", string(t)))
	}

	id, err := models.BuildID(t, g.nextBody())
	if err != nil {
		panic("idgen: " + err.Error())
	}

	return id
}

// nextBody возвращает следующий ULID.
func (g *Generator) nextBody() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Время — 48-битное окно (1970 → ~год 10889); часы до 1970 не поддерживаются.
	ms := uint64(g.now().UnixMilli())
	if g.started && ms <= g.lastMs {
		// Та же миллисекунда (или часы ушли назад): сохраняем порядок,
		// увеличивая энтропию; при переполнении переходим к следующей
		// миллисекунде со свежей энтропией.
		ms = g.lastMs
		if !increment(&g.entropy) {
			ms++
			g.fill()
		}
	} else {
		g.fill()
	}
	g.started, g.lastMs = true, ms

	return encode(ms, g.entropy)
}

// fill заполняет энтропию случайными байтами.
func (g *Generator) fill() {
	if _, err := io.ReadFull(g.rand, g.entropy[:]); err != nil {
		panic(fmt.Sprintf("idgen: не удалось получить случайные байты: %v", err))
	}
}

// increment увеличивает 80-битное число (big-endian) на единицу; false — переполнение.
func increment(e *[10]byte) bool {
	for i := len(e) - 1; i >= 0; i-- {
		e[i]++
		if e[i] != 0 {
			return true
		}
	}

	return false
}

// encode кодирует 48 бит времени и 80 бит энтропии в 26 символов Crockford
// base32: 128 бит дополняются двумя нулями слева до 130 (26 по 5 бит).
func encode(ms uint64, entropy [10]byte) string {
	var raw [16]byte
	for i := 0; i < 6; i++ {
		raw[i] = byte(ms >> (8 * (5 - i)))
	}
	copy(raw[6:], entropy[:])

	var out [26]byte
	for i := range out {
		v := 0
		for b := 0; b < 5; b++ {
			v = v<<1 | bit(&raw, i*5+b-2)
		}
		out[i] = alphabet[v]
	}

	return string(out[:])
}

// bit возвращает n-й бит массива (0 — старший бит первого байта); n < 0 — ноль.
func bit(raw *[16]byte, n int) int {
	if n < 0 {
		return 0
	}

	return int(raw[n/8]>>(7-n%8)) & 1
}
