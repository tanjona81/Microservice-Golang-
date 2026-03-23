package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword turns a plain text password into a secure bcrypt hash
func HashPassword(password string, bcryptCost int) (string, error) {
	// DefaultCost is 10. Senior tip: Don't go too high (slows down server)
	// or too low (less secure). 10-12 is the sweet spot.
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(bytes), err
}

// CheckPasswordHash compares a plain text password with a hash
func CheckPasswordHash(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
