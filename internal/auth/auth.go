package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashedPassword(password string) string {
	params := &argon2id.Params{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}
	hash, err := argon2id.CreateHash(password, params)
	if err != nil {
		panic(err)
	}
	return hash
}

func ComparePasswordAndHash(password, hash string) bool {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		panic(err)
	}
	return match
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject:   userID.String(),
	}
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)
	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenStr string, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenUnverifiable
		}
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return uuid.Nil, jwt.ErrTokenInvalidClaims
		}
		return userID, nil
	} else {
		return uuid.Nil, jwt.ErrTokenInvalidClaims
	}
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", jwt.ErrTokenInvalidClaims
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", jwt.ErrTokenInvalidClaims
	}
	return strings.TrimPrefix(authHeader, "Bearer "), nil
}

func MakeRefreshToken() string {
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(tokenBytes)
}

func GetAPIKey(headers http.Header) (string, error) {
	apiKey := headers.Get("Authorization")
	if apiKey == "" {
		return "", jwt.ErrTokenInvalidClaims
	}
	return strings.TrimPrefix(apiKey, "ApiKey "), nil
}
