package utils

import (
	"errors"
	"example/hello/internal/domain"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret []byte

// CustomClaims represents the data we want to store inside the token
type CustomClaims struct {
	UserID int      `json:"user_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

// SetupErrors is called ONCE in main.go
func SetSecretKey(key string) {
	jwtSecret = []byte(key)
}

// GenerateJWT creates a new token for a user
func GenerateJWT(userID int, roles []string, expirationTime time.Time) (time.Time, string, error) {
	// Set the claims (the payload)
	claims := CustomClaims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			// Expiration: 15 to 30 min is for security
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			// Issued At: helps track when it was created
			IssuedAt: jwt.NewNumericDate(time.Now()),
			// Issuer: identifies who created the token
			Issuer: "examplehello",
		},
	}

	// Create the token using the HS256 algorithm
	// Create and sign the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtSecret)
	if err != nil {
		return time.Time{}, "", err
	}

	// Sign the token with our secret
	return expirationTime, signedToken, nil
}

// ValidateJWT checks if the token is valid and returns the claims
func ValidateJWT(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.NewInternalError(fmt.Errorf("unexpected signing method"))
		}
		return jwtSecret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.NewTokenExpired(err)
		}
		return nil, domain.NewInvalidCredential(err)
	}

	// Extract the claims
	claims, ok := token.Claims.(*CustomClaims)

	if !ok || !token.Valid {
		return nil, domain.NewInvalidCredential(fmt.Errorf("token extraction error"))
	}

	if claims.Issuer != "examplehello" {
		return nil, domain.NewInvalidCredential(fmt.Errorf("unauthorized issuer"))
	}

	return claims, nil
}
