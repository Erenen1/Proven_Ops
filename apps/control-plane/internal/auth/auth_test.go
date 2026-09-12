package auth

import (
	"testing"
)

func TestAuthTokens(t *testing.T) {
	svc := NewService("test_secret_key_12345678901234567890", 1)

	// Password hashing
	hash, err := svc.HashPassword("secretpass")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	if !svc.CheckPassword("secretpass", hash) {
		t.Errorf("expected valid password check")
	}
	if svc.CheckPassword("wrongpass", hash) {
		t.Errorf("expected invalid password check to fail")
	}

	// JWT Generation & Verification
	token, err := svc.GenerateToken("usr-1", "alice", []string{"admin", "operator"})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	if claims.Username != "alice" || claims.UserID != "usr-1" {
		t.Errorf("claims mismatch: %+v", claims)
	}

	// RBAC checks
	if !HasRole(claims.Roles, "operator") {
		t.Errorf("expected user to have operator role")
	}
	if !HasRole(claims.Roles, "admin") {
		t.Errorf("expected user to have admin role")
	}
	if HasRole([]string{"viewer"}, "operator") {
		t.Errorf("viewer should not have operator role")
	}
}
