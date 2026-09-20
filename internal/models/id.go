package models

import (
	"errors"
	"fmt"
	"strings"
)

// ID — строгий идентификатор сущности и целевой строгой ссылки.
//
// Формат: ПРЕФИКС-ULID, например I-01J8X4T0K2M9Q7R5V3B6N8C1D4. Префикс
// кодирует тип сущности (см. Type.IDPrefix), ULID — 26 символов алфавита
// Crockford (заглавные, без I L O U).
type ID string

const (
	// idBodyLen — длина ULID в символах.
	idBodyLen = 26
	// idAlphabet — алфавит Crockford base32, заглавные, без I L O U.
	idAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

// ErrInvalidID — идентификатор нарушает формат ПРЕФИКС-ULID. Ошибки разбора и
// проверки оборачивают его (errors.Is).
var ErrInvalidID = errors.New("неверный формат идентификатора")

// validateIDBody проверяет ULID: длина, алфавит, первый символ не больше «7»
// (иначе не помещается 48 бит времени).
func validateIDBody(body string) error {
	if len(body) != idBodyLen {
		return fmt.Errorf("ULID должен содержать %d символов, получено %d", idBodyLen, len(body))
	}
	for i := 0; i < len(body); i++ {
		if !strings.ContainsRune(idAlphabet, rune(body[i])) {
			return fmt.Errorf("недопустимый символ %q в позиции %d", body[i], i)
		}
	}
	if body[0] > '7' {
		return fmt.Errorf("первый символ ULID %q больше «7»: переполнение времени", body[0])
	}

	return nil
}

// ParseID разбирает идентификатор вида ПРЕФИКС-ULID и возвращает тип
// сущности по префиксу.
func ParseID(id ID) (Type, error) {
	prefix, body, ok := strings.Cut(string(id), "-")
	if !ok {
		return "", fmt.Errorf("%w: %q — нет разделителя «-»", ErrInvalidID, string(id))
	}

	t, ok := TypeByIDPrefix(prefix)
	if !ok {
		return "", fmt.Errorf("%w: %q — неизвестный префикс %q", ErrInvalidID, string(id), prefix)
	}

	if err := validateIDBody(body); err != nil {
		return "", fmt.Errorf("%w: %q — %s", ErrInvalidID, string(id), err)
	}

	return t, nil
}

// Validate проверяет формат идентификатора и что префикс соответствует
// ожидаемому типу сущности.
func (id ID) Validate(want Type) error {
	got, err := ParseID(id)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%w: %q — префикс относится к типу %s, ожидается %s",
			ErrInvalidID, string(id), got, want)
	}

	return nil
}

// BuildID собирает идентификатор из типа сущности и готового ULID.
func BuildID(t Type, body string) (ID, error) {
	prefix := t.IDPrefix()
	if prefix == "" {
		return "", fmt.Errorf("%w: для типа %q нет префикса", ErrInvalidID, string(t))
	}
	if err := validateIDBody(body); err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalidID, err)
	}

	return ID(prefix + "-" + body), nil
}
