package transport

import (
	"testing"
	"time"

	"github.com/amarin/genodex/internal/auth"
)

func TestAuthSessionFromOwner(t *testing.T) {
	got := AuthSessionFromOwner(&auth.Owner{Login: "vladelec"})
	if got.Login != "vladelec" {
		t.Fatalf("Login = %q", got.Login)
	}
}

func TestAPITokensFromModelsEmptyIsEmptySlice(t *testing.T) {
	got := APITokensFromModels(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("got = %v, ожидался непустой указатель на пустой срез", got)
	}
}

func TestAPITokenFromModelOmitsHash(t *testing.T) {
	now := time.Now()
	got := APITokenFromModel(auth.APIToken{
		ID: "AT-1", Label: "MCP", TokenHash: "секрет-не-должен-попасть-в-dto",
		CreatedAt: now, LastUsedAt: &now,
	})
	if got.ID != "AT-1" || got.Label != "MCP" || got.LastUsedAt == nil {
		t.Fatalf("got = %+v", got)
	}
	// TokenHash отсутствует как поле DTO — компилятор уже это гарантирует
	// (нет способа его прочитать из got), тест фиксирует поведение конвертера.
}
