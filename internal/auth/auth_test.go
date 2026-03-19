package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashedPassword(t *testing.T) {
	password := "mysecurepassword"
	hash := HashedPassword(password)

	if hash == "" {
		t.Fatal("HashedPassword returned empty string")
	}

	if hash == password {
		t.Fatal("HashedPassword returned plain password")
	}
}

func TestComparePasswordAndHash(t *testing.T) {
	password := "mysecurepassword"
	hash := HashedPassword(password)

	// Test correct password
	if !ComparePasswordAndHash(password, hash) {
		t.Fatal("ComparePasswordAndHash returned false for matching password")
	}

	// Test incorrect password
	if ComparePasswordAndHash("wrongpassword", hash) {
		t.Fatal("ComparePasswordAndHash returned true for non-matching password")
	}
}

func TestMakeJWT(t *testing.T) {
	userID := uuid.New()
	tokenSecret := "test-secret-key"
	expiresIn := time.Hour

	token, err := MakeJWT(userID, tokenSecret, expiresIn)

	if err != nil {
		t.Fatalf("MakeJWT returned error: %v", err)
	}

	if token == "" {
		t.Fatal("MakeJWT returned empty token")
	}
}

func TestValidateJWT(t *testing.T) {
	userID := uuid.New()
	tokenSecret := "test-secret-key"
	expiresIn := time.Hour

	// Create a valid token
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	// Validate the token
	parsedUserID, err := ValidateJWT(token, tokenSecret)
	if err != nil {
		t.Fatalf("ValidateJWT returned error: %v", err)
	}

	if parsedUserID != userID {
		t.Fatalf("ValidateJWT returned different userID. Got %v, expected %v", parsedUserID, userID)
	}
}

func TestValidateJWTExpired(t *testing.T) {
	userID := uuid.New()
	tokenSecret := "test-secret-key"
	expiresIn := -time.Hour // Negative duration = already expired

	// Create an expired token
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	// Try to validate the expired token
	_, err = ValidateJWT(token, tokenSecret)
	if err == nil {
		t.Fatal("ValidateJWT should have returned an error for expired token")
	}
}

func TestValidateJWTWrongSecret(t *testing.T) {
	userID := uuid.New()
	tokenSecret := "test-secret-key"
	wrongSecret := "wrong-secret-key"
	expiresIn := time.Hour

	// Create a token with original secret
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	// Try to validate with wrong secret
	_, err = ValidateJWT(token, wrongSecret)
	if err == nil {
		t.Fatal("ValidateJWT should have returned an error for wrong secret")
	}
}

func TestValidateJWTMalformed(t *testing.T) {
	tokenSecret := "test-secret-key"

	// Try to validate a malformed token
	_, err := ValidateJWT("not.a.valid.token", tokenSecret)
	if err == nil {
		t.Fatal("ValidateJWT should have returned an error for malformed token")
	}
}
