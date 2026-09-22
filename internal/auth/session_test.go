package auth

import (
	"errors"
	"testing"
	"time"
)

func validSession() Session {
	id, _ := newID(KindSession)
	ownerID, _ := newID(KindOwner)

	return Session{
		ID:               id,
		OwnerID:          ownerID,
		AccessTokenHash:  "access_hash",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshTokenHash: "refresh_hash",
		RefreshExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:        time.Now(),
	}
}

func TestSessionValidate(t *testing.T) {
	if err := validSession().Validate(); err != nil {
		t.Fatalf("валидный Session не прошёл: %v", err)
	}

	bad := validSession()
	bad.AccessTokenHash = ""
	var ve *ValidationError
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "access_token_hash" {
		t.Fatalf("пустой access_token_hash: err = %v", err)
	}

	bad = validSession()
	bad.RefreshTokenHash = ""
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "refresh_token_hash" {
		t.Fatalf("пустой refresh_token_hash: err = %v", err)
	}

	bad = validSession()
	bad.RefreshExpiresAt = bad.AccessExpiresAt
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "refresh_expires_at" {
		t.Fatalf("refresh_expires_at не позже access_expires_at: err = %v", err)
	}

	bad = validSession()
	bad.ID = "not-an-id"
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("неверный id: err = %v", err)
	}

	bad = validSession()
	bad.OwnerID = "not-an-id"
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "owner_id" {
		t.Fatalf("неверный owner_id: err = %v", err)
	}
}
