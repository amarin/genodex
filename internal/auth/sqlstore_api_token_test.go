package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testAPIToken(id, ownerID ID) APIToken {
	now := time.Now().UTC().Truncate(time.Second)
	return APIToken{
		ID:         id,
		OwnerID:    ownerID,
		Label:      "test-token-" + string(id),
		TokenHash:  "hash-" + string(id),
		CreatedAt:  now,
		LastUsedAt: nil,
		RevokedAt:  nil,
	}
}

func TestSQLStoreCreateAndGetAPIToken(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// Create owner first (FK requirement)
	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	// Create API token
	tokenID, _ := newID(KindAPIToken)
	want := testAPIToken(tokenID, ownerID)

	if err := s.CreateAPIToken(ctx, want); err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	// Get by ID
	byID, err := s.GetAPIToken(ctx, tokenID)
	if err != nil {
		t.Fatalf("GetAPIToken: %v", err)
	}
	if *byID != want {
		t.Fatalf("GetAPIToken = %+v, want %+v", *byID, want)
	}
	// Verify pointers are nil
	if byID.LastUsedAt != nil {
		t.Fatalf("LastUsedAt should be nil, got %v", byID.LastUsedAt)
	}
	if byID.RevokedAt != nil {
		t.Fatalf("RevokedAt should be nil, got %v", byID.RevokedAt)
	}

	// Get by hash
	byHash, err := s.GetAPITokenByHash(ctx, want.TokenHash)
	if err != nil {
		t.Fatalf("GetAPITokenByHash: %v", err)
	}
	if *byHash != want {
		t.Fatalf("GetAPITokenByHash = %+v, want %+v", *byHash, want)
	}
	// Verify pointers are nil
	if byHash.LastUsedAt != nil {
		t.Fatalf("LastUsedAt should be nil, got %v", byHash.LastUsedAt)
	}
	if byHash.RevokedAt != nil {
		t.Fatalf("RevokedAt should be nil, got %v", byHash.RevokedAt)
	}
}

func TestSQLStoreGetAPITokenNotFound(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// GetAPIToken with nonexistent ID
	if _, err := s.GetAPIToken(ctx, ID("AT-nonexistent")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, expected ErrNotFound", err)
	}

	// GetAPITokenByHash with nonexistent hash
	if _, err := s.GetAPITokenByHash(ctx, "nonexistent-hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, expected ErrNotFound", err)
	}
}

func TestSQLStoreListAPITokensOrderAndOwnerFilter(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// Create two owners
	owner1ID, _ := newID(KindOwner)
	owner2ID, _ := newID(KindOwner)

	if err := s.CreateOwner(ctx, testOwner(owner1ID)); err != nil {
		t.Fatalf("CreateOwner owner1: %v", err)
	}

	// Create owner2 with different login
	now := time.Now().UTC().Truncate(time.Second)
	owner2 := Owner{ID: owner2ID, Login: "other-user", PasswordHash: "hash", CreatedAt: now}
	if err := s.CreateOwner(ctx, owner2); err != nil {
		t.Fatalf("CreateOwner owner2: %v", err)
	}

	// Create 2 tokens for owner1
	token1ID, _ := newID(KindAPIToken)
	token1 := testAPIToken(token1ID, owner1ID)
	token1.Label = "token-1-owner1"
	if err := s.CreateAPIToken(ctx, token1); err != nil {
		t.Fatalf("CreateAPIToken 1: %v", err)
	}

	// created_at is RFC3339, second-granular, so rows created within the same
	// second can share it; ORDER BY created_at, id then falls back to id as
	// the tiebreaker. id is a time-ordered ULID, so the sleep exists to force
	// distinct millisecond-resolution ULID prefixes between rows, not
	// distinct created_at values (which it would not reliably do anyway).
	time.Sleep(10 * time.Millisecond)

	token2ID, _ := newID(KindAPIToken)
	token2 := testAPIToken(token2ID, owner1ID)
	token2.Label = "token-2-owner1"
	if err := s.CreateAPIToken(ctx, token2); err != nil {
		t.Fatalf("CreateAPIToken 2: %v", err)
	}

	// See comment above the first sleep: forces a distinct ULID prefix, not a
	// distinct created_at.
	time.Sleep(10 * time.Millisecond)

	// Create 2 tokens for owner2
	token3ID, _ := newID(KindAPIToken)
	token3 := testAPIToken(token3ID, owner2ID)
	token3.Label = "token-1-owner2"
	if err := s.CreateAPIToken(ctx, token3); err != nil {
		t.Fatalf("CreateAPIToken 3: %v", err)
	}

	// See comment above the first sleep: forces a distinct ULID prefix, not a
	// distinct created_at.
	time.Sleep(10 * time.Millisecond)

	token4ID, _ := newID(KindAPIToken)
	token4 := testAPIToken(token4ID, owner2ID)
	token4.Label = "token-2-owner2"
	if err := s.CreateAPIToken(ctx, token4); err != nil {
		t.Fatalf("CreateAPIToken 4: %v", err)
	}

	// List tokens for owner1
	list1, err := s.ListAPITokens(ctx, owner1ID)
	if err != nil {
		t.Fatalf("ListAPITokens owner1: %v", err)
	}
	if len(list1) != 2 {
		t.Fatalf("owner1 should have 2 tokens, got %d", len(list1))
	}
	// Check labels are owner1's tokens
	if list1[0].Label != "token-1-owner1" {
		t.Fatalf("first token label should be 'token-1-owner1', got %s", list1[0].Label)
	}
	if list1[1].Label != "token-2-owner1" {
		t.Fatalf("second token label should be 'token-2-owner1', got %s", list1[1].Label)
	}

	// List tokens for owner2
	list2, err := s.ListAPITokens(ctx, owner2ID)
	if err != nil {
		t.Fatalf("ListAPITokens owner2: %v", err)
	}
	if len(list2) != 2 {
		t.Fatalf("owner2 should have 2 tokens, got %d", len(list2))
	}
	// Check labels are owner2's tokens
	if list2[0].Label != "token-1-owner2" {
		t.Fatalf("first token label should be 'token-1-owner2', got %s", list2[0].Label)
	}
	if list2[1].Label != "token-2-owner2" {
		t.Fatalf("second token label should be 'token-2-owner2', got %s", list2[1].Label)
	}
}

func TestSQLStoreListAPITokensEmptyIsEmptySlice(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// Create owner
	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	// List tokens for owner with no tokens
	got, err := s.ListAPITokens(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListAPITokens: %v", err)
	}

	// Must be non-nil empty slice, not nil
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d tokens", len(got))
	}
}

func TestSQLStoreTouchAPITokenSetsLastUsedAt(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// Create owner
	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	// Create token
	tokenID, _ := newID(KindAPIToken)
	token := testAPIToken(tokenID, ownerID)
	before := time.Now().UTC().Truncate(time.Second)
	if err := s.CreateAPIToken(ctx, token); err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	// Touch token
	if err := s.TouchAPIToken(ctx, tokenID); err != nil {
		t.Fatalf("TouchAPIToken: %v", err)
	}

	// Read back and verify LastUsedAt
	got, err := s.GetAPIToken(ctx, tokenID)
	if err != nil {
		t.Fatalf("GetAPIToken: %v", err)
	}

	if got.LastUsedAt == nil {
		t.Fatal("LastUsedAt should not be nil after TouchAPIToken")
	}

	if got.LastUsedAt.Before(before) {
		t.Fatalf("LastUsedAt should not be before touch time: %v < %v", got.LastUsedAt, before)
	}

	if got.LastUsedAt.Before(got.CreatedAt) {
		t.Fatalf("LastUsedAt should not be before CreatedAt: %v < %v", got.LastUsedAt, got.CreatedAt)
	}
}

func TestSQLStoreTouchAPITokenNotFound(t *testing.T) {
	s := newSQLStore(t)

	if err := s.TouchAPIToken(context.Background(), ID("AT-nonexistent")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, expected ErrNotFound", err)
	}
}

func TestSQLStoreRevokeAPITokenSetsRevokedAt(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// Create owner
	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	// Create token
	tokenID, _ := newID(KindAPIToken)
	token := testAPIToken(tokenID, ownerID)
	if err := s.CreateAPIToken(ctx, token); err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	// Revoke token
	if err := s.RevokeAPIToken(ctx, tokenID); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}

	// Read back and verify RevokedAt
	got, err := s.GetAPIToken(ctx, tokenID)
	if err != nil {
		t.Fatalf("GetAPIToken: %v", err)
	}

	if got.RevokedAt == nil {
		t.Fatal("RevokedAt should not be nil after RevokeAPIToken")
	}
}

func TestSQLStoreRevokeAPITokenNotFound(t *testing.T) {
	s := newSQLStore(t)

	if err := s.RevokeAPIToken(context.Background(), ID("AT-nonexistent")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, expected ErrNotFound", err)
	}
}
