package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testInvite(id, createdBy ID) Invite {
	now := time.Now().UTC().Truncate(time.Second)
	return Invite{
		ID:        id,
		CreatedBy: createdBy,
		TokenHash: "hash-" + string(id),
		ExpiresAt: now.Add(24 * time.Hour),
		UsedAt:    nil,
		UsedBy:    nil,
		CreatedAt: now,
	}
}

func TestSQLStoreCreateAndGetInvite(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// Create owner (author/CreatedBy)
	authorID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(authorID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	// Create invite
	inviteID, _ := newID(KindInvite)
	want := testInvite(inviteID, authorID)

	if err := s.CreateInvite(ctx, want); err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	// Get by token hash
	got, err := s.GetInviteByHash(ctx, want.TokenHash)
	if err != nil {
		t.Fatalf("GetInviteByHash: %v", err)
	}

	if *got != want {
		t.Fatalf("GetInviteByHash = %+v, want %+v", *got, want)
	}

	// Verify UsedAt and UsedBy are nil
	if got.UsedAt != nil {
		t.Fatalf("UsedAt should be nil, got %v", got.UsedAt)
	}
	if got.UsedBy != nil {
		t.Fatalf("UsedBy should be nil, got %v", got.UsedBy)
	}
}

func TestSQLStoreGetInviteNotFound(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	if _, err := s.GetInviteByHash(ctx, "nonexistent-hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestSQLStoreMarkInviteUsedSetsFields(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// Create author owner
	authorID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(authorID)); err != nil {
		t.Fatalf("CreateOwner author: %v", err)
	}

	// Create redeemer owner
	redeemerID, _ := newID(KindOwner)
	now := time.Now().UTC().Truncate(time.Second)
	redeemer := Owner{ID: redeemerID, Login: "redeemer", PasswordHash: "hash", CreatedAt: now}
	if err := s.CreateOwner(ctx, redeemer); err != nil {
		t.Fatalf("CreateOwner redeemer: %v", err)
	}

	// Create invite from author
	inviteID, _ := newID(KindInvite)
	invite := testInvite(inviteID, authorID)

	if err := s.CreateInvite(ctx, invite); err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	// Mark as used by redeemer
	if err := s.MarkInviteUsed(ctx, inviteID, redeemerID); err != nil {
		t.Fatalf("MarkInviteUsed: %v", err)
	}

	// Read back and verify
	got, err := s.GetInviteByHash(ctx, invite.TokenHash)
	if err != nil {
		t.Fatalf("GetInviteByHash: %v", err)
	}

	if got.UsedAt == nil {
		t.Fatal("UsedAt should not be nil after MarkInviteUsed")
	}

	if got.UsedBy == nil {
		t.Fatal("UsedBy should not be nil after MarkInviteUsed")
	}

	if *got.UsedBy != redeemerID {
		t.Fatalf("UsedBy = %v, want %v", *got.UsedBy, redeemerID)
	}
}

func TestSQLStoreMarkInviteUsedNotFound(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// Create owner
	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	// Try to mark nonexistent invite as used
	if err := s.MarkInviteUsed(ctx, ID("IV-nonexistent"), ownerID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestSQLStoreInviteCreatedByRestrict(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	// Create author owner
	authorID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(authorID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	// Create invite from author
	inviteID, _ := newID(KindInvite)
	invite := testInvite(inviteID, authorID)
	if err := s.CreateInvite(ctx, invite); err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	// Attempt to delete author via raw SQL
	// Should fail due to ON DELETE RESTRICT foreign key
	_, err := s.db.ExecContext(ctx, "DELETE FROM owners WHERE id = ?", string(authorID))
	if err == nil {
		t.Fatal("expected error when deleting owner with existing invite (FK RESTRICT), but got nil")
	}
}
