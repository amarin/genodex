package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// apiTokenPrefix — узнаваемый префикс сырого значения API-токена (как
// GitHub/Stripe), только для читаемости — на проверку не влияет.
const apiTokenPrefix = "gnx_"

// hashPassword хеширует пароль bcrypt (стоимость по умолчанию — медленно
// намеренно).
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: не удалось хешировать пароль: %w", err)
	}

	return string(hash), nil
}

// verifyPassword сравнивает пароль с bcrypt-хешем; ошибка сравнения (в т.ч.
// неверный пароль) — false, без деталей причины.
func verifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// hashToken — SHA-256 сырого токена в hex: для хранения и поиска в БД.
// Токен уже высокоэнтропийный (32 байта crypto/rand) — быстрый хеш
// достаточен, в отличие от пароля.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}

// newRawToken возвращает случайную строку: 32 байта crypto/rand в base64url
// без паддинга.
func newRawToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("auth: не удалось получить случайные байты: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// newAPIToken — как newRawToken, с префиксом apiTokenPrefix.
func newAPIToken() (string, error) {
	raw, err := newRawToken()
	if err != nil {
		return "", err
	}

	return apiTokenPrefix + raw, nil
}
