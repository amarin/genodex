package auth

import (
	"errors"
	"testing"
	"time"
)

func validAPIToken() APIToken {
	id, _ := newID(KindAPIToken)
	ownerID, _ := newID(KindOwner)

	return APIToken{
		ID:        id,
		OwnerID:   ownerID,
		Label:     "test-token",
		TokenHash: "token_hash",
		CreatedAt: time.Now(),
	}
}

func TestAPITokenValidate(t *testing.T) {
	if err := validAPIToken().Validate(); err != nil {
		t.Fatalf("валидный APIToken не прошёл: %v", err)
	}

	bad := validAPIToken()
	bad.TokenHash = ""
	var ve *ValidationError
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "token_hash" {
		t.Fatalf("пустой token_hash: err = %v", err)
	}

	bad = validAPIToken()
	bad.ID = "not-an-id"
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("неверный id: err = %v", err)
	}

	bad = validAPIToken()
	bad.OwnerID = "not-an-id"
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "owner_id" {
		t.Fatalf("неверный owner_id: err = %v", err)
	}
}
