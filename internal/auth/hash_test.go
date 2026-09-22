package auth

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := hashPassword("верный-пароль")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if hash == "верный-пароль" {
		t.Fatal("хеш совпал с исходным паролем")
	}
	if !verifyPassword(hash, "верный-пароль") {
		t.Fatal("verifyPassword: верный пароль не прошёл")
	}
	if verifyPassword(hash, "неверный-пароль") {
		t.Fatal("verifyPassword: неверный пароль прошёл")
	}
}

func TestHashPasswordSaltsEachCall(t *testing.T) {
	h1, _ := hashPassword("пароль")
	h2, _ := hashPassword("пароль")
	if h1 == h2 {
		t.Fatal("два хеша одного пароля совпали — соль не применяется")
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	if hashToken("abc") != hashToken("abc") {
		t.Fatal("hashToken недетерминирован")
	}
	if hashToken("abc") == hashToken("abd") {
		t.Fatal("разные токены дали одинаковый хеш")
	}
}

func TestNewRawTokenUniqueAndFormat(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 1000; i++ {
		raw, err := newRawToken()
		if err != nil {
			t.Fatalf("newRawToken: %v", err)
		}
		if len(raw) == 0 {
			t.Fatal("пустой токен")
		}
		if _, dup := seen[raw]; dup {
			t.Fatalf("повтор токена %q", raw)
		}
		seen[raw] = struct{}{}
	}
}

func TestNewAPITokenHasPrefix(t *testing.T) {
	raw, err := newAPIToken()
	if err != nil {
		t.Fatalf("newAPIToken: %v", err)
	}
	if len(raw) <= len(apiTokenPrefix) || raw[:len(apiTokenPrefix)] != apiTokenPrefix {
		t.Fatalf("токен %q без префикса %q", raw, apiTokenPrefix)
	}
}
