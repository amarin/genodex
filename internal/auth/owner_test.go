package auth

import (
	"errors"
	"testing"
	"time"
)

func validOwner() Owner {
	id, _ := newID(KindOwner)

	return Owner{ID: id, Login: "vladelec", PasswordHash: "hash", CreatedAt: time.Now()}
}

func TestOwnerValidate(t *testing.T) {
	if err := validOwner().Validate(); err != nil {
		t.Fatalf("валидный Owner не прошёл: %v", err)
	}

	bad := validOwner()
	bad.Login = "  "
	var ve *ValidationError
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "login" {
		t.Fatalf("пустой login: err = %v", err)
	}

	bad = validOwner()
	bad.PasswordHash = ""
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "password_hash" {
		t.Fatalf("пустой password_hash: err = %v", err)
	}

	bad = validOwner()
	bad.ID = "not-an-id"
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("неверный id: err = %v", err)
	}
}
