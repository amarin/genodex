package auth

import (
	"errors"
	"testing"
	"time"
)

func validInvite() Invite {
	id, _ := newID(KindInvite)
	createdBy, _ := newID(KindOwner)

	return Invite{
		ID:        id,
		CreatedBy: createdBy,
		TokenHash: "token_hash",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}
}

func TestInviteValidate(t *testing.T) {
	if err := validInvite().Validate(); err != nil {
		t.Fatalf("валидный Invite не прошёл: %v", err)
	}

	bad := validInvite()
	bad.TokenHash = ""
	var ve *ValidationError
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "token_hash" {
		t.Fatalf("пустой token_hash: err = %v", err)
	}

	bad = validInvite()
	bad.ExpiresAt = time.Time{}
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "expires_at" {
		t.Fatalf("нулевой expires_at: err = %v", err)
	}

	bad = validInvite()
	bad.ID = "not-an-id"
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("неверный id: err = %v", err)
	}

	bad = validInvite()
	bad.CreatedBy = "not-an-id"
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "created_by" {
		t.Fatalf("неверный created_by: err = %v", err)
	}
}
