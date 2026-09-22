// Package auth реализует владельцев, сессии, API-токены и invite-ссылки —
// независимый от internal/models домен (см. docs/data-model/auth.md).
package auth

import (
	"errors"
	"fmt"
	"strings"
)

// Kind — вид auth-сущности, кодируется префиксом ID.
type Kind string

const (
	KindOwner    Kind = "owner"
	KindSession  Kind = "session"
	KindAPIToken Kind = "api_token"
	KindInvite   Kind = "invite"
)

// kindPrefix — префиксы ID; не пересекаются с models.idPrefixTable (auth и
// models — независимые ID-пространства, совпадение префиксов не опасно, но
// не проверяется и не должно проверяться намеренно).
var kindPrefix = map[Kind]string{
	KindOwner:    "OW",
	KindSession:  "SS",
	KindAPIToken: "AT",
	KindInvite:   "IV",
}

// ID — идентификатор auth-сущности, формат ПРЕФИКС-ULID (как models.ID:
// 26-символьный ULID Crockford base32, заглавные, без I L O U).
type ID string

const (
	idBodyLen  = 26
	idAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

// ErrInvalidID — идентификатор нарушает формат ПРЕФИКС-ULID.
var ErrInvalidID = errors.New("неверный формат идентификатора")

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

// parseID разбирает идентификатор вида ПРЕФИКС-ULID и возвращает вид по префиксу.
func parseID(id ID) (Kind, error) {
	prefix, body, ok := strings.Cut(string(id), "-")
	if !ok {
		return "", fmt.Errorf("%w: %q — нет разделителя «-»", ErrInvalidID, string(id))
	}

	var kind Kind

	found := false

	for k, p := range kindPrefix {
		if p == prefix {
			kind, found = k, true

			break
		}
	}

	if !found {
		return "", fmt.Errorf("%w: %q — неизвестный префикс %q", ErrInvalidID, string(id), prefix)
	}

	if err := validateIDBody(body); err != nil {
		return "", fmt.Errorf("%w: %q — %s", ErrInvalidID, string(id), err)
	}

	return kind, nil
}

// Validate проверяет формат идентификатора и что префикс соответствует
// ожидаемому виду.
func (id ID) Validate(want Kind) error {
	got, err := parseID(id)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%w: %q — префикс относится к виду %s, ожидается %s",
			ErrInvalidID, string(id), got, want)
	}

	return nil
}

// buildID собирает идентификатор из вида и готового ULID.
func buildID(kind Kind, body string) (ID, error) {
	prefix, ok := kindPrefix[kind]
	if !ok {
		return "", fmt.Errorf("%w: у вида %q нет префикса", ErrInvalidID, string(kind))
	}
	if err := validateIDBody(body); err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalidID, err)
	}

	return ID(prefix + "-" + body), nil
}
