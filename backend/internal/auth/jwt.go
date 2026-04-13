package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 24 * time.Hour

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func GenerateToken(userID, email, secret string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

func ValidateToken(tokenStr, secret string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
