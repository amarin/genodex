package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testSession(id, ownerID ID) Session {
	now := time.Now().UTC().Truncate(time.Second)
	return Session{
		ID:               id,
		OwnerID:          ownerID,
		AccessTokenHash:  "access-hash-" + string(id),
		AccessExpiresAt:  now.Add(1 * time.Hour),
		RefreshTokenHash: "refresh-hash-" + string(id),
		RefreshExpiresAt: now.Add(24 * time.Hour),
		CreatedAt:        now,
	}
}

func TestSQLStoreCreateAndGetSessionByAccessHash(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	sessID, _ := newID(KindSession)
	want := testSession(sessID, ownerID)

	if err := s.CreateSession(ctx, want); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := s.GetSessionByAccessHash(ctx, want.AccessTokenHash)
	if err != nil {
		t.Fatalf("GetSessionByAccessHash: %v", err)
	}
	if *got != want {
		t.Fatalf("GetSessionByAccessHash = %+v, want %+v", *got, want)
	}
}

func TestSQLStoreCreateAndGetSessionByRefreshHash(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	sessID, _ := newID(KindSession)
	want := testSession(sessID, ownerID)

	if err := s.CreateSession(ctx, want); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := s.GetSessionByRefreshHash(ctx, want.RefreshTokenHash)
	if err != nil {
		t.Fatalf("GetSessionByRefreshHash: %v", err)
	}
	if *got != want {
		t.Fatalf("GetSessionByRefreshHash = %+v, want %+v", *got, want)
	}
}

func TestSQLStoreGetSessionNotFound(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	if _, err := s.GetSessionByAccessHash(ctx, "nonexistent-hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if _, err := s.GetSessionByRefreshHash(ctx, "nonexistent-hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestSQLStoreReplaceSessionRotatesAndOldHashGone(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	sessID, _ := newID(KindSession)
	sess := testSession(sessID, ownerID)

	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	oldAccessHash := sess.AccessTokenHash
	oldRefreshHash := sess.RefreshTokenHash

	// Replace with new hashes but same ID/OwnerID
	now := time.Now().UTC().Truncate(time.Second)
	newSess := Session{
		ID:               sessID,
		OwnerID:          ownerID,
		AccessTokenHash:  "new-access-hash",
		AccessExpiresAt:  now.Add(2 * time.Hour),
		RefreshTokenHash: "new-refresh-hash",
		RefreshExpiresAt: now.Add(48 * time.Hour),
		CreatedAt:        now,
	}

	if err := s.ReplaceSession(ctx, sessID, newSess); err != nil {
		t.Fatalf("ReplaceSession: %v", err)
	}

	// Old hashes should not be resolvable
	if _, err := s.GetSessionByAccessHash(ctx, oldAccessHash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old access hash should not be found, err = %v", err)
	}

	if _, err := s.GetSessionByRefreshHash(ctx, oldRefreshHash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old refresh hash should not be found, err = %v", err)
	}

	// New hashes should be resolvable
	got, err := s.GetSessionByAccessHash(ctx, "new-access-hash")
	if err != nil {
		t.Fatalf("GetSessionByAccessHash with new hash: %v", err)
	}
	if got.ID != sessID {
		t.Fatalf("ID changed after ReplaceSession: got %s, want %s", got.ID, sessID)
	}
	if *got != newSess {
		t.Fatalf("GetSessionByAccessHash = %+v, want %+v", *got, newSess)
	}

	got, err = s.GetSessionByRefreshHash(ctx, "new-refresh-hash")
	if err != nil {
		t.Fatalf("GetSessionByRefreshHash with new hash: %v", err)
	}
	if *got != newSess {
		t.Fatalf("GetSessionByRefreshHash = %+v, want %+v", *got, newSess)
	}
}

func TestSQLStoreReplaceSessionNotFound(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	nonexistentID := ID("SS-nonexistent")
	sess := testSession(nonexistentID, ownerID)

	if err := s.ReplaceSession(ctx, nonexistentID, sess); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestSQLStoreDeleteSession(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	sessID, _ := newID(KindSession)
	sess := testSession(sessID, ownerID)

	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := s.DeleteSession(ctx, sessID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if _, err := s.GetSessionByAccessHash(ctx, sess.AccessTokenHash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound после DeleteSession", err)
	}
}

func TestSQLStoreDeleteSessionNotFound(t *testing.T) {
	s := newSQLStore(t)

	if err := s.DeleteSession(context.Background(), ID("SS-nonexistent")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestSQLStoreDeleteSessionsByOwner(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

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

	// Create 2 sessions for owner1
	sess1ID, _ := newID(KindSession)
	sess1 := testSession(sess1ID, owner1ID)
	if err := s.CreateSession(ctx, sess1); err != nil {
		t.Fatalf("CreateSession 1: %v", err)
	}

	sess2ID, _ := newID(KindSession)
	sess2 := testSession(sess2ID, owner1ID)
	if err := s.CreateSession(ctx, sess2); err != nil {
		t.Fatalf("CreateSession 2: %v", err)
	}

	// Create 1 session for owner2
	sess3ID, _ := newID(KindSession)
	sess3 := testSession(sess3ID, owner2ID)
	if err := s.CreateSession(ctx, sess3); err != nil {
		t.Fatalf("CreateSession 3: %v", err)
	}

	// Delete all sessions for owner1
	if err := s.DeleteSessionsByOwner(ctx, owner1ID); err != nil {
		t.Fatalf("DeleteSessionsByOwner: %v", err)
	}

	// Both owner1's sessions should be unresolvable
	if _, err := s.GetSessionByAccessHash(ctx, sess1.AccessTokenHash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session 1 should not be found after DeleteSessionsByOwner, err = %v", err)
	}

	if _, err := s.GetSessionByAccessHash(ctx, sess2.AccessTokenHash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session 2 should not be found after DeleteSessionsByOwner, err = %v", err)
	}

	// owner2's session should still be resolvable
	got, err := s.GetSessionByAccessHash(ctx, sess3.AccessTokenHash)
	if err != nil {
		t.Fatalf("owner2's session should still be found: %v", err)
	}
	if *got != sess3 {
		t.Fatalf("owner2's session = %+v, want %+v", *got, sess3)
	}
}

func TestSQLStoreDeleteSessionsByOwnerNoSessionsIsNotError(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	// Call DeleteSessionsByOwner on owner with zero sessions
	if err := s.DeleteSessionsByOwner(ctx, ownerID); err != nil {
		t.Fatalf("DeleteSessionsByOwner with no sessions should return nil, got: %v", err)
	}
}

func TestSQLStoreSessionOwnerRestrict(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	ownerID, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(ownerID)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	sessID, _ := newID(KindSession)
	sess := testSession(sessID, ownerID)
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Attempt to delete owner via raw SQL
	// Should fail due to ON DELETE RESTRICT foreign key
	_, err := s.db.ExecContext(ctx, "DELETE FROM owners WHERE id = ?", string(ownerID))
	if err == nil {
		t.Fatal("expected error when deleting owner with existing session (FK RESTRICT), but got nil")
	}
}
